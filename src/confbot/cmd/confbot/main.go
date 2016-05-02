package main

import (
	"confbot"
	"net/http"
	"os"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/kelseyhightower/envconfig"

	"confbot/api"
	"confbot/logging"
	"confbot/slack"
)

const (
	appName = "confbot"
)

var (
	rootLog = logrus.New()
)

// Specification describes the enviroment required to run confbot.
type Specification struct {
	Env               string `default:"development"`
	SlackToken        string `envconfig:"slack_token" required:"true"`
	PaperTrailHost    string `envconfig:"papertrail_host"`
	PaperTrailPort    int    `envconfig:"papertrail_port"`
	DigitalOceanToken string `envconfig:"digitalocean_token" required:"true"`
	RedisURL          string `envconfig:"redis_url" required:"true"`
	HTTPAddr          string `envconfig:"http_addr" default:"localhost:8080"`
	RemoteLogging     bool   `envconfig:"remote_logging" default:"false"`
}

func main() {
	var spec Specification
	err := envconfig.Process("confbot", &spec)
	if err != nil {
		rootLog.WithError(err).Fatal("unable to read environment")
	}

	log := rootLog.WithFields(logrus.Fields{
		"env": spec.Env,
	})

	setupLogger(spec, log)

	ctx := context.WithValue(context.Background(), "log", log)

	s, err := slack.New(ctx, spec.SlackToken)
	if err != nil {
		rootLog.WithError(err).Fatal("unable to connect to slack")
	}

	log.Info("application started")

	repo, err := confbot.NewRepo(ctx, spec.RedisURL, spec.Env)
	if err != nil {
		rootLog.WithError(err).Fatalf("unable to create repo")
	}

	cb := confbot.New(ctx, s, repo)

	cb.AddTextAction("./boot shell", confbot.CreateBootShellAction(ctx, spec.DigitalOceanToken, repo))
	cb.AddTextAction("./delete", confbot.CreateDeleteAction(ctx, spec.DigitalOceanToken, repo))
	cb.AddTextAction("./reset", confbot.CreateResetAction(ctx, repo))
	cb.AddTextAction("./provision", confbot.CreateProvisionAction(ctx, repo))
	cb.AddTextAction("./settings", confbot.CreateSettingsAction(repo))
	go cb.Listen()

	a := api.New(ctx, repo, s)
	http.Handle("/", a.Mux)

	log.WithField("addr", spec.HTTPAddr).Info("created http server")
	log.Fatal(http.ListenAndServe(spec.HTTPAddr, nil))
}

func setupLogger(spec Specification, log *logrus.Entry) {
	if spec.RemoteLogging {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("unable to retrieve app hostname: %v", err)
		}

		hook, err := logging.NewPapertrailHook(&logging.Hook{
			Host:     spec.PaperTrailHost,
			Port:     spec.PaperTrailPort,
			Hostname: hostname,
			Appname:  "confbot",
		})

		if err != nil {
			log.WithError(err).Fatalf("unable to set up papertrail logging")
		}

		rootLog.Hooks.Add(hook)
	}
}
