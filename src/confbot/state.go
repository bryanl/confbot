package confbot

import (
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/Sirupsen/logrus"
)

type stateFn func(*provision) stateFn

func initState(p *provision) stateFn {
	p.log.WithField("state", "initState").Info("running initState")
	if _, err := p.slack.IM(p.userID, "*--- provisioning process started ---*"); err != nil {
		return errorStateGen(err)
	}
	return infraState
}

func infraState(p *provision) stateFn {
	log := p.log.WithField("state", "infraState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	if _, err := p.slack.IM(p.userID, "*starting infrastructure provisioning*"); err != nil {
		return errorStateGen(err)
	}
	out, err := sshClient.Execute("shell", "cd /home/workshop/infra && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return errorStateGen(err)
	}

	r := strings.NewReader(out)
	if err := p.slack.Upload("infra-provision.txt", r, []string{p.channel}); err != nil {
		log.WithError(err).Error("upload infra-provision.txt")
		return errorStateGen(err)
	}

	if _, err := p.slack.IM(p.userID, "*infrastructure provisioned*"); err != nil {
		return errorStateGen(err)
	}

	return certsState
}

func certsState(p *provision) stateFn {
	log := p.log.WithField("state", "certState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	if _, err := p.slack.IM(p.userID, "*update root certificates*"); err != nil {
		return errorStateGen(err)
	}

	out, err := sshClient.Execute("shell", `sudo perl -pi -e 's/^\!//' /etc/ca-certificates.conf`)
	if err != nil {
		log.WithError(err).Error("verify ca certificates")
		return errorStateGen(err)

	}

	if err := uploadOutput(p, log, "verify-cert.txt", out); err != nil {
		return errorStateGen(err)
	}

	out, err = sshClient.Execute("shell", `sudo update-ca-certificates`)
	if err != nil {
		log.WithError(err).Error("update ca certificates")
		return errorStateGen(err)
	}

	if err := uploadOutput(p, log, "update-cert.txt", out); err != nil {
		return errorStateGen(err)
	}

	return ansibleState
}

func ansibleState(p *provision) stateFn {
	log := p.log.WithField("state", "ansibleState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	if _, err := p.slack.IM(p.userID, "*starting ansible -- this may take a few minutes*"); err != nil {
		return errorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "cd /home/workshop/ansible && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return errorStateGen(err)
	}

	r := strings.NewReader(out)
	if err := p.slack.Upload("ansible.txt", r, []string{p.channel}); err != nil {
		log.WithError(err).Error("upload ansible.txt")
		return errorStateGen(err)
	}

	if _, err := p.slack.IM(p.userID, "*ansible complete*"); err != nil {
		return errorStateGen(err)
	}

	return esState
}

func esState(p *provision) stateFn {
	log := p.log.WithField("state", "esState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	if _, err := p.slack.IM(p.userID, "*waiting for elasticsearch to boot*"); err != nil {
		return errorStateGen(err)
	}

	c := 1
	for {
		if c > 5 {
			return errorStateGen(errors.New("timed out while waiting for elasticsearch to answer"))
		}

		host := fmt.Sprintf("app.%s.%s:9200", p.projectID, dropletDomain)
		log.WithField("count", c).Info("check to see if elasticsearch is up")
		conn, err := net.DialTimeout("tcp", host, time.Minute*1)
		if err == nil {
			log.WithField("count", c).Info("elasticsearch is up")
			conn.Close()
			break
		}

		log.WithError(err).Warn("connection to elasticsearch")
		c++
	}

	if _, err := p.slack.IM(p.userID, "*elasticsearch is up and listening*"); err != nil {
		return errorStateGen(err)
	}

	if _, err := p.slack.IM(p.userID, "*uploading elasticsearch templates*"); err != nil {
		return errorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "curl https://s3.pifft.com/oscon2016/create-beats.sh | bash")
	if err != nil {
		log.WithError(err).Error("create beats")
		return errorStateGen(err)
	}

	r := strings.NewReader(out)
	if err := p.slack.Upload("es.txt", r, []string{p.channel}); err != nil {
		log.WithError(err).Error("upload es.txt")
		return errorStateGen(err)
	}

	if _, err := p.slack.IM(p.userID, "*templates have been uploaded*"); err != nil {
		return errorStateGen(err)
	}

	return completeState
}

func errorStateGen(err error) stateFn {
	return func(p *provision) stateFn {
		p.log.WithField("state", "errorState").WithError(err).Error("provision failed")
		_, _ = p.slack.IM(p.userID, "*--- provisioning process failed ---*")
		return nil
	}
}

func completeState(p *provision) stateFn {
	log := p.log.WithField("state", "completeState")
	log.Info("provision complete")
	_, _ = p.slack.IM(p.userID, "*--- provisioning process completed ---*")
	return nil
}

func uploadOutput(p *provision, log *logrus.Entry, name, out string) error {
	r := strings.NewReader(out)
	if err := p.slack.Upload(name, r, []string{p.channel}); err != nil {
		log.WithError(err).
			WithField("file-name", name).
			Errorf("provision upload")
		return err
	}

	return nil
}
