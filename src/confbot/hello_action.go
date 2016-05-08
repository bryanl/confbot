package confbot

import (
	"confbot/slack"
	"fmt"

	"github.com/Sirupsen/logrus"

	"golang.org/x/net/context"
)

// CreateHelloAction creates a hello action.
func CreateHelloAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		userID := m.User
		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": userID})

		user, err := s.UserInfo(m.User)
		if err != nil {
			log.WithError(err).
				Error("unable to lookup user")
		}

		msg := fmt.Sprintf(helloResp1, user.Name, "confbot")
		if _, err := s.IM(userID, msg); err != nil {
			return err
		}

		id, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		if id == "" {
			if _, err := s.IM(userID, helloResp2); err != nil {
				return err
			}
		} else {
			msg = fmt.Sprintf(helloResp3, id)
			if _, err := s.IM(userID, msg); err != nil {
				return err
			}
		}

		return nil
	}

}

var (
	helloResp1 = "Hello, *%s*, I'm %s, and I will be working with you during the OSCON AppOps tutorial. " +
		"I can help you create your enviroment, and offer help when I can. To get started, you will have to issue me a command."

	helloResp2 = "It doesn't look like you have a project defined. To start a new project, tell me to `./boot shell`"
	helloResp3 = "It looks like you already have a project (_%s_) defined. If you require help or want to know what to do next, ask me for help with `./help`"
)
