package confbot

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

const (
	reactionUp    = "white_check_mark"
	reactionNew   = "warning"
	reactionReady = "100"
)

// CreateBootShellAction returns a function that boot a new shell.
func CreateBootShellAction(ctx context.Context, doToken string, repo Repo) ActionFn {
	log := logFromContext(ctx)

	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client) error {

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			return err
		}

		id := projectID()
		sb := NewShellBooter(id, doToken, log)

		userID := m.User
		if err := repo.RegisterProject(id, userID); err != nil {
			params := slack.PostMessageParameters{}
			msg := fmt.Sprintf("unknown error: %v", err)
			slackClient.PostMessage(channelID, msg, params)

		}

		log.WithFields(logrus.Fields{
			"user-id":    userID,
			"project-id": id,
		}).Info("new shell request")

		if err != nil {
			switch err.(type) {
			case *ProjectExistsErr:
				params := slack.PostMessageParameters{}

				id, err = repo.ProjectID(userID)
				if err != nil {
					return err
				}

				params = slack.PostMessageParameters{}
				msg := fmt.Sprintf("You already have an existing shell at *%s*", id)
				slackClient.PostMessage(channelID, msg, params)

			default:
				params := slack.PostMessageParameters{}
				msg := fmt.Sprintf("unknown error: %v", err)
				slackClient.PostMessage(channelID, msg, params)
			}
			return err
		}

		txt := fmt.Sprintf(shellResp1, id, dropletDomain)
		params := slack.PostMessageParameters{}
		slackClient.PostMessage(channelID, txt, params)

		sc, err := sb.Boot()
		if err != nil {
			log.WithError(err).Error("couldn't boot shell")
			msg := fmt.Sprintf("couldn't boot shell: %s", err)

			params := slack.PostMessageParameters{}
			slackClient.PostMessage(channelID, msg, params)
		}

		if err := repo.SaveKey(id, sc.KeyPair.private); err != nil {
			return err
		}

		return nil
	}
}

var shellResp1 = `I'm currently booting a server named _shell.%s.%s_. ` +
	`After it has booted, I will use it to provision the rest of your environment. ` +
	`This process will take a few minutes, and I'll let you know when it is completed.`
