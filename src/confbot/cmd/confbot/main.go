package main

import (
	"confbot"
	"flag"
	"os"

	"golang.org/x/net/context"

	"github.com/Sirupsen/logrus"
	"github.com/kouhin/envflag"

	"confbot/logging"
	"confbot/slack"
)

const (
	appName = "confbot"
)

func main() {
	var (
		slackToken     = flag.String("confbot-slack-token", "", "slack token")
		paperTrailHost = flag.String("confbot-papertrail-host", "", "papertrail host")
		paperTrailPort = flag.Int("confbot-papertrail-port", 0, "papertrail port")
	)
	envflag.Parse()

	log := logrus.New()
	log.Formatter = &logrus.JSONFormatter{}

	if *slackToken == "" {
		log.Fatalf("CONFBOT_SLACK_TOKEN environment variable is required")
	}

	if *paperTrailHost == "" {
		log.Fatalf("CONFBOT_PAPERTRAIL_HOST environment variable is required")
	}

	if *paperTrailPort == 0 {
		log.Fatalf("CONFBOT_PAPERTRAIL_PORT environment variable is required")
	}

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
		log.WithError(err).Fatalf("unable to set up logging")
	}

	log.Hooks.Add(hook)

	ctx := context.WithValue(context.Background(), "log", log)

	s, err := slack.New(ctx, *slackToken)
	if err != nil {
		log.WithError(err).Fatal("unable to connect to slack")
	}

	log.Info("application started")

	cb := confbot.New(ctx, s)
	cb.Listen()
}
