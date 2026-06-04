package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

const isolationAlertRuleId = "builtin:auto-isolation"
const isolationAlertRuleName = "自动隔离操作"

func ExecuteIsolation(ctx context.Context, project *db.Project, record *db.ServiceIsolationRecord) error {
	webhook := project.Settings.Integrations.Webhook
	if webhook == nil || !webhook.Isolations {
		return fmt.Errorf("service mesh isolation webhook is not configured")
	}
	return NewWebhook(webhook).SendIsolation(ctx, project, record)
}

func BuildIsolationRecordDetails(app *model.Application) *db.ServiceIsolationDetails {
	details := &db.ServiceIsolationDetails{Services: isolationServices(app)}
	if incident := latestIsolationIncident(app); incident != nil {
		details.RCA = incidentDetails(app, incident)
	}
	if len(details.Services) > 0 {
		details.Trigger = fmt.Sprintf("内存增长超过前版本 50%%，并持续 3 个采样周期；隔离服务：%s", strings.Join(details.Services, ", "))
	} else {
		details.Trigger = "内存增长超过前版本 50%，并持续 3 个采样周期"
	}
	if details.RCA == nil && details.Trigger == "" && len(details.Services) == 0 {
		return nil
	}
	return details
}

func EnqueueIsolationAlert(database *db.DB, project *db.Project, app *model.Application, record *db.ServiceIsolationRecord, now timeseries.Time) {
	categorySettings := project.GetApplicationCategories()[app.Category]
	if categorySettings == nil {
		return
	}
	settings := categorySettings.NotificationSettings.Alerts
	if !settings.Enabled {
		return
	}

	notification := db.AlertNotification{
		ProjectId:     project.Id,
		AlertId:       fmt.Sprintf("auto-isolation:%s:%s", app.Id.String(), record.DeploymentId),
		RuleId:        isolationAlertRuleId,
		ApplicationId: app.Id,
		Status:        model.CRITICAL,
		Timestamp:     now,
		Details:       isolationAlertDetails(project, app, record),
	}
	if slack := settings.Slack; slack != nil && slack.Enabled {
		n := notification
		n.Destination = db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeSlack, SlackChannel: slack.Channel}
		database.PutAlertNotification(n)
	}
	if teams := settings.Teams; teams != nil && teams.Enabled {
		n := notification
		n.Destination = db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeTeams}
		database.PutAlertNotification(n)
	}
	if pagerduty := settings.Pagerduty; pagerduty != nil && pagerduty.Enabled {
		n := notification
		n.Destination = db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypePagerduty}
		database.PutAlertNotification(n)
	}
	if opsgenie := settings.Opsgenie; opsgenie != nil && opsgenie.Enabled {
		n := notification
		n.Destination = db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeOpsgenie}
		database.PutAlertNotification(n)
	}
	if webhook := settings.Webhook; webhook != nil && webhook.Enabled {
		n := notification
		n.Destination = db.IncidentNotificationDestination{IntegrationType: db.IntegrationTypeWebhook}
		database.PutAlertNotification(n)
	}
}

func latestIsolationIncident(app *model.Application) *model.ApplicationIncident {
	for i := len(app.Incidents) - 1; i >= 0; i-- {
		incident := app.Incidents[i]
		if incident == nil || incident.Resolved() {
			continue
		}
		if incident.RCA != nil || len(app.Reports) > 0 {
			return incident
		}
	}
	return nil
}

func isolationServices(app *model.Application) []string {
	seen := map[string]bool{}
	var services []string
	for _, svc := range app.KubernetesServices {
		if svc == nil {
			continue
		}
		name := svc.Name
		if svc.Namespace != "" {
			name = svc.Namespace + "/" + name
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		services = append(services, name)
	}
	if len(services) == 0 {
		for _, name := range app.LogServices() {
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			services = append(services, name)
		}
	}
	return sortedStrings(services)
}

func isolationAlertDetails(project *db.Project, app *model.Application, record *db.ServiceIsolationRecord) *db.AlertNotificationDetails {
	var incident *db.IncidentNotificationDetails
	var services string
	var trigger string
	if record.Details != nil {
		incident = record.Details.RCA
		services = strings.Join(record.Details.Services, ", ")
		trigger = record.Details.Trigger
	}
	var details []model.AlertDetail
	if services != "" {
		details = append(details, model.AlertDetail{Name: "隔离服务", Value: services})
	}
	details = append(details,
		model.AlertDetail{Name: "当前版本", Value: record.CurrentVersion},
		model.AlertDetail{Name: "前一版本", Value: record.PreviousVersion},
		model.AlertDetail{Name: "当前内存增长", Value: fmt.Sprintf("%.1f%%/h", record.CurrentMemoryGrowthPct)},
		model.AlertDetail{Name: "前版本内存增长", Value: fmt.Sprintf("%.1f%%/h", record.PreviousMemoryGrowthPct)},
		model.AlertDetail{Name: "隔离阈值", Value: fmt.Sprintf("%.1f%%/h", record.ThresholdMemoryGrowthPct)},
		model.AlertDetail{Name: "连续采样周期", Value: fmt.Sprintf("%d", record.ConsecutiveSamples)},
		model.AlertDetail{Name: "处理动作", Value: "已从服务网格路由摘除"},
	)
	if trigger != "" {
		details = append(details, model.AlertDetail{Name: "触发条件", Value: trigger})
	}
	return &db.AlertNotificationDetails{
		ProjectName:     project.Name,
		RuleName:        isolationAlertRuleName,
		Severity:        model.CRITICAL.String(),
		Summary:         fmt.Sprintf("%s 内存泄漏持续扩大，已自动隔离", app.Id.Name),
		Details:         details,
		URL:             applicationUrl(project.Settings.Integrations.BaseUrl, project.Id, app.Id),
		IncidentDetails: incident,
	}
}
