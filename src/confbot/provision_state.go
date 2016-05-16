package confbot

import (
	"errors"
	"fmt"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
)

type provisionStateFn func(*provision) provisionStateFn

func provisionInitState(p *provision) provisionStateFn {
	p.log.WithField("provision-state", "initState").Info("running initState")
	params := slack.NewPostMessageParameters()
	msg := "*Provisioning process started*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}
	return provisionInfraState
}

func provisionInfraState(p *provision) provisionStateFn {
	log := p.log.WithField("provision-state", "infraState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	msg := "*... Creating hosts and certificates*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}
	out, err := sshClient.Execute("shell", "cd /home/workshop/infra && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return provisionErrorStateGen(err)
	}

	if err := provisionUploadOutput(p, log, "infra-provision.txt", out); err != nil {
		return provisionErrorStateGen(err)
	}

	msg = "*... Hosts and certificats are up to date*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	return provisionCertsState
}

func provisionCertsState(p *provision) provisionStateFn {
	log := p.log.WithField("provision-state", "certState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	msg := "*...Making sure root certificates are up to date*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	_, err := sshClient.Execute("shell", `sudo perl -pi -e 's/^\!//' /etc/ca-certificates.conf`)
	if err != nil {
		log.WithError(err).Error("verify ca certificates")
		return provisionErrorStateGen(err)
	}

	out, err := sshClient.Execute("shell", `sudo update-ca-certificates`)
	if err != nil {
		log.WithError(err).Error("update ca certificates")
		return provisionErrorStateGen(err)
	}

	if err = provisionUploadOutput(p, log, "update-cert.txt", out); err != nil {
		return provisionErrorStateGen(err)
	}

	return provisionAnsibleState
}

func provisionAnsibleState(p *provision) provisionStateFn {
	log := p.log.WithField("provision-state", "ansibleState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	msg := "*... Provisioning services with Ansible. This may take some time*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "cd /home/workshop/ansible && ./setup.sh")
	if err != nil {
		log.WithError(err).Error("execute ssh")
		return provisionErrorStateGen(err)
	}

	if err := provisionUploadOutput(p, log, "ansible.txt", out); err != nil {
		return provisionErrorStateGen(err)
	}

	msg = "*... Ansible provisioning is complete*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	return provisionEsState
}

func provisionEsState(p *provision) provisionStateFn {
	log := p.log.WithField("provision-state", "esState")
	sshClient := NewSSHClient(p.ctx, p.projectID, p.repo)

	params := slack.NewPostMessageParameters()
	msg := "*... Waiting for ElasticSearch to become available*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	c := 1
	for {
		if c > 5 {
			return provisionErrorStateGen(errors.New("timed out while waiting for elasticsearch to answer"))
		}

		host := fmt.Sprintf("app.%s.%s:9200", p.projectID, DropletDomain)
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

	msg = "*... ElasticSearch is up and listening*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	msg = "*... Uploading ElasticSearch templates*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	out, err := sshClient.Execute("shell", "curl https://s3.pifft.com/oscon2016/create-beats.sh | bash")
	if err != nil {
		log.WithError(err).Error("create beats")
		return provisionErrorStateGen(err)
	}

	if err := provisionUploadOutput(p, log, "es.txt", out); err != nil {
		return provisionErrorStateGen(err)
	}

	msg = "* ... ElasticSearch templates have been uploaded*"
	if _, _, err := p.slack.PostMessage(p.channel, msg, params); err != nil {
		return provisionErrorStateGen(err)
	}

	return provisionCompleteState
}

func provisionErrorStateGen(err error) provisionStateFn {
	return func(p *provision) provisionStateFn {
		params := slack.NewPostMessageParameters()
		p.log.WithField("provision-state", "errorState").WithError(err).Error("provision failed")
		msg := "*Provisioning process Failed* All was not well with the provisioning process. " +
			"This is expected as the cloud is a chaotic environment. To restart the provision process " +
			"issue the `./provision` command."
		_, _, _ = p.slack.PostMessage(p.channel, msg, params)
		return nil
	}
}

func provisionCompleteState(p *provision) provisionStateFn {
	log := p.log.WithField("provision-state", "completeState")
	log.Info("provision complete")
	params := slack.NewPostMessageParameters()
	msg := "Your environment is ready to go. Before you can use it, you will " +
		"need to configure your ssh client. I can assist you with directions " +
		"for Linux, Mac, or Windows. To start this process, issue the `./configure ssh <type>` " +
		"command substituting <type> with *linux*, *mac*, or *windows*."
	_, _, _ = p.slack.PostMessage(p.channel, msg, params)
	return nil
}

func provisionUploadOutput(p *provision, log *logrus.Entry, name, out string) error {
	uploadParams := slack.FileUploadParameters{
		Filename: name,
		Content:  out,
		Channels: []string{p.channel},
	}

	if f, err := p.slack.UploadFile(uploadParams); err != nil {
		log.WithError(err).
			WithFields(logrus.Fields{
				"filename": name,
				"content":  fmt.Sprintf("%#v\n", f)}).
			Error("upload failed")
		return err
	}

	return nil
}
