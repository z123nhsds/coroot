package notifications

import (
	"context"
	"fmt"
	"strings"

	goteamsnotify "github.com/atc0005/go-teams-notify/v2"
	"github.com/atc0005/go-teams-notify/v2/adaptivecard"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
)

type Teams struct {
	client     *goteamsnotify.TeamsClient
	webhookUrl string
}

func NewTeams(webhookUrl string) *Teams {
	client := goteamsnotify.NewTeamsClient()
	client.SkipWebhookURLValidationOnSend(true)
	return &Teams{webhookUrl: webhookUrl, client: client}
}

func (t *Teams) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	var title string
	if n.Status == model.OK {
		title = fmt.Sprintf("**%s** incident resolved", n.ApplicationId.Name)
	} else {
		title = fmt.Sprintf("[%s] **%s** is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name)
	}
	text := ""
	if n.Details != nil {
		for _, r := range n.Details.Reports {
			text += fmt.Sprintf("* **%s** / %s: %s\n", r.Name, r.Check, r.Message)
		}
		if n.Details.RCASummary != "" {
			text += fmt.Sprintf("\n**Root Cause**: %s\n", n.Details.RCASummary)
			if n.Details.RCARemediations != "" {
				text += fmt.Sprintf("\n**Remediations**: %s\n", utils.Truncate(n.Details.RCARemediations, 2000))
			}
		}
	}
	if text == "" {
		text = " "
	}
	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(incidentUrl(baseUrl, n), "View incident")
	if err != nil {
		return err
	}
	if err = card.AddAction(true, action); err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	return t.client.SendWithContext(ctx, t.webhookUrl, msg)
}

func (t *Teams) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	displayName := alertDisplayName(n)
	url := alertNotificationUrl(baseUrl, n)
	var title string
	if n.Status == model.OK {
		resolvedText := "resolved"
		if n.Details != nil && n.Details.ResolvedBy != "" {
			resolvedText = fmt.Sprintf("manually resolved by **%s**", n.Details.ResolvedBy)
		}
		if n.Details != nil && n.Details.Duration != "" {
			title = fmt.Sprintf("**%s** alert %s (duration: %s)", displayName, resolvedText, n.Details.Duration)
		} else {
			title = fmt.Sprintf("**%s** alert %s", displayName, resolvedText)
		}
	} else {
		summary := "alert fired"
		if n.Details != nil && n.Details.Summary != "" {
			summary = n.Details.Summary
		}
		title = fmt.Sprintf("[%s] **%s**: %s", strings.ToUpper(n.Status.String()), displayName, summary)
	}
	text := alertMarkdownDetails(n)
	if text == "" {
		text = " "
	}
	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(url, "View alert")
	if err != nil {
		return err
	}
	if err = card.AddAction(true, action); err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	return t.client.SendWithContext(ctx, t.webhookUrl, msg)
}

func (t *Teams) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	d := ds.Deployment

	status := "Deployed"
	switch ds.State {
	case model.ApplicationDeploymentStateInProgress:
		return nil
	case model.ApplicationDeploymentStateStuck:
		status = "Stuck"
	case model.ApplicationDeploymentStateCancelled:
		status = "Cancelled"
	}

	title := fmt.Sprintf("Deployment of **%s** to **%s**", d.ApplicationId.Name, project.Name)
	text := fmt.Sprintf("**Status**: %s\n\n", status)
	text += fmt.Sprintf("**Version**: %s\n\n", d.Version())
	if ds.State == model.ApplicationDeploymentStateSummary {
		summary := ""
		if len(ds.Summary) > 0 {
			for _, s := range ds.Summary {
				summary += fmt.Sprintf("* %s %s\n", s.Emoji(), s.Message)
			}
		} else {
			summary = "No notable changes"
		}
		text += "**Summary:**\n\n"
		text += summary
	}

	card, err := adaptivecard.NewTextBlockCard(text, title, true)
	if err != nil {
		return err
	}
	action, err := adaptivecard.NewActionOpenURL(deploymentUrl(project.Settings.Integrations.BaseUrl, project.Id, d), "View deployment")
	if err != nil {
		return err
	}
	if err = card.AddAction(true, action); err != nil {
		return err
	}
	msg, err := adaptivecard.NewMessageFromCard(card)
	if err != nil {
		return err
	}
	return t.client.SendWithContext(ctx, t.webhookUrl, msg)
}
