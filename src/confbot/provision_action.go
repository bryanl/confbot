package confbot

import (
	"confbot/slack"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
)

type provision struct {
	ctx       context.Context
	log       *logrus.Entry
	repo      Repo
	userID    string
	projectID string
	slack     *slack.Slack
	channel   string
}

func newProvision(ctx context.Context, userID, projectID, channel string, repo Repo, s *slack.Slack) *provision {
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

func (p *provision) run() {
	for state := initState; state != nil; {
		state = state(p)
	}
}

// CreateProvisionAction creates a provision action.
func CreateProvisionAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		userID := m.User
		projectID, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": userID})
		log.Info("creating provisioner")

		p := newProvision(ctx, userID, projectID, m.Channel(), repo, s)
		p.run()

		return nil
	}
}
