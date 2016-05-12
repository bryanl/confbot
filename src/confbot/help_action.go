package confbot

import (
	"github.com/Sirupsen/logrus"
	"github.com/go-errors/errors"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

// CreateHelpAction creates a help action.
func CreateHelpAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client, matches [][]string) error {
		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": m.User, "action": "hello"})

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			log.WithError(err).Error("unable to open im channel")
			return err
		}

		id, err := repo.ProjectID(m.User)
		if err != nil {
			return errors.Wrap(err, 1)
		}

		if id == "" {
			msg := "Hello, it looks like you don't currently have a workshop environment created. " +
				"To get started run `./boot shell`."
			params := slack.NewPostMessageParameters()
			if _, _, err := slackClient.PostMessage(channelID, msg, params); err != nil {
				return errors.Wrap(err, 1)
			}
		} else {
			msg := "Hello, it looks like your workshop environment is in the process of, or has been booted. " +
				"The instructur will inform you of the next course of actions."
			params := slack.NewPostMessageParameters()
			if _, _, err := slackClient.PostMessage(channelID, msg, params); err != nil {
				return errors.Wrap(err, 1)
			}
		}

		return nil
	}
}
