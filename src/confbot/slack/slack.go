package slack

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
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
func (s *Slack) Receive() (*Message, string, error) {
	var in string
	err := websocket.Message.Receive(s.ws, &in)
	if err != nil {
		return nil, in, fmt.Errorf("websocket receive: %v", err)
	}

	s.log.WithField("content", in).Debug("incoming message")

	var m Message
	err = json.Unmarshal([]byte(in), &m)
	if err != nil {
		s.log.WithField("incoming-message", in).Warn("unknown message type")
		return nil, in, fmt.Errorf("json unmarshal: %v", err)
	}

	return &m, in, nil
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
func (s *Slack) SendToChannel(msg, channel string, attachments ...Attachment) (*Message, error) {
	ca := CallArgs{
		"channel":     channel,
		"text":        msg,
		"as_user":     true,
		"attachments": attachments,
	}

	s.log.WithField("channel", channel).Info("chat.postMessage")

	resp, err := s.Call("chat.postMessage", ca, UnmarshalMessage)
	if err != nil {
		return nil, err
	}

	m := resp.(*Message)
	return m, nil
}

// IM sends an instance essage to a user.
func (s *Slack) IM(userID, msg string, attachments ...Attachment) (*Message, error) {
	ca := CallArgs{"user": userID}

	s.log.WithField("user_id", userID).Info("sending IM")

	resp, err := s.Call("im.open", ca, UnmarshalIMOpen)
	if err != nil {
		return nil, err
	}

	im := resp.(*IM)
	if !im.OK {
		s.log.WithFields(logrus.Fields{
			"user_id": userID,
			"err":     im.Error}).
			Error("unable to open im channel")
		return nil, errors.New("unable to open im channel")
	}

	return s.SendToChannel(msg, im.Channel.ID, attachments...)
}

// Upload uploads a reader to a channel.
func (s *Slack) Upload(filename string, r io.Reader, channels []string) error {
	ff := formField{
		r:        r,
		fileName: filename,
	}

	ca := CallArgs{
		"file":     ff,
		"filename": filename,
		"channels": strings.Join(channels, ","),
	}

	resp, err := s.Call("files.upload", ca, UnmarshalMap)
	if err != nil {
		return err
	}

	m := resp.(map[string]interface{})
	if e := m["error"]; e != "" {
		return fmt.Errorf("slack api error: %s", e)
	}

	return nil
}

// Leave leaves a channel
func (s *Slack) Leave(channel string) error {
	s.log.WithField("channel", channel).Info("channels.leave")

	ca := CallArgs{"channel": channel}
	resp, err := s.Call("channels.leave", ca, UnmarshalMap)
	if err != nil {
		return err
	}

	m := resp.(map[string]interface{})
	if !m["ok"].(bool) {
		return fmt.Errorf("slack api error: %s", m["error"])
	}

	return nil
}

// UserInfo returns infor about a user.
func (s *Slack) UserInfo(userID string) (*User, error) {
	ca := CallArgs{"user": userID}
	resp, err := s.Call("users.info", ca, UnmarshalUser)
	if err != nil {
		return nil, err
	}

	user := resp.(*User)

	return user, nil
}

// AddReaction adds a reaction to a message
func (s *Slack) AddReaction(msgID, channel, reaction string) error {
	ca := CallArgs{
		"name":      reaction,
		"channel":   channel,
		"timestamp": msgID,
	}
	_, err := s.Call("reactions.add", ca, UnmarshalMap)
	return err
}

// RemoveReaction removes a reaction from a message
func (s *Slack) RemoveReaction(msgID, channel, reaction string) error {
	ca := CallArgs{
		"name":      reaction,
		"channel":   channel,
		"timestamp": msgID,
	}
	_, err := s.Call("reactions.remove", ca, UnmarshalMap)
	return err
}

// CallArgs are arguments to a Call request.
type CallArgs map[string]interface{}

// CallResults are the result of a Call request
type CallResults map[string]interface{}

// Call a Slack web api method.
func (s *Slack) Call(endpoint string, args CallArgs, fn UnmarshalFn) (interface{}, error) {
	args["token"] = s.token
	resp, err := call(endpoint, args, fn)
	if err != nil {
		s.log.WithError(err).
			WithFields(logrus.Fields{
			"endpoint": endpoint,
		}).Error("error calling Slack API")

		return nil, err
	}

	return resp, nil
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

type formField struct {
	r        io.Reader
	fileName string
}

func call2(endpoint string, args CallArgs) ([]byte, error) {
	u, err := url.Parse(apiURL)
	if err != nil {
		return nil, err
	}

	u.Path = "/api/" + endpoint

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	for k, v := range args {
		var vStr string
		switch t := v.(type) {
		case formField:
			ff := v.(formField)
			part, err := writer.CreateFormFile(k, ff.fileName)
			if err != nil {
				return nil, err
			}

			if _, err := io.Copy(part, ff.r); err != nil {
				return nil, err
			}
		case string:
			_ = writer.WriteField(k, v.(string))
		case bool:
			if v.(bool) {
				vStr = "true"
			} else {
				vStr = "false"
			}
			_ = writer.WriteField(k, vStr)
		case []Attachment:
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}
			_ = writer.WriteField(k, string(b))
		default:
			return nil, fmt.Errorf("unknown field type (%T) %v", t, t)
		}
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", u.String(), body)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", writer.FormDataContentType())

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("slack API request failed with code %d", resp.StatusCode)
	}

	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}

type slackResponse struct {
	OK   bool `json:"ok,omitempty"`
	User json.RawMessage
}

// UnmarshalUser unmarshals a slack response as a user.
func UnmarshalUser(b []byte) (interface{}, error) {
	var u User
	var sr slackResponse

	err := json.Unmarshal(b, &sr)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(sr.User, &u)
	if err != nil {
		return nil, err
	}

	return &u, nil
}

// UnmarshalIMOpen unmarshals a slack response as an im open.
func UnmarshalIMOpen(b []byte) (interface{}, error) {
	var i IM
	err := json.Unmarshal(b, &i)
	if err != nil {
		return nil, err
	}

	return &i, nil
}

// UnmarshalMessage unmarshals a slack response as a message.
func UnmarshalMessage(b []byte) (interface{}, error) {
	var m Message
	err := json.Unmarshal(b, &m)
	if err != nil {
		return nil, err
	}

	return &m, nil
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
