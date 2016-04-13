package logrus_papertrail

import (
	"fmt"
	"testing"

	"github.com/Sirupsen/logrus"
	"github.com/stvp/go-udp-testing"
)

func TestWritingToUDP(t *testing.T) {
	port := 16661
	udp.SetAddr(fmt.Sprintf(":%d", port))

	hook, err := NewPapertrailHook(&Hook{
		Host:     "localhost",
		Port:     port,
		Hostname: "test.local",
		Appname:  "test",
	})
	if err != nil {
		t.Errorf("Unable to connect to local UDP server.")
	}

	log := logrus.New()
	log.Hooks.Add(hook)

	udp.ShouldReceive(t, "foo", func() {
		log.Info("foo")
	})
}
