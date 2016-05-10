package confbot

import (
	"fmt"
	"strings"

	"github.com/digitalocean/godo"
	"github.com/nlopes/slack"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

// CreateDeleteAction returns a function that deletes a project.
func CreateDeleteAction(ctx context.Context, doToken string, repo Repo) ActionFn {
	return func(ctx context.Context, m *slack.MessageEvent, slackClient *slack.Client, matches [][]string) error {
		var err error
		userID := m.User

		log := logFromContext(ctx).WithField("user-id", userID)

		_, _, channelID, err := slackClient.OpenIMChannel(m.User)
		if err != nil {
			return err
		}

		projectID, err := repo.ProjectID(userID)
		if err != nil {
			return err
		}

		params := slack.PostMessageParameters{}

		defer func() {
			if err != nil {
				log.WithError(err).Error("unable to delete project")
				params = slack.PostMessageParameters{}
				slackClient.PostMessage(channelID, fmt.Sprintf("unable to delete project _%s_", projectID), params)
			}
		}()

		if _, _, err = slackClient.PostMessage(channelID, fmt.Sprintf("Deleting project _%s_ and it's associated resources", projectID), params); err != nil {
			return err
		}

		token := &oauth2.Token{AccessToken: doToken}
		ts := oauth2.StaticTokenSource(token)
		oauthClient := oauth2.NewClient(oauth2.NoContext, ts)
		client := godo.NewClient(oauthClient)

		if _, _, err = slackClient.PostMessage(channelID, "*... Deleting DNS records*", params); err != nil {
			return err
		}

		if err = deleteRecords(client, projectID, dropletDomain); err != nil {
			return err
		}
		if _, _, err = slackClient.PostMessage(channelID, "*... Deleting SSH Keys*", params); err != nil {
			return err
		}

		if err = deleteKeys(client, projectID); err != nil {
			return err
		}

		if _, _, err = slackClient.PostMessage(channelID, "*... Deleting Droplets*", params); err != nil {
			return err
		}

		if err = deleteDroplets(client, projectID); err != nil {
			return err
		}

		if _, _, err = slackClient.PostMessage(channelID, fmt.Sprintf("*... Resetting project for _%s_ *", m.Name), params); err != nil {
			return err
		}

		if err = repo.ResetProject(userID); err != nil {
			return err
		}

		params = slack.PostMessageParameters{}
		if _, _, err = slackClient.PostMessage(channelID, fmt.Sprintf("Project _%s_ has been deleted. Send command `./boot shell` to start a new project.", projectID), params); err != nil {
			return err
		}

		return nil
	}
}

func deleteRecords(client *godo.Client, projectID, domain string) error {
	recs, err := listRecords(client)
	if err != nil {
		return err
	}

	for _, rec := range recs {
		if strings.HasSuffix(rec.Name, projectID) {
			_, err := client.Domains.DeleteRecord(dropletDomain, rec.ID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func listRecords(client *godo.Client) ([]godo.DomainRecord, error) {
	list := []godo.DomainRecord{}
	opt := &godo.ListOptions{}
	for {
		recs, resp, err := client.Domains.Records(dropletDomain, opt)
		if err != nil {
			return nil, err
		}

		for _, rec := range recs {
			list = append(list, rec)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return list, nil
}

func deleteKeys(client *godo.Client, projectID string) error {
	keys, err := listKeys(client)
	if err != nil {
		return err
	}

	for _, key := range keys {
		if key.Name == fmt.Sprintf("oscon-%s", projectID) {
			_, err := client.Keys.DeleteByID(key.ID)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func listKeys(client *godo.Client) ([]godo.Key, error) {
	list := []godo.Key{}
	opt := &godo.ListOptions{}
	for {
		keys, resp, err := client.Keys.List(opt)
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			list = append(list, key)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return list, nil
}

func deleteDroplets(client *godo.Client, projectID string) error {
	droplets, err := listDroplets(client)
	if err != nil {
		return err
	}

	for _, droplet := range droplets {
		if strings.HasSuffix(droplet.Name, projectID) {
			if _, err := client.Droplets.Delete(droplet.ID); err != nil {
				return err
			}
		}
	}

	return nil
}

func listDroplets(client *godo.Client) ([]godo.Droplet, error) {
	list := []godo.Droplet{}
	opt := &godo.ListOptions{}
	for {
		keys, resp, err := client.Droplets.List(opt)
		if err != nil {
			return nil, err
		}

		for _, key := range keys {
			list = append(list, key)
		}

		if resp.Links == nil || resp.Links.IsLastPage() {
			break
		}

		page, err := resp.Links.CurrentPage()
		if err != nil {
			return nil, err
		}

		opt.Page = page + 1
	}

	return list, nil
}
