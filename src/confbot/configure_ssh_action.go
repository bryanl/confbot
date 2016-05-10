package confbot

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/nlopes/slack"
	"golang.org/x/net/context"
)

// CreateConfigureSSHAction creates a configure action.
func CreateConfigureSSHAction(ctx context.Context, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client, matches [][]string) error {
		if len(matches) == 0 {
			return fmt.Errorf("nothing to configure")
		}

		userID := m.User

		log := logFromContext(ctx).WithField("user-id", userID)

		_, _, channelID, err := slackClient.OpenIMChannel(userID)
		if err != nil {
			return err
		}

		projectID, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		privKey, err := repo.GetKey(projectID)
		if err != nil {
			return err
		}

		subject := matches[0][1]
		switch subject {
		case "windows":
			if err := uploadPPK(log, slackClient, channelID, privKey); err != nil {
				return err
			}

			msg := fmt.Sprintf("Download the Putty key file oscon2016.ppk. "+
				"Launch Putty and enter `shell.%s.%s` as your "+
				"Host Name. Next, navigate to the SSH / Auth Category, and "+
				"browse for your oscon2016.ppk in the `Private key file for authentication` "+
				"text box. Afterwards, click open, and enter `workshop` as your user name.",
				projectID, dropletDomain)

			params := slack.NewPostMessageParameters()
			slackClient.PostMessage(channelID, msg, params)

		case "mac":
			if err := uploadPrivateKey(log, slackClient, channelID, privKey); err != nil {
				return err
			}

			msg := fmt.Sprintf("Download the SSH private key id_rsa. Most likely, "+
				"id_rsa will be download to $HOME/Downloads. In your terminal, "+
				"run `chmod 600 id_rsa`. You can SSH to your shell Droplet by running "+
				"`ssh -i id_rsa workshop@shell.%s.%s`.",
				projectID, dropletDomain)

			params := slack.NewPostMessageParameters()
			slackClient.PostMessage(channelID, msg, params)

		case "linux":
			if err := uploadPrivateKey(log, slackClient, channelID, privKey); err != nil {
				return err
			}

			msg := fmt.Sprintf("Download the SSH private key id_rsa. In your terminal, "+
				"navigate to your download folder and run `chmod 600 id_rsa`. "+
				"You can SSH to your shell Droplet by running "+
				"`ssh -i id_rsa workshop@shell.%s.%s`. ",
				projectID, dropletDomain)

			params := slack.NewPostMessageParameters()
			slackClient.PostMessage(channelID, msg, params)

		default:
			params := slack.NewPostMessageParameters()
			msg := fmt.Sprintf("I don't know how to configure ssh for *%s*, but I bet it's very similar to linux. Try `./configure ssh linux`", subject)
			slackClient.PostMessage(channelID, msg, params)
		}

		return nil
	}
}

func uploadPrivateKey(log *logrus.Entry, slackClient *slack.Client, channelID string, privKey []byte) error {
	uploadParams := slack.FileUploadParameters{
		Title:    "id_rsa",
		Filename: "id_rsa",
		Content:  string(privKey),
		Channels: []string{channelID},
	}

	if _, err := slackClient.UploadFile(uploadParams); err != nil {
		log.WithError(err).
			WithFields(logrus.Fields{
			"filename": "id_rsa"}).
			Error("key upload failed")

		return err
	}

	return nil
}

func uploadPPK(log *logrus.Entry, slackClient *slack.Client, channelID string, privKey []byte) error {
	dir, err := ioutil.TempDir("", channelID)
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)

	privKeyFilePath := filepath.Join(dir, "id_rsa")
	ppkFilePath := filepath.Join(dir, "oscon2016.ppk")

	if err := ioutil.WriteFile(privKeyFilePath, privKey, 0600); err != nil {
		return err
	}

	cmd := exec.Command("puttygen", privKeyFilePath, "-o", ppkFilePath)
	if err := cmd.Run(); err != nil {
		return err
	}

	b, err := ioutil.ReadFile(ppkFilePath)
	if err != nil {
		return err
	}

	uploadParams := slack.FileUploadParameters{
		Title:    "oscon2016.ppk",
		Filename: "oscon2016.ppk",
		Content:  string(b),
		Channels: []string{channelID},
	}

	if _, err := slackClient.UploadFile(uploadParams); err != nil {
		log.WithError(err).
			WithFields(logrus.Fields{
			"filename": "oscon2016.ppk"}).
			Error("ppk upload failed")

		return err
	}

	return nil
}
