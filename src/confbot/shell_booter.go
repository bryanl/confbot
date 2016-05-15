package confbot

import (
	"bytes"
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
	"golang.org/x/oauth2"
)

var (
	dropletSize  = "4gb"
	dropletImage = godo.DropletCreateImage{
		Slug: "ubuntu-14-04-x64",
	}
	dropletSSHKeys = []godo.DropletCreateSSHKey{}

	// DropletDomain is the domain for droplets.
	DropletDomain = "x.pifft.com"

	// WebhookURL is the URL for the bot.
	WebhookURL = "https://devconfbot.ngrok.io/webhook"

	masterKey = "ssh-rsa AAAAB3NzaC1yc2EAAAABIwAAAQEA3ntoDAFrpg2wDQnqU3+T4wci5qzTThKaJmivUHIVhDoA91fHX89Crtr5GiSt997uG30xm2y1UNWOnbVLrX8UgCEX4/cYTKDtYyD4uHYOez/TyiJANO0mynWBOufMkt3O3Xz/Vp/bpfWqqQLDUUi0DwfpBHa7ZRDFBdu5IQtBGsMzEAbSnf1VCU5YC86NdRiuvSFAu9xq3QS80hBdfY77x5cge6iHNEnTE0yOnY7X+LpEXxJLqlQq81eX3UYjbhrpBX1konAn0UsNtDPDwzAqKYFZNnIPrLqKI+h1ZK4oAc9YziH9kx4DMB3kq8JgqZCg7ViMQQHZzccU1t4bDDn3QQ== bryan@dmac.local"
)

// ShellConfig is the generated configuration for a shell.
type ShellConfig struct {
	KeyPair   *KeyPair
	ProjectID string
	Hostname  string
}

// ShellBooter boots shells for demos.
type ShellBooter struct {
	id            string
	doToken       string
	log           *logrus.Entry
	dropletRegion string
	client        *godo.Client
	masterClient  *godo.Client
}

func buildDoClient(pat string) *godo.Client {
	token := &oauth2.Token{AccessToken: pat}
	ts := oauth2.StaticTokenSource(token)
	oauthClient := oauth2.NewClient(oauth2.NoContext, ts)
	return godo.NewClient(oauthClient)
}

// NewShellBooter creates an instance of ShellBooter,
func NewShellBooter(id, doToken, masterToken, dropletRegion string, log *logrus.Entry) *ShellBooter {

	return &ShellBooter{
		id:            id,
		doToken:       doToken,
		client:        buildDoClient(doToken),
		masterClient:  buildDoClient(masterToken),
		log:           log,
		dropletRegion: dropletRegion,
	}
}

// Boot does the boot process.
func (sb *ShellBooter) Boot() (*ShellConfig, error) {
	id := sb.id

	kp, err := sb.makeSSHKeyPair()
	if err != nil {
		return nil, err
	}

	if sb.doToken == "" {
		return nil, fmt.Errorf("invalid do token")
	}

	td := templateData{
		PubKey:               string(kp.public),
		MasterKey:            masterKey,
		EncodedProjectID:     base64.StdEncoding.EncodeToString([]byte(id)),
		EncodedToken:         base64.StdEncoding.EncodeToString([]byte(sb.doToken)),
		EncodedInstallScript: base64.StdEncoding.EncodeToString([]byte(runShellInstaller)),
		EncodedRegion:        base64.StdEncoding.EncodeToString([]byte(sb.dropletRegion)),
		EncodedWebhookURL:    base64.StdEncoding.EncodeToString([]byte(WebhookURL)),
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
		Hostname:  fmt.Sprintf("shell-%s.%s", id, DropletDomain),
	}, nil
}

func (sb *ShellBooter) bootDroplet(t, id string) error {
	dropletName := fmt.Sprintf("shell.%s", id)
	sb.log.WithFields(logrus.Fields{
		"project_id": id,
		"region":     sb.dropletRegion,
	}).Info("creating shell droplet")

	cr := &godo.DropletCreateRequest{
		Name:     dropletName,
		Region:   sb.dropletRegion,
		Size:     dropletSize,
		Image:    dropletImage,
		SSHKeys:  dropletSSHKeys,
		UserData: t,
	}

	d, resp, err := sb.client.Droplets.Create(cr)
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

	err = util.WaitForActive(sb.client, action.HREF)
	if err != nil {
		return err
	}

	sb.log.WithFields(logrus.Fields{
		"project_id": id,
	}).Info("droplet booted")

	d, _, err = sb.client.Droplets.Get(d.ID)
	if err != nil {
		return err
	}

	ip, err := d.PublicIPv4()
	if err != nil {
		return err
	}

	drer := &godo.DomainRecordEditRequest{
		Type: "A",
		Name: dropletName,
		Data: ip,
	}
	_, _, err = sb.masterClient.Domains.CreateRecord(DropletDomain, drer)

	return err
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
	MasterKey            string
	EncodedProjectID     string
	EncodedToken         string
	EncodedInstallScript string
	EncodedRegion        string
	EncodedWebhookURL    string
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
      - {{ .MasterKey }}
write_files:
  - encoding: b64
    content: {{ .EncodedProjectID }}
    owner: root:root
    path: /etc/project-id
    permissions: '0644'
  - encoding: b64
    content: {{ .EncodedRegion }}
    owner: root:root
    path: /etc/project-region
    permissions: '0644'
  - encoding: b64
    content: {{ .EncodedToken }}
    owner: root:root
    path: /etc/digitalocean-token
    permissions: '0644'
  - encoding: b64
    content: {{ .EncodedWebhookURL }}
    owner: root:root
    path: /etc/confbot-webhook-url
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
