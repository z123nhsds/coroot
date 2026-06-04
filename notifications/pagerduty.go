package notifications

import (
	"context"
	"fmt"
	"strings"

	"github.com/PagerDuty/go-pagerduty"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/utils"
)

	integrationKey string
}

func NewPagerduty(integrationKey string) *Pagerduty {
	return &Pagerduty{integrationKey: integrationKey}
}

func (pd *Pagerduty) SendIncident(ctx context.Context, baseUrl string, n *db.IncidentNotification) error {
	e := pagerduty.V2Event{
		RoutingKey: pd.integrationKey,
		DedupKey:   n.ExternalKey,
	}
	if n.Status == model.OK {
		e.Action = "resolve"
	} else {
		e.Action = "trigger"
		e.Client = "Coroot"
		e.ClientURL = incidentUrl(baseUrl, n)
		e.Payload = &pagerduty.V2Payload{
			Summary:   fmt.Sprintf("[%s] %s is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name),
			Source:    "Coroot",
			Severity:  n.Status.String(),
			Timestamp: n.Timestamp.ToStandard().String(),
		}
		if n.Details != nil {
			details := map[string]string{}
			for _, r := range n.Details.Reports {
				details[fmt.Sprintf("%s / %s", r.Name, r.Check)] = r.Message
			}
			if n.Details.RCASummary != "" {
				details["Root Cause"] = n.Details.RCASummary
			}
			if n.Details.RCARemediations != "" {
				details["Remediations"] = utils.Truncate(n.Details.RCARemediations, 2000)
			}
			if len(details) > 0 {
				e.Payload.Details = details
			}
		}
}
	_, err := pagerduty.ManageEventWithContext(ctx, e)
	return err
	if n.Status == model.OK {
		e.Action = "resolve"
	} else {
	e := pagerduty.V2Event{
		RoutingKey: pd.integrationKey,
		if n.Details != nil {
		}
	}
		e.Action = "resolve"
	} else {
		e.Action = "trigger"
		e.Client = "Coroot"
		e.ClientURL = alertUrl(baseUrl, n)
		displayName := alertDisplayName(n)
		e.Payload = &pagerduty.V2Payload{
			Summary:   fmt.Sprintf("[%s] %s: %s", strings.ToUpper(n.Status.String()), displayName, n.Details.Summary),
			Source:    "Coroot",
			Severity:  n.Status.String(),
			Timestamp: n.Timestamp.ToStandard().String(),
}
