package api

import (
	"confbot"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"

	"golang.org/x/net/context"

	"gopkg.in/labstack/echo.v1"
	mw "gopkg.in/labstack/echo.v1/middleware"
)

type requestWebhook struct {
	Type      string `json:"type"`
	ProjectID string `json:"project_id"`
}

// API is the confbot API.
type API struct {
	Mux         http.Handler
	repo        confbot.Repo
	log         *logrus.Entry
	slackClient *slack.Client
}

// New creates an instance of API.
func New(ctx context.Context, repo confbot.Repo, s *slack.Client) *API {
	log := ctx.Value("log").(*logrus.Entry)
	a := &API{
		repo:        repo,
		log:         log,
		slackClient: s,
	}

	e := echo.New()

	e.Use(NewWithNameAndLogger("api", log))
	e.Use(mw.Recover())

	e.Post("/webhook", a.webhook)

	a.Mux = e

	return a
}

func (a *API) webhook(c *echo.Context) error {
	r := &requestWebhook{}
	if err := c.Bind(r); err != nil {
		return err
	}

	a.log.WithFields(logrus.Fields{
		"project_id": r.ProjectID,
		"type":       r.Type,
	}).Info("webhook received")

	userID, err := a.repo.User(r.ProjectID)
	if err != nil {
		a.log.WithError(err).
			WithField("project_id", r.ProjectID).
			Error("unknown project")
		return c.NoContent(http.StatusNotFound)
	}

	_, _, channelID, err := a.slackClient.OpenIMChannel(userID)
	if err != nil {
		return err
	}

	params := slack.PostMessageParameters{}
	msg := fmt.Sprintf("received webhook of type _%s_", r.Type)
	a.slackClient.PostMessage(channelID, msg, params)

	return c.NoContent(http.StatusNoContent)
}
