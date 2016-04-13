package confbot

import (
	"confbot/slack"
	"fmt"
	"regexp"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
)

// Confbot is a conference workshop bot.
type Confbot struct {
	s   *slack.Slack
	ctx context.Context

	textActions []textAction
}

// New creates an instance of Confbot.
func New(ctx context.Context, s *slack.Slack) *Confbot {
	cb := &Confbot{
		s:   s,
		ctx: ctx,
	}

	return cb
}

// Listen listens for new slack messages.
func (c *Confbot) Listen() {
	log := logFromContext(c.ctx)
	s := c.s

	for {
		m, err := s.Receive()
		if err != nil {
			log.WithError(err).Error("error receiving message from slack")
			continue
		}

		go func(m *slack.Message) {
			for _, ta := range c.textActions {
				if ta.re.Match([]byte(m.Text)) {
					ta.fn(c.ctx, m, s)
				}
			}
		}(m)
	}
}

// ActionFn is an action func.
type ActionFn func(context.Context, *slack.Message, *slack.Slack) error

type textAction struct {
	re *regexp.Regexp
	fn ActionFn
}

// AddTextAction adds a TextAction to the bot.
func (c *Confbot) AddTextAction(trigger string, fn ActionFn) error {
	re, err := regexp.Compile(trigger)
	if err != nil {
		return err
	}

	log := logFromContext(c.ctx)
	log.WithFields(logrus.Fields{
		"trigger": trigger,
	}).Info("adding text action to bot")

	c.textActions = append(c.textActions, textAction{re: re, fn: fn})
	return nil
}

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

func logFromContext(ctx context.Context) *logrus.Logger {
	return ctx.Value("log").(*logrus.Logger)
}
