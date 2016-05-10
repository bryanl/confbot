package confbot

import (
	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

// CreateResetAction returns a function that can reset a current user's settings.
func CreateResetAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client) error {
		userID := m.User
		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": userID})

		log.Info("reseting project")

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			return err
		}

		params := slack.PostMessageParameters{}
		if _, _, err := slackClient.PostMessage(channelID, "resetting your projects", params); err != nil {
			return err
		}

		if err := repo.ResetProject(userID); err != nil {
			log.WithError(err).Error("unable to reset project")
		}

		return nil
	}
}
