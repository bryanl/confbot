package confbot

import (
	"fmt"

	"github.com/Sirupsen/logrus"
	"github.com/go-errors/errors"
	"github.com/nlopes/slack"

	"golang.org/x/net/context"
)

// CreateHelloAction creates a hello action.
func CreateHelloAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client) error {
		log := logFromContext(ctx).WithFields(logrus.Fields{"user-id": m.User})

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			log.WithError(err).Error("unable to open im channel")
			return err
		}

		user, err := slackClient.GetUserInfo(m.User)
		if err != nil {
			log.WithError(err).Error("unable to fetch user info")
		}

		msg := fmt.Sprintf(helloResponse, user.Name, "confbot")
		params := slack.NewPostMessageParameters()
		if _, _, err := slackClient.PostMessage(channelID, msg, params); err != nil {
			return err
		}

		id, err := repo.ProjectID(m.User)
		if err != nil {
			return errors.Wrap(err, 1)
		}

		if id == "" {
			params := slack.NewPostMessageParameters()
			if _, _, err := slackClient.PostMessage(channelID, projecIsNotDefined, params); err != nil {
				return errors.Wrap(err, 1)
			}
		} else {
			msg = fmt.Sprintf(projectIsDefined, id)
			params := slack.NewPostMessageParameters()
			if _, _, err := slackClient.PostMessage(channelID, msg, params); err != nil {
				return err
			}
		}

		return nil
	}

}

var (
	helloResponse = "Hello, *%s*, I'm %s, and I will be working with you during the OSCON AppOps tutorial. " +
		"I can help you create your enviroment, and offer help when I can. To get started, you will have to issue me a command."

	projecIsNotDefined = "It doesn't look like you have a project defined. To start a new project, tell me to `./boot shell`"
	projectIsDefined   = "It looks like you already have a project (_%s_) defined. If you require help or want to know what to do next, ask me for help with `./help`"
)
