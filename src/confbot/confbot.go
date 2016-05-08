package confbot

import (
	"confbot/slack"
	"regexp"
	"time"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
)

// Confbot is a conference workshop bot.
type Confbot struct {
	repo Repo
	s    *slack.Slack
	ctx  context.Context

	textActions   []textAction
	validChannels []string
}

// New creates an instance of Confbot.
func New(ctx context.Context, s *slack.Slack, repo Repo) *Confbot {
	cb := &Confbot{
		repo: repo,
		s:    s,
		ctx:  ctx,
	}

	return cb
}

// Listen listens for new slack messages.
func (c *Confbot) Listen() {
	log := logFromContext(c.ctx)
	s := c.s

	for {
		m, raw, err := s.Receive()
		if err != nil {
			log.WithError(err).Error("error receiving message from slack")
			continue
		}

		go func(m *slack.Message) {
			l := log.WithFields(logrus.Fields{
				"type": m.Type,
				"raw":  raw,
			})

			for _, ta := range c.textActions {
				if ta.re.Match([]byte(m.Text)) {
					err := ta.fn(c.ctx, m, s)
					if err != nil {
						log.WithError(err).
							WithField("action", m.Text).
							Error("could not run action")
					}

					return
				}
			}

			switch m.Type {
			case "group_joined":
				ch := m.Channel()
				if !any(c.validChannels, func(s string) bool {
					return s == ch
				}) {
					s.SendToChannel("*i'm not authorized to be in this channel*", ch)
					time.Sleep(5 * time.Second)
					s.Leave(ch)
				}
			case "hello":
				log.Info("successful connected to slack message server")
			case "reconnect_url":
				// no op. looks to be some sort of slack experiment: https://api.slack.com/events/reconnect_url
			case "presence_change", "user_typing":
				// no op. these aren't useful.
			default:
				if u := m.User; u != "" {
					l = l.WithField("user", u)
				}

				if ch := m.Channel(); ch != "" {
					l = l.WithField("channel", ch)
				}

				l.Info("unhandled message")
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

func logFromContext(ctx context.Context) *logrus.Entry {
	return ctx.Value("log").(*logrus.Entry)
}

func any(vs []string, f func(string) bool) bool {
	for _, v := range vs {
		if f(v) {
			return true
		}
	}
	return false
}
