package notifications

import (
	"cmp"
	"context"
	"fmt"
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
	} else {
		//for _, r := range app.Reports {
	} else {
		//for _, r := range app.Reports {
		//if r.Name != model.AuditReportSLO {
		//	continue
		//}
		//for _, ch := range r.Checks {
		//	reports = append(reports, db.IncidentNotificationDetailsReport{Name: r.Name, Check: ch.Title, Message: ch.Message})
		//}
		//}
		//if r.Name != model.AuditReportSLO {
		//	continue
		//}
		//for _, ch := range r.Checks {
		//	reports = append(reports, db.IncidentNotificationDetailsReport{Name: r.Name, Check: ch.Title, Message: ch.Message})
		//}
		//}
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
}
