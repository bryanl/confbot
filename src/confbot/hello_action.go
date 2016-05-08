package confbot

import (
	"confbot/slack"
	"fmt"

	"github.com/Sirupsen/logrus"
	"golang.org/x/net/context"
)

func helloWorldAction(ctx context.Context, m *slack.Message, s *slack.Slack) error {
	log := logFromContext(ctx)

	user, err := s.UserInfo(m.User)
	if err != nil {
		log.WithError(err).
			Error("unable to lookup user")
	}

	reply := &slack.OutgoingMessage{
		Channel: m.Channel(),
		Type:    "message",
		Text:    fmt.Sprintf("Hello %s", user.Profile.FirstName),
	}

	if err := s.Send(reply); err != nil {
		log.WithError(err).WithFields(logrus.Fields{}).Error("unable to send message")
		return err
	}

	return nil
}
