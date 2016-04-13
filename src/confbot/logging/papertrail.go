package logging

import (
	"crypto/tls"
	"fmt"
	"os"
	"time"

	"github.com/Sirupsen/logrus"
)

const (
	format = "Jan 2 15:04:05"
)

// Hook to send logs to a logging service compatible with the Papertrail API.
type Hook struct {
	// Connection Details
	Host string
	Port int

	// App Details
	Appname  string
	Hostname string

	tlsConn *tls.Conn
}

// NewPapertrailHook creates a hook to be added to an instance of logger.
func NewPapertrailHook(hook *Hook) (*Hook, error) {
	var err error

	hook.tlsConn, err = tls.Dial("tcp", fmt.Sprintf("%s:%d", hook.Host, hook.Port), nil)
	return hook, err
}

// Fire is called when a log event is fired.
func (hook *Hook) Fire(entry *logrus.Entry) error {
	date := time.Now().Format(format)
	msg, _ := entry.String()
	payload := fmt.Sprintf("<22> %s %s %s: %s", date, hook.Hostname, hook.Appname, msg)

	bytesWritten, err := hook.tlsConn.Write([]byte(payload))

	// bytesWritten, err := hook.udpConn.Write([]byte(payload))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to send log line to Papertrail via TCP over TLS. Wrote %d bytes before error: %v", bytesWritten, err)
		return err
	}

	return nil
}

// Levels returns the available logging levels.
func (hook *Hook) Levels() []logrus.Level {
	return []logrus.Level{
		logrus.PanicLevel,
		logrus.FatalLevel,
		logrus.ErrorLevel,
		logrus.WarnLevel,
		logrus.InfoLevel,
		logrus.DebugLevel,
	}
}
