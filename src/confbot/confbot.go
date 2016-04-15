package confbot

import (
	"confbot/slack"
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

func logFromContext(ctx context.Context) *logrus.Logger {
	return ctx.Value("log").(*logrus.Logger)
}
