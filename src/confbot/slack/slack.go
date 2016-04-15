package slack

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"sync/atomic"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
	"golang.org/x/net/websocket"
)

const (
	apiURL = "https://slack.com/api"
	rtURL  = "https://api.slack.com"
)

// Slack is a connection to Slack.
type Slack struct {
	log            *logrus.Entry
	token          string
	ws             *websocket.Conn
	id             string
	messageCounter uint64
}

// New creates an instance of Slack given a Slack token.
func New(ctx context.Context, token string) (*Slack, error) {
	wsurl, id, err := start(token)
	if err != nil {
		return nil, err
	}

	ws, err := websocket.Dial(wsurl, "", rtURL)
	if err != nil {
		return nil, err
	}

	logger := ctx.Value("log").(*logrus.Entry)

	return &Slack{
		log:   logger,
		token: token,
		ws:    ws,
		id:    id,
	}, nil
}

// ID returns the user id of the Slack connection.
func (s *Slack) ID() string {
	return s.id
}

// Receive blocks until it receives a message from the Slack API.
func (s *Slack) Receive() (*Message, error) {
	var in string
	err := websocket.Message.Receive(s.ws, &in)
	if err != nil {
		return nil, err
	}

	s.log.WithField("content", in).Debug("incoming message")

	var m Message
	err = json.Unmarshal([]byte(in), &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
}

// Send sends a message to the Slack API.
func (s *Slack) Send(om *OutgoingMessage) error {
	om.ID = atomic.AddUint64(&s.messageCounter, 1)
	if err := websocket.JSON.Send(s.ws, om); err != nil {
		s.log.WithError(err).WithFields(logrus.Fields{}).Error("unable to send message")

		return err
	}

	return nil
}

// SendToChannel sends a text message to a channel.
func (s *Slack) SendToChannel(msg, channel string) error {
	om := &OutgoingMessage{
		Channel: channel,
		Type:    "message",
		Text:    msg,
	}

	return s.Send(om)
}

// UserInfo returns infor about a user.
func (s *Slack) UserInfo(userID string) (*User, error) {
	ca := CallArgs{"user": userID}
	resp, err := s.Call("users.info", ca, UnmarshalUser)
	if err != nil {
		s.log.WithError(err).
			WithFields(logrus.Fields{
			"api_call": "users.info",
		}).Error("unable to lookup user")
		return nil, err
	}

	user := resp.(User)

	return &user, nil
}

// CallArgs are arguments to a Call request.
type CallArgs map[string]string

// CallResults are the result of a Call request
type CallResults map[string]interface{}

// Call a Slack web api method.
func (s *Slack) Call(endpoint string, args CallArgs, fn UnmarshalFn) (interface{}, error) {
	args["token"] = s.token
	return call(endpoint, args, fn)
}

// start starts a slack connection with rtm.start, and returns a websock URL and user ID, or an error.
func start(token string) (string, string, error) {
	args := CallArgs{}

	args["token"] = token
	resp, err := call("rtm.start", args, UnmarshalMap)
	if err != nil {
		return "", "", err
	}

	cr := resp.(map[string]interface{})

	if !cr["ok"].(bool) {
		errStr := cr["error"].(string)
		return "", "", fmt.Errorf("slack API error: %s", errStr)
	}

	u := cr["url"].(string)
	self := cr["self"].(map[string]interface{})
	id := self["id"].(string)

	return u, id, nil
}

// UnmarshalFn is a function that unmarshals a response to a type.
type UnmarshalFn func(b []byte) (interface{}, error)

func call(endpoint string, args CallArgs, fn UnmarshalFn) (interface{}, error) {
	body, err := call2(endpoint, args)
	if err != nil {
		return nil, err
	}
	return fn(body)
}

func call2(endpoint string, args CallArgs) ([]byte, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	u.Path = "/api/" + endpoint
	values := u.Query()

	for k, v := range args {
		values.Set(k, v)
	}

	u.RawQuery = values.Encode()

	resp, err := http.Get(u.String())
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack API request failed with code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

// UnmarshalUser unmarshals a slack response as a user.
func UnmarshalUser(b []byte) (interface{}, error) {
	var u User
	err := json.Unmarshal(b, &u)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// UnmarshalMap unmarshals a slack api response as a map.
func UnmarshalMap(b []byte) (interface{}, error) {
	var m map[string]interface{}
	err := json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	return m, nil
}
