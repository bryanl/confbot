package confbot

import (
	"confbot/slack"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
)

// CreateResetAction returns a function that can reset a current user's settings.
func CreateResetAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		userID := m.User
		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": userID})

		log.Info("reseting project")

		if _, err := s.IM(userID, "resetting your projects"); err != nil {
			return err
		}

		if err := repo.ResetProject(userID); err != nil {
			log.WithError(err).Error("unable to reset project")
		}

		return nil
	}
}
