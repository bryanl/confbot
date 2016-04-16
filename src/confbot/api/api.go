package api

import (
	"confbot"
	"confbot/slack"
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"

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
	Mux  http.Handler
	repo confbot.Repo
	s    *slack.Slack
	log  *logrus.Entry
}

// New creates an instance of API.
func New(ctx context.Context, repo confbot.Repo, s *slack.Slack) *API {
	log := ctx.Value("log").(*logrus.Entry)
	a := &API{
		repo: repo,
		s:    s,
		log:  log,
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

	if _, err := a.s.IM(userID, fmt.Sprintf("received webhook of type _%s_", r.Type)); err != nil {
		a.log.WithError(err).
			WithField("project_id", r.ProjectID).
			Error("could not send message")
	}

	return c.NoContent(http.StatusNoContent)
}
