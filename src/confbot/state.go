package confbot

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
)

type stateFn func(*provision) stateFn

func initState(p *provision) stateFn {
	p.log.WithField("state", "initState").Info("running initState")
	params := slack.NewPostMessageParameters()
	if _, _, err := p.slack.PostMessage(p.channel, "*--- provisioning process started ---*", params); err != nil {
		return errorStateGen(err)
	}
	return infraState
}

func infraState(p *provision) stateFn {
	log := p.log.WithField("state", "infraState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	if _, _, err := p.slack.PostMessage(p.channel, "*starting infrastructure provisioning*", params); err != nil {
		return errorStateGen(err)
	}
	out, err := sshClient.Execute("shell", "cd /home/workshop/infra && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return errorStateGen(err)
	}

	if err := uploadOutput(p, log, "infra-provision.txt", out); err != nil {
		return errorStateGen(err)
	}

	if _, _, err := p.slack.PostMessage(p.channel, "*infrastructure provisioned*", params); err != nil {
		return errorStateGen(err)
	}

	return certsState
}

func certsState(p *provision) stateFn {
	log := p.log.WithField("state", "certState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	if _, _, err := p.slack.PostMessage(p.channel, "*update root certificates*", params); err != nil {
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

	params := slack.NewPostMessageParameters()
	if _, _, err := p.slack.PostMessage(p.channel, "*starting ansible -- this may take a few minutes*", params); err != nil {
		return errorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "cd /home/workshop/ansible && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return errorStateGen(err)
	}

	if err := uploadOutput(p, log, "ansible.txt", out); err != nil {
		return errorStateGen(err)
	}

	if _, _, err := p.slack.PostMessage(p.channel, "*ansible complete*", params); err != nil {
		return errorStateGen(err)
	}

	return esState
}

func esState(p *provision) stateFn {
	log := p.log.WithField("state", "esState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	if _, _, err := p.slack.PostMessage(p.channel, "*waiting for elasticsearch to boot*", params); err != nil {
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

	if _, _, err := p.slack.PostMessage(p.channel, "*elasticsearch is up and listening*", params); err != nil {
		return errorStateGen(err)
	}

	if _, _, err := p.slack.PostMessage(p.channel, "*uploading elasticsearch templates*", params); err != nil {
		return errorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "curl https://s3.pifft.com/oscon2016/create-beats.sh | bash")
	if err != nil {
		log.WithError(err).Error("create beats")
		return errorStateGen(err)
	}

	if err := uploadOutput(p, log, "es.txt", out); err != nil {
		return errorStateGen(err)
	}

	if _, _, err := p.slack.PostMessage(p.channel, "*templates have been uploaded*", params); err != nil {
		return errorStateGen(err)
	}

	return completeState
}

func errorStateGen(err error) stateFn {
	return func(p *provision) stateFn {
		params := slack.NewPostMessageParameters()
		p.log.WithField("state", "errorState").WithError(err).Error("provision failed")
		_, _, _ = p.slack.PostMessage(p.channel, "*--- provisioning process failed ---*", params)
		return nil
	}
}

func completeState(p *provision) stateFn {
	log := p.log.WithField("state", "completeState")
	log.Info("provision complete")
	params := slack.NewPostMessageParameters()
	_, _, _ = p.slack.PostMessage(p.channel, "*--- provisioning process completed ---*", params)
	return nil
}

func uploadOutput(p *provision, log *logrus.Entry, name, out string) error {
	uploadParams := slack.FileUploadParameters{
		Filename: name,
		Content:  out,
	}

	if _, err := p.slack.UploadFile(uploadParams); err != nil {
		log.WithError(err).WithField("filename", name).Error("upload failed")
		return err
	}

	return nil
}
