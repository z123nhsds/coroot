package notifications

import (
	"cmp"
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const (
	sendTimeout   = 30 * time.Second
	retryInterval = time.Minute
	retryWindow   = timeseries.Hour
)

type NotificationClient interface {
	SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error
	SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error
	SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error
}

type NotificationType int

const (
	NotificationTypeIncident NotificationType = iota
	NotificationTypeAlert
)

func getClient(destination db.IncidentNotificationDestination, integrations db.Integrations, notificationType NotificationType) NotificationClient {
	switch destination.IntegrationType {
	case db.IntegrationTypeSlack:
		if cfg := integrations.Slack; cfg != nil && isEnabled(cfg.Incidents, cfg.Alerts, notificationType) {
			return NewSlack(cfg.Token, cmp.Or(destination.SlackChannel, cfg.DefaultChannel))
		}
	case db.IntegrationTypeTeams:
		if cfg := integrations.Teams; cfg != nil && isEnabled(cfg.Incidents, cfg.Alerts, notificationType) {
			return NewTeams(cfg.WebhookUrl)
		}
	case db.IntegrationTypePagerduty:
		if cfg := integrations.Pagerduty; cfg != nil && isEnabled(cfg.Incidents, cfg.Alerts, notificationType) {
			return NewPagerduty(cfg.IntegrationKey)
		}
	case db.IntegrationTypeOpsgenie:
		if cfg := integrations.Opsgenie; cfg != nil && isEnabled(cfg.Incidents, cfg.Alerts, notificationType) {
			return NewOpsgenie(cfg.ApiKey, cfg.EUInstance)
		}
	case db.IntegrationTypeWebhook:
		if cfg := integrations.Webhook; cfg != nil && isEnabled(cfg.Incidents, cfg.Alerts, notificationType) {
			return NewWebhook(cfg)
		}
	}
	return nil
}

func isEnabled(incidents bool, alerts *bool, notificationType NotificationType) bool {
	switch notificationType {
	case NotificationTypeIncident:
		return incidents
	case NotificationTypeAlert:
		return alerts != nil && *alerts
	}
	return false
}

func incidentDetails(app *model.Application, incident *model.ApplicationIncident) *db.IncidentNotificationDetails {
	var reports []db.IncidentNotificationDetailsReport
	if !incident.Resolved() {
		for _, r := range app.Reports {
			for _, ch := range r.Checks {
				if r.Name == model.AuditReportSLO || ch.Status < model.WARNING {
					continue
				}
				reports = append(reports, db.IncidentNotificationDetailsReport{Name: r.Name, Check: ch.Title, Message: ch.Message})
			}
		}
	}
	if len(reports) == 0 && incident.RCA == nil {
		return nil
	}
	details := &db.IncidentNotificationDetails{Reports: reports}
	if rca := incident.RCA; rca != nil && rca.Status == "OK" {
		details.RCASummary = rca.ShortSummary
		details.RCARemediations = rca.ImmediateFixes
	}
	return details
}

func incidentUrl(baseUrl string, n *db.IncidentNotification) string {
	return fmt.Sprintf("%s/p/%s/incidents?incident=%s", baseUrl, n.ProjectId, n.IncidentKey)
}

func deploymentUrl(baseUrl string, projectId db.ProjectId, d *model.ApplicationDeployment) string {
	return fmt.Sprintf("%s/p/%s/app/%s/Deployments#%s", baseUrl, projectId, d.ApplicationId.String(), d.Id())
}

func applicationUrl(baseUrl string, projectId db.ProjectId, appId model.ApplicationId) string {
	return fmt.Sprintf("%s/p/%s/app/%s", baseUrl, projectId, appId.String())
}

func alertUrl(baseUrl string, n *db.AlertNotification) string {
	return fmt.Sprintf("%s/p/%s/alerts?alert=%s", baseUrl, n.ProjectId, n.AlertId)
}

func alertNotificationUrl(baseUrl string, n *db.AlertNotification) string {
	if n.Details != nil && n.Details.URL != "" {
		return n.Details.URL
	}
	return alertUrl(baseUrl, n)
}

func alertDisplayName(n *db.AlertNotification) string {
	if n.ApplicationId.Name != "" {
		return n.ApplicationId.Name
	}
	if n.Details != nil && n.Details.RuleName != "" {
		return n.Details.RuleName
	}
	return "Alert"
}

func alertIncidentDetails(n *db.AlertNotification) *db.IncidentNotificationDetails {
	if n.Details == nil {
		return nil
	}
	return n.Details.IncidentDetails
}

func alertStringDetails(n *db.AlertNotification) []string {
	var details []string
	if n.Details != nil {
		if n.Details.ProjectName != "" {
			details = append(details, fmt.Sprintf("Project: %s", n.Details.ProjectName))
		}
		if n.Details.RuleName != "" {
			details = append(details, fmt.Sprintf("Alerting rule: %s", n.Details.RuleName))
		}
		for _, d := range n.Details.Details {
			details = append(details, fmt.Sprintf("%s: %s", d.Name, d.Value))
		}
	}
	if incident := alertIncidentDetails(n); incident != nil {
		for _, r := range incident.Reports {
			details = append(details, fmt.Sprintf("%s / %s: %s", r.Name, r.Check, r.Message))
		}
		if incident.RCASummary != "" {
			details = append(details, fmt.Sprintf("Root Cause: %s", incident.RCASummary))
		}
		if incident.RCARemediations != "" {
			details = append(details, fmt.Sprintf("Remediations: %s", incident.RCARemediations))
		}
	}
	return details
}

func alertMapDetails(n *db.AlertNotification) map[string]string {
	details := map[string]string{}
	if n.Details != nil {
		if n.Details.ProjectName != "" {
			details["Project"] = n.Details.ProjectName
		}
		if n.Details.RuleName != "" {
			details["Alerting rule"] = n.Details.RuleName
		}
		for _, d := range n.Details.Details {
			details[d.Name] = d.Value
		}
	}
	if incident := alertIncidentDetails(n); incident != nil {
		for _, r := range incident.Reports {
			details[fmt.Sprintf("%s / %s", r.Name, r.Check)] = r.Message
		}
		if incident.RCASummary != "" {
			details["Root Cause"] = incident.RCASummary
		}
		if incident.RCARemediations != "" {
			details["Remediations"] = incident.RCARemediations
		}
	}
	if len(details) == 0 {
		return nil
	}
	return details
}

func alertMarkdownDetails(n *db.AlertNotification) string {
	if n.Details == nil && alertIncidentDetails(n) == nil {
		return ""
	}
	var lines []string
	if n.Details != nil {
		if n.Details.ProjectName != "" {
			lines = append(lines, fmt.Sprintf("**Project**: %s", n.Details.ProjectName))
		}
		if n.Details.RuleName != "" {
			lines = append(lines, fmt.Sprintf("**Alerting rule**: %s", n.Details.RuleName))
		}
		for _, d := range n.Details.Details {
			if d.Code {
				lines = append(lines, fmt.Sprintf("**%s**:\n```\n%s\n```", d.Name, d.Value))
			} else {
				lines = append(lines, fmt.Sprintf("**%s**: %s", d.Name, d.Value))
			}
		}
	}
	if incident := alertIncidentDetails(n); incident != nil {
		for _, r := range incident.Reports {
			lines = append(lines, fmt.Sprintf("**%s** / %s: %s", r.Name, r.Check, r.Message))
		}
		if incident.RCASummary != "" {
			lines = append(lines, fmt.Sprintf("**Root Cause**: %s", incident.RCASummary))
		}
		if incident.RCARemediations != "" {
			lines = append(lines, fmt.Sprintf("**Remediations**: %s", incident.RCARemediations))
		}
	}
	return strings.Join(lines, "\n\n")
}

func sortedStrings(values []string) []string {
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}
