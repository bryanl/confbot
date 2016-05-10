package confbot

import (
	"fmt"
	"regexp"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
)

// Confbot is a conference workshop bot.
type Confbot struct {
	repo   Repo
	client *slack.Client
	ctx    context.Context

	textActions   []textAction
	validChannels []string
}

// New creates an instance of Confbot.
func New(ctx context.Context, s *slack.Client, repo Repo) *Confbot {
	cb := &Confbot{
		repo:   repo,
		ctx:    ctx,
		client: s,
	}

	return cb
}

// Listen listens for slack events.
func (c *Confbot) Listen() {
	log := logFromContext(c.ctx)

	c.client.SetDebug(false)
	rtm := c.client.NewRTM()
	go rtm.ManageConnection()

	for {
		select {
		case msg := <-rtm.IncomingEvents:
			switch ev := msg.Data.(type) {
			case *slack.MessageEvent:
				log.WithField("raw-event", fmt.Sprintf("%#v", ev)).Info("incoming message")

				for _, ta := range c.textActions {
					matches := ta.re.FindAllStringSubmatch(ev.Text, -1)
					if len(matches) > 0 {
						if err := ta.fn(c.ctx, ev, c.client, matches); err != nil {
							log.WithError(err).
								WithField("action", ev.Text).
								Error("could not run action")
						}
					}
				}

			case *slack.RTMError:
				fmt.Printf("Error: %s\n", ev.Error())
			}
		}
	}

}

// ActionFn is an action func.
type ActionFn func(context.Context, *slack.MessageEvent, *slack.Client, [][]string) error

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
