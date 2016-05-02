package confbot

import (
	"bytes"
	"fmt"

	"github.com/Sirupsen/logrus"

	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
)

const (
	defaultSSHPort = 22
)

// SSHClient is a SSH client.
type SSHClient struct {
	projectID string
	repo      Repo
	log       *logrus.Entry
}

// NewSSHClient builds an instance of SSHClient.
func NewSSHClient(ctx context.Context, projectID string, repo Repo) *SSHClient {
	return &SSHClient{
		projectID: projectID,
		repo:      repo,
		log:       logFromContext(ctx).WithField("project-id", projectID),
	}
}

// Execute executes a command on a remote ssh host.
func (s *SSHClient) Execute(host, cmd string) (string, error) {
	hostname := fmt.Sprintf("%s.%s.%s:%d", host, s.projectID, dropletDomain, defaultSSHPort)

	pemBytes, err := s.repo.GetKey(s.projectID)
	if err != nil {
		return "", err
	}

	signer, err := ssh.ParsePrivateKey(pemBytes)
	if err != nil {
		return "", fmt.Errorf("parse key failed: %v", err)
	}

	config := &ssh.ClientConfig{
		User: "workshop",
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}

	s.log.WithField("hostname", hostname).Info("dialing")
	conn, err := ssh.Dial("tcp", hostname, config)
	if err != nil {
		return "", err
	}

	s.log.WithField("hostname", hostname).Info("creating session")
	session, err := conn.NewSession()
	if err != nil {
		return "", err
	}

	defer session.Close()

	var buf bytes.Buffer
	session.Stdout = &buf

	s.log.WithFields(logrus.Fields{
		"hostname": hostname,
		"cmd":      cmd}).
		Info("running command")
	err = session.Run(cmd)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}
