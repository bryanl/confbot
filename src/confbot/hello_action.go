package confbot

import (
	"confbot/slack"
	"fmt"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

func helloWorldAction(ctx context.Context, m *slack.Message, s *slack.Slack) error {
	log := logFromContext(ctx)

	ca := slack.CallArgs{
		"user": m.User,
	}
	cr, err := s.Call("users.info", ca)
	if err != nil {
		log.WithError(err).
			Error("unable to lookup user")
	}

	user := cr["user"].(map[string]interface{})
	profile := user["profile"].(map[string]interface{})
	firstName := profile["first_name"].(string)

	reply := &slack.OutgoingMessage{
		Channel: m.Channel,
		Type:    "message",
		Text:    fmt.Sprintf("Hello %s", firstName),
	}

	if err := s.Send(reply); err != nil {
		log.WithError(err).WithFields(logrus.Fields{}).Error("unable to send message")
		return err
	}

	return nil
}
