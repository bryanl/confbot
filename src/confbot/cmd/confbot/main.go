package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/kouhin/envflag"
	"gopkg.in/polds/logrus-papertrail-hook.v2"

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

	hook, err := logrus_papertrail.NewPapertrailHook(&logrus_papertrail.Hook{
		Host:     *paperTrailHost,
		Port:     *paperTrailPort,
		Hostname: hostname,
		Appname:  "confbot",
	})

	if err != nil {
		log.WithError(err).Fatalf("unable to set up logging")
	}

	log.Hooks.Add(hook)

	s, err := slack.New(*slackToken)
	if err != nil {
		log.WithError(err).Fatal("unable to connect to slack")
	}

	for {
		m, err := s.Receive()
		if err != nil {
			log.WithError(err).Error("error receiving message from slack")
		}

		go func() {
			if m.Text == "hello world" {
				ca := slack.CallArgs{
					"user": m.User,
				}
				cr, err := s.Call("users.info", ca)
				if err != nil {
					log.WithError(err).
						Error("unable to lookup user")
				}

				user := cr["user"].(map[string]interface{})
				profile := user["profile"].(map[string]interface{})
				firstName := profile["first_name"].(string)

				reply := slack.Message{
					Channel: m.Channel,
					Type:    "message",
					Text:    fmt.Sprintf("Hello %s", firstName),
				}

				err = s.Send(reply)
				if err != nil {
					log.WithError(err).WithFields(logrus.Fields{}).Error("unable to send message")
				}
			}
		}()
	}
}
