package confbot

import "strings"

type stateFn func(*provision) stateFn

func initState(p *provision) stateFn {
	p.log.WithField("state", "initState").Info("running initState")
	if _, err := p.slack.IM(p.userID, "*--- provisioning process completed ---*"); err != nil {
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
