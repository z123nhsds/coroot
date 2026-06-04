package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
)

type Pagerduty struct {
	client     *pagerduty.V2EventsAPIClient
	routingKey string
}

func NewPagerduty(integrationKey string) *Pagerduty {
	return &Pagerduty{client: pagerduty.NewV2EventsAPIClient(integrationKey), routingKey: integrationKey}
}

func (pd *Pagerduty) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	action := "trigger"
	body := &pagerduty.V2Event{
		RoutingKey: pd.routingKey,
		Action:     action,
		DedupKey:   n.IncidentKey,
		Payload: &pagerduty.V2Payload{
			Summary:   n.ApplicationId.Name + " is not meeting its SLOs",
			Source:    n.ApplicationId.Name,
			Severity:  strings.ToLower(n.Status.String()),
			Timestamp: pagerduty.APIIncidentTimestamp(n.Timestamp.ToStandard()),
			Class:     n.ApplicationId.String(),
			Group:     n.ProjectId.String(),
			Component: n.ApplicationId.Name,
		},
		Links: []pagerduty.Link{{Href: incidentUrl(baseUrl, n), Text: n.ApplicationId.Name}},
	}
	if n.Details != nil {
		for _, r := range n.Details.Reports {
			body.Payload.CustomDetails = append(body.Payload.CustomDetails, map[string]any{r.Name + " / " + r.Check: r.Message})
		}
		if n.Details.RCASummary != "" {
			body.Payload.CustomDetails = append(body.Payload.CustomDetails, map[string]any{"Root Cause": n.Details.RCASummary})
		}
		if n.Details.RCARemediations != "" {
			body.Payload.CustomDetails = append(body.Payload.CustomDetails, map[string]any{"Remediations": n.Details.RCARemediations})
		}
	}
	if n.Status == model.OK {
		action = "resolve"
		body.Payload.Severity = "info"
		body.Payload.Summary = n.ApplicationId.Name + " incident resolved"
	}
	if action == "resolve" && body.DedupKey == "" {
		return nil
	}
	res, err := pd.client.ManageEventWithContext(ctx, body)
	if err != nil {
		return err
	}
	n.ExternalKey = res.DedupKey
	return nil
}

func (pd *Pagerduty) SendAlert(ctx context.Context, baseUrl string, n *db.AlertNotification) error {
	action := "trigger"
	displayName := alertDisplayName(n)
	summary := displayName + " alert fired"
	severity := strings.ToLower(n.Status.String())
	if n.Details != nil && n.Details.Summary != "" {
		summary = n.Details.Summary
	}
	body := &pagerduty.V2Event{
		RoutingKey: pd.routingKey,
		Action:     action,
		DedupKey:   n.ExternalKey,
		Payload: &pagerduty.V2Payload{
			Summary:   summary,
			Source:    displayName,
			Severity:  severity,
			Timestamp: pagerduty.APIIncidentTimestamp(n.Timestamp.ToStandard()),
			Class:     n.ApplicationId.String(),
			Group:     n.ProjectId.String(),
			Component: displayName,
		},
		Links: []pagerduty.Link{{Href: alertNotificationUrl(baseUrl, n), Text: displayName}},
	}
	for k, v := range alertMapDetails(n) {
		body.Payload.CustomDetails = append(body.Payload.CustomDetails, map[string]any{k: v})
	}
	if n.Status == model.OK {
		action = "resolve"
		body.Action = action
		body.Payload.Severity = "info"
		resolvedText := "resolved"
		if n.Details != nil && n.Details.ResolvedBy != "" {
			resolvedText = fmt.Sprintf("manually resolved by %s", n.Details.ResolvedBy)
		}
		if n.Details != nil && n.Details.Duration != "" {
			body.Payload.Summary = fmt.Sprintf("%s alert %s (duration: %s)", displayName, resolvedText, n.Details.Duration)
		} else {
			body.Payload.Summary = fmt.Sprintf("%s alert %s", displayName, resolvedText)
		}
	}
	if action == "resolve" && body.DedupKey == "" {
		return nil
	}
	res, err := pd.client.ManageEventWithContext(ctx, body)
	if err != nil {
		return err
	}
	n.ExternalKey = res.DedupKey
	return nil
}

func (pd *Pagerduty) SendDeployment(ctx context.Context, _ *db.Project, ds model.ApplicationDeploymentStatus) error {
	return nil
}
