package api

import (
	"confbot"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"

	"golang.org/x/net/context"

	"gopkg.in/labstack/echo.v1"
	mw "gopkg.in/labstack/echo.v1/middleware"
)

type requestWebhook struct {
	Type      string            `json:"type"`
	ProjectID string            `json:"project_id"`
	Options   map[string]string `json:"options"`
}

// API is the confbot API.
type API struct {
	Mux         http.Handler
	repo        confbot.Repo
	log         *logrus.Entry
	slackClient *slack.Client
	ctx         context.Context
}

// New creates an instance of API.
func New(ctx context.Context, repo confbot.Repo, s *slack.Client) *API {
	log := ctx.Value("log").(*logrus.Entry)
	a := &API{
		repo:        repo,
		log:         log,
		slackClient: s,
		ctx:         ctx,
	}

	e := echo.New()

	e.Use(NewWithNameAndLogger("api", log))
	e.Use(mw.Recover())

	e.Post("/webhook", a.webhook)
	e.Get("/status", a.status)

	a.Mux = e

	return a
}

func (a *API) status(c *echo.Context) error {
	return c.String(http.StatusOK, "OK")
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

	log := a.log.WithFields(logrus.Fields{
		"webhook": r.Type,
		"user-id": userID})

	switch r.Type {
	case "install_complete":
		projectID, err := a.repo.ProjectID(userID)
		if err != nil {
			return err
		}

		log.Info("starting provisioner")

		params := slack.PostMessageParameters{}
		msg := fmt.Sprintf(
			"I've booted the shell Droplet for _%s_. Next, I will run the provisioner which will create the full "+
				"environment. This process will take a few more minutes.", projectID)
		a.slackClient.PostMessage(channelID, msg, params)

		provisioner := confbot.NewProvision(a.ctx, userID, projectID, channelID, a.repo, a.slackClient)
		provisioner.Run()
	case "jenkins":
		log.WithField("raw", fmt.Sprintf("%#v", r)).Info("jenkins received")

		jobName := r.Options["name"]
		buildNum := r.Options["number"]
		buildURL := fmt.Sprintf("http://app.%s.%s:8080/job/%s/%s", r.ProjectID, confbot.DropletDomain, jobName, buildNum)
		buildURLJSON := fmt.Sprintf("%s/api/json", buildURL)

		log.WithField("jenkins-info-url", buildURLJSON).Info("fetching job data")
		res, err := http.Get(buildURLJSON)
		if err != nil {
			log.WithError(err).WithField("build-url", buildURLJSON).Error("unable to retrieve build data")
			return err
		}
		defer res.Body.Close()

		b, err := ioutil.ReadAll(res.Body)
		if err != nil {
			return err
		}

		var in map[string]interface{}
		err = json.Unmarshal(b, &in)
		if err != nil {
			log.WithError(err).WithField("raw", string(b)).Error("unable to decode build json")
		}

		buildStatus := in["result"].(string)

		params := slack.NewPostMessageParameters()
		params.Username = "jenkins"
		params.IconURL = "https://s3.pifft.com/oscon2016/jenkins.png"
		msg := fmt.Sprintf("Build Number: %s - %s - %s", buildNum, buildStatus, buildURL)
		a.slackClient.PostMessage(channelID, msg, params)
	}

	return c.NoContent(http.StatusNoContent)
}
