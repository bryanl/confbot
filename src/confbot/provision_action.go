package confbot

import (
	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"

	"golang.org/x/net/context"
)

type provision struct {
	ctx       context.Context
	log       *logrus.Entry
	repo      Repo
	userID    string
	projectID string
	slack     *slack.Client
	channel   string
}

func NewProvision(ctx context.Context, userID, projectID, channel string, repo Repo, s *slack.Client) *provision {
	return &provision{
		log:       logFromContext(ctx),
		repo:      repo,
		userID:    userID,
		projectID: projectID,
		ctx:       ctx,
		slack:     s,
		channel:   channel,
	}
}

func (p *provision) Run() {
	for state := provisionInitState; state != nil; {
		state = state(p)
	}
}

// CreateProvisionAction creates a provision action.
func CreateProvisionAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client, matches [][]string) error {
		userID := m.User
		projectID, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			return err
		}

		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": userID})
		log.Info("creating provisioner")

		p := NewProvision(ctx, userID, projectID, channelID, repo, slackClient)
		p.Run()

		return nil
	}
}
