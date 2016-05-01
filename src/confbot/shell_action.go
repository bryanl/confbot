package confbot

import (
	"bytes"
	"confbot/slack"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"text/template"

	"github.com/Sirupsen/logrus"
	"github.com/digitalocean/godo"
	"github.com/digitalocean/godo/util"
	"github.com/satori/go.uuid"
	"golang.org/x/crypto/ssh"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

const (
	reactionUp    = "white_check_mark"
	reactionNew   = "warning"
	reactionReady = "100"
)

var (
	dropletRegion = "nyc1"
	dropletSize   = "4gb"
	dropletImage  = godo.DropletCreateImage{
		Slug: "ubuntu-14-04-x64",
	}
	dropletSSHKeys = []godo.DropletCreateSSHKey{
		{ID: 104064},
	}

	// TODO pass this in fom somewhere.
	dropletDomain = "x.pifft.com"
)

// CreateBootShellAction returns a function that boot a new shell.
func CreateBootShellAction(ctx context.Context, doToken string, repo Repo) ActionFn {
	log := logFromContext(ctx)

	return func(ctx context.Context, m *slack.Message, s *slack.Slack) error {
		id := projectID()
		sb := NewShellBooter(id, doToken, log)

		userID := m.User
		err := repo.RegisterProject(id, userID)

		log.WithFields(logrus.Fields{
			"user-id":    userID,
			"project-id": id,
		}).Info("new shell request")

		if err != nil {
			switch err.(type) {
			case *ProjectExistsErr:
				if _, sErr := s.IM(userID, fmt.Sprintf("unable to boot shell: %v", err)); sErr != nil {
					return sErr
				}

				id, err = repo.ProjectID(userID)
				if err != nil {
					return err
				}

				_, _ = s.IM(userID, fmt.Sprintf("You already have an existing shell at *%s*", id))

			default:
				if _, sErr := s.IM(userID, fmt.Sprintf("unknown error: %v", err)); sErr != nil {
					return sErr
				}
			}
			return err
		}

		var msg *slack.Message
		if msg, err = s.IM(userID, "booting shell for project _"+id+"_"); err != nil {
			return err
		}

		_ = s.AddReaction(msg.Timestamp, msg.Channel, reactionNew)

		var reply slack.OutgoingMessage
		sc, err := sb.Boot()
		if err != nil {
			log.WithError(err).Error("couldn't boot shell")
			msg := fmt.Sprintf("couldn't boot shell: %s", err)
			if _, err := s.IM(userID, msg); err != nil {
				return err
			}
		} else {
			_ = s.RemoveReaction(msg.Timestamp, msg.Channel, reactionNew)
			_ = s.AddReaction(msg.Timestamp, msg.Channel, reactionUp)
		}

		if err := s.Send(&reply); err != nil {
			return err
		}

		if err := repo.SaveKey(id, sc.KeyPair.private); err != nil {
			return err
		}

		r := bytes.NewReader(sc.KeyPair.private)
		if err := s.Upload("id_rsa", r, []string{msg.Channel}); err != nil {
			return err
		}

		return nil
	}
}

// ShellConfig is the generated configuration for a shell.
type ShellConfig struct {
	KeyPair   *KeyPair
	ProjectID string
	Hostname  string
}

// ShellBooter boots shells for demos.
type ShellBooter struct {
	id      string
	doToken string
	log     *logrus.Entry
}

// NewShellBooter creates an instance of ShellBooter,
func NewShellBooter(id, doToken string, log *logrus.Entry) *ShellBooter {
	return &ShellBooter{
		id:      id,
		doToken: doToken,
		log:     log,
	}
}

// Boot does the boot process.
func (sb *ShellBooter) Boot() (*ShellConfig, error) {
	id := sb.id

	kp, err := sb.makeSSHKeyPair()
	if err != nil {
		return nil, err
	}

	td := templateData{
		PubKey:               string(kp.public),
		EncodedProjectID:     base64.StdEncoding.EncodeToString([]byte(id)),
		EncodedToken:         base64.StdEncoding.EncodeToString([]byte(sb.doToken)),
		EncodedInstallScript: base64.StdEncoding.EncodeToString([]byte(runShellInstaller)),
	}

	t, err := generateTemplate(td)
	if err != nil {
		return nil, err
	}

	err = sb.bootDroplet(t, id)
	if err != nil {
		return nil, err
	}

	return &ShellConfig{
		KeyPair:   kp,
		ProjectID: id,
		Hostname:  fmt.Sprintf("shell-%s.%s", id, dropletDomain),
	}, nil
}

func (sb *ShellBooter) bootDroplet(t, id string) error {
	token := &oauth2.Token{AccessToken: sb.doToken}
	ts := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(oauth2.NoContext, ts)
	client := godo.NewClient(oauthClient)

	dropletName := fmt.Sprintf("shell-%s", id)
	sb.log.WithFields(logrus.Fields{
		"project_id": id,
	}).Info("creating shell droplet")

	cr := &godo.DropletCreateRequest{
		Name:     dropletName,
		Region:   dropletRegion,
		Size:     dropletSize,
		Image:    dropletImage,
		SSHKeys:  dropletSSHKeys,
		UserData: t,
	}

	_, resp, err := client.Droplets.Create(cr)
	if err != nil {
		return err
	}

	var action *godo.LinkAction
	for _, a := range resp.Links.Actions {
		if a.Rel == "create" {
			action = &a
			break
		}
	}

	if action == nil {
		return errors.New("unable to wait for droplet to be created because there is no create action")
	}

	sb.log.WithFields(logrus.Fields{
		"project_id": id,
		"action_id":  action.ID,
	}).Info("waiting for droplet to boot")

	err = util.WaitForActive(client, action.HREF)
	if err != nil {
		return err
	}

	sb.log.WithFields(logrus.Fields{
		"project_id": id,
	}).Info("droplet booted")

	return nil
}

// KeyPair is a SSH key pair.
type KeyPair struct {
	public  []byte
	private []byte
}

func (sb *ShellBooter) makeSSHKeyPair() (*KeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	kp := &KeyPair{}

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	kp.private = pem.EncodeToMemory(privateKeyPEM)
	kp.public = ssh.MarshalAuthorizedKey(pub)

	return kp, nil
}

func projectID() string {
	h := md5.New()
	io.WriteString(h, uuid.NewV4().String())
	str := fmt.Sprintf("%x", h.Sum(nil))
	return str[:7]
}

type templateData struct {
	PubKey               string
	EncodedProjectID     string
	EncodedToken         string
	EncodedInstallScript string
}

func generateTemplate(td templateData) (string, error) {
	t, err := template.New("output").Parse(userDataTemplate)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	err = t.Execute(&buf, td)
	if err != nil {
		return "", err
	}

	return buf.String(), nil
}

var userDataTemplate = `
#cloud-config

users:
  - name: workshop
    shell: /bin/bash
    sudo: ['ALL=(ALL) NOPASSWD:ALL']
    ssh-authorized-keys:
      - {{ .PubKey }}
write_files:
  - encoding: b64
    content: {{ .EncodedProjectID }}
    owner: root:root
    path: /etc/project-id
    permissions: '0644'
  - encoding: b64
    cotent: {{ .EncodedToken }}
    owner: root:root
    path: /etc/digitalocean-token
    permissions: '0644'
  - encoding: b64
    content: {{ .EncodedInstallScript }}
    owner: root:root
    path: /usr/local/bin/install-shell.sh
    permissions: '0755'
package_update: true
apt_sources:
  - source: "ppa:gluster/glusterfs-3.5"
  - source: "ppa:ansible/ansible"
packages:
  - glusterfs-client
  - glusterfs-server
  - ansible
runcmd:
  - [/usr/local/bin/install-shell.sh]
`

var runShellInstaller = `
#!/usr/bin/env bash

curl -s https://s3.pifft.com/oscon2016/install.sh | bash
`
