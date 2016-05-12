package confbot

import (
	"fmt"

	"github.com/nlopes/slack"

	"golang.org/x/net/context"
)

// CreateSettingsAction return a fuction that lists some settings.
func CreateSettingsAction(repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, s *slack.Client, matches [][]string) error {
		userID := m.User
		id, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		_, _, channelID, err := s.OpenIMChannel(m.User)
		if err != nil {
			return err
		}

		params := slack.NewPostMessageParameters()

		attachment := slack.Attachment{
			Pretext: "List of URLs for your environment",
			Fields:  createSettings(id),
		}

		params.Attachments = []slack.Attachment{attachment}

		if _, _, err := s.PostMessage(channelID, "Settings", params); err != nil {
			return err
		}

		return nil
	}
}

func createSettings(id string) []slack.AttachmentField {
	return []slack.AttachmentField{
		{Title: "Project ID", Value: id},
		{Title: "Site URL", Value: fmt.Sprintf("http://app.%s.%s:8888", id, dropletDomain), Short: false},
		{Title: "Consul URL", Value: fmt.Sprintf("http://app.%s.%s", id, dropletDomain), Short: false},
		{Title: "Jenkins URL", Value: fmt.Sprintf("http://app.%s.%s:8080", id, dropletDomain), Short: false},
		{Title: "Kibana URL", Value: fmt.Sprintf("http://app.%s.%s:5601", id, dropletDomain), Short: false},
		{Title: "Graphana URL", Value: fmt.Sprintf("http://app.%s.%s:3000", id, dropletDomain), Short: false},
		{Title: "Prometheus URL", Value: fmt.Sprintf("http://app.%s.%s:9090", id, dropletDomain), Short: false},
	}
}
