package confbot

import (
	"confbot/slack"

	"golang.org/x/net/context"
)

// CreateSettingsAction return a fuction that lists some settings.
func CreateSettingsAction(repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		userID := m.User
		id, _ := repo.ProjectID(userID)

		if _, err := s.IM(userID, "project id: "+id); err != nil {
			return err
		}

		return nil
	}
}
