package confbot

import (
	"confbot/slack"
	"fmt"

	"golang.org/x/net/context"
)

// CreateSettingsAction return a fuction that lists some settings.
func CreateSettingsAction(repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		userID := m.User
		id, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		attachmennt := slack.Attachment{
			Fields: createSettings(id),
		}

		if _, err := s.IM(userID, "Settings", attachmennt); err != nil {
			return err
		}

		return nil
	}
}

func createSettings(id string) []slack.AttachmentField {
	return []slack.AttachmentField{
		{Title: "Project ID", Value: id},
		{Title: "Site URL", Value: "http://example.com", Short: true},
		{Title: "Consul URL", Value: fmt.Sprintf("http://app.%s.%s", id, dropletDomain), Short: true},
		{Title: "Jenkins URL", Value: fmt.Sprintf("http://example.com"), Short: true},
		{Title: "Kibana URL", Value: fmt.Sprintf("http://app.%s.%s:5601", id, dropletDomain), Short: true},
		{Title: "Graphana URL", Value: fmt.Sprintf("http://example.com"), Short: true},
		{Title: "Prometheus URL", Value: fmt.Sprintf("http://example.com"), Short: true},
	}
}
