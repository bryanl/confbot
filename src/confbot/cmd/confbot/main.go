package main

import (
	"confbot"
	"flag"
	"net/http"
	"os"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/kouhin/envflag"

	"confbot/api"
	"confbot/logging"
	"confbot/slack"
)

const (
	appName = "confbot"
)

var (
	env            = flag.String("confbot-env", "development", "environment")
	slackToken     = flag.String("confbot-slack-token", "", "slack token")
	paperTrailHost = flag.String("confbot-papertrail-host", "", "papertrail host")
	paperTrailPort = flag.Int("confbot-papertrail-port", 0, "papertrail port")
	doToken        = flag.String("confbot-digitalocean-token", "", "digitalocean token")
	redisURL       = flag.String("redis-url", "", "redis url")
	httpAddr       = flag.String("confbot-http-addr", "localhost:8080", "api listen address")
	remoteLogging  = flag.Bool("confbot-enable-remote-logging", false, "enable remote logging")

	rootLog = logrus.New()
)

func main() {
	envflag.Parse()

	log := rootLog.WithFields(logrus.Fields{
		"env": *env,
	})

	setupLogger(log)
	verifyEnv(log)

	ctx := context.WithValue(context.Background(), "log", log)

	s, err := slack.New(ctx, *slackToken)
	if err != nil {
		rootLog.WithError(err).Fatal("unable to connect to slack")
	}

	log.Info("application started")

	repo, err := confbot.NewRepo(ctx, *redisURL, *env)
	if err != nil {
		rootLog.WithError(err).Fatalf("unable to create repo")
	}

	cb := confbot.New(ctx, s, repo)

	cb.AddTextAction("./boot shell", confbot.CreateBootShellAction(ctx, *doToken, repo))
	go cb.Listen()

	a := api.New(ctx, repo, s)
	http.Handle("/", a.Mux)

	log.WithField("addr", *httpAddr).Info("created http server")
	log.Fatal(http.ListenAndServe(*httpAddr, nil))
}

func verifyEnv(log *logrus.Entry) {
	if *slackToken == "" {
		log.Fatalf("CONFBOT_SLACK_TOKEN environment variable is required")
	}

	if *remoteLogging && *paperTrailHost == "" {
		log.Fatalf("CONFBOT_PAPERTRAIL_HOST environment variable is required")
	}

	if *remoteLogging && *paperTrailPort == 0 {
		log.Fatalf("CONFBOT_PAPERTRAIL_PORT environment variable is required")
	}

	if *redisURL == "" {
		log.Fatalf("REDIS_URL environment variable is required")
	}
}

func setupLogger(log *logrus.Entry) {
	if *remoteLogging {
		hostname, err := os.Hostname()
		if err != nil {
			log.Fatalf("unable to retrieve app hostname: %v", err)
		}

		hook, err := logging.NewPapertrailHook(&logging.Hook{
			Host:     *paperTrailHost,
			Port:     *paperTrailPort,
			Hostname: hostname,
			Appname:  "confbot",
		})

		if err != nil {
			log.WithError(err).Fatalf("unable to set up papertrail logging")
		}

		rootLog.Hooks.Add(hook)
	}
}
