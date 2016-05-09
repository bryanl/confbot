package confbot

import (
	"confbot/slack"
	"fmt"

	"github.com/Sirupsen/logrus"
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

	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		id := projectID()
		sb := NewShellBooter(id, doToken, log)

		userID := m.User
		err := repo.RegisterProject(id, userID)

		log.WithFields(logrus.Fields{
			"user-id":    userID,
			"project-id": id,
		}).Info("new shell request")

		if err != nil {
			switch err.(type) {
			case *ProjectExistsErr:
				if _, sErr := s.IM(userID, fmt.Sprintf("unable to boot shell: %v", err)); sErr != nil {
					return sErr
				}

				id, err = repo.ProjectID(userID)
				if err != nil {
					return err
				}

				_, _ = s.IM(userID, fmt.Sprintf("You already have an existing shell at *%s*", id))

			default:
				if _, sErr := s.IM(userID, fmt.Sprintf("unknown error: %v", err)); sErr != nil {
					return sErr
				}
			}
			return err
		}

		txt := fmt.Sprintf(shellResp1, id, dropletDomain)
		if _, err := s.IM(userID, txt); err != nil {
			return err
		}

		var reply slack.OutgoingMessage
		sc, err := sb.Boot()
		if err != nil {
			log.WithError(err).Error("couldn't boot shell")
			msg := fmt.Sprintf("couldn't boot shell: %s", err)
			if _, err := s.IM(userID, msg); err != nil {
				return err
			}
		}

		if err := s.Send(&reply); err != nil {
			return err
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
