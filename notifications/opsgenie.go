package notifications

import (
	"context"
	"fmt"
	"strings"

	opsgenieSDK "github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Opsgenie struct {
	client *alert.Client
}

func NewOpsgenie(apiKey string, euInstance bool) *Opsgenie {
	cfg := &opsgenieSDK.Config{ApiKey: apiKey}
	if euInstance {
		cfg.APIUrl = alert.EUAlertAPIURL
	}
	client, err := alert.NewClient(cfg)
	if err != nil {
		panic(err)
	}
	return &Opsgenie{client: client}
}

func (og *Opsgenie) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	if n.Status == model.OK {
		if n.ExternalKey == "" {
			return nil
		}
		_, err := og.client.Close(ctx, &alert.CloseAlertRequest{
			IdentifierType: alert.AliasIdentifier,
			Identifier:     n.ExternalKey,
			Note:           "incident resolved",
		})
		return err
	}

	msg := n.ApplicationId.Name + " is not meeting its SLOs"
	req := &alert.CreateAlertRequest{
		Alias:       n.IncidentKey,
		Message:     msg,
		Description: msg,
		Details:     map[string]string{"Application": n.ApplicationId.String(), "Incident": incidentUrl(baseUrl, n)},
		Source:      n.ProjectId.String(),
		Priority:    toOpsgeniePriority(n.Status),
	}
	if n.Details != nil {
		for _, r := range n.Details.Reports {
			req.Details[r.Name+" / "+r.Check] = r.Message
		}
		if n.Details.RCASummary != "" {
			req.Details["Root Cause"] = n.Details.RCASummary
		}
		if n.Details.RCARemediations != "" {
			req.Details["Remediations"] = n.Details.RCARemediations
		}
	}
	_, err := og.client.Create(ctx, req)
	if err != nil {
		return err
	}
	n.ExternalKey = req.Alias
	return nil
}

func (og *Opsgenie) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	displayName := alertDisplayName(n)
	if n.Status == model.OK {
		if n.ExternalKey == "" {
			return nil
		}
		message := displayName + " alert resolved"
		if n.Details != nil && n.Details.ResolvedBy != "" {
			message = fmt.Sprintf("%s alert manually resolved by %s", displayName, n.Details.ResolvedBy)
		}
		_, err := og.client.Close(ctx, &alert.CloseAlertRequest{
			IdentifierType: alert.AliasIdentifier,
			Identifier:     n.ExternalKey,
			Note:           message,
		})
		return err
	}
	summary := displayName + " alert fired"
	if n.Details != nil && n.Details.Summary != "" {
		summary = n.Details.Summary
	}
	req := &alert.CreateAlertRequest{
		Alias:       n.AlertId,
		Message:     summary,
		Description: summary,
		Details: map[string]string{
			"Application": n.ApplicationId.String(),
			"Alert":       alertNotificationUrl(baseUrl, n),
		},
		Source:   n.ProjectId.String(),
		Priority: toOpsgeniePriority(n.Status),
	}
	for k, v := range alertMapDetails(n) {
		req.Details[k] = v
	}
	_, err := og.client.Create(ctx, req)
	if err != nil {
		return err
	}
	n.ExternalKey = req.Alias
	return nil
}

func (og *Opsgenie) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	return nil
}

func toOpsgeniePriority(status model.Status) string {
	switch strings.ToUpper(status.String()) {
	case strings.ToUpper(model.CRITICAL.String()):
		return alert.P1.String()
	case strings.ToUpper(model.WARNING.String()):
		return alert.P3.String()
	default:
		return alert.P5.String()
	}
}
