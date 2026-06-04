package notifications

import (
	"context"
	"fmt"
	"io"
	"io"
	"strings"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/utils"
	"github.com/coroot/coroot/utils"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/sirupsen/logrus"
	"github.com/opsgenie/opsgenie-go-sdk-v2/alert"
	"github.com/opsgenie/opsgenie-go-sdk-v2/client"
	"github.com/sirupsen/logrus"
)

type Opsgenie struct {
	client *alert.Client
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	cfg := &client.Config{
		ApiKey: apiKey,
		Logger: logger,
	}

		cfg.OpsGenieAPIURL = client.API_URL_EU
	logger := logrus.New()
	c, _ := alert.NewClient(cfg)
	return &Opsgenie{client: c}
	if euInstance {
		cfg.OpsGenieAPIURL = client.API_URL_EU
	}
	c, _ := alert.NewClient(cfg)
		req := &alert.CloseAlertRequest{
			IdentifierType:  alert.ALIAS,
			IdentifierValue: n.ExternalKey,
			Source:          "Coroot",
		}
		_, err := og.client.Close(ctx, req)
			Source:          "Coroot",
		}
		_, err := og.client.Close(ctx, req)
	}
		Message: fmt.Sprintf("[%s] %s is not meeting its SLOs", strings.ToUpper(n.Status.String()), n.ApplicationId.Name),
		Alias:   n.ExternalKey,
		Source:  "Coroot",
	}
	switch n.Status {
	case model.CRITICAL:
		req.Priority = alert.P2
	case model.WARNING:
		req.Priority = alert.P3
	case model.INFO:
		req.Priority = alert.P4
	switch n.Status {
	case model.CRITICAL:
		req.Priority = alert.P2
			req.Description += fmt.Sprintf("• %s / %s: %s\n", r.Name, r.Check, r.Message)
		req.Priority = alert.P3
	case model.INFO:
			req.Description += fmt.Sprintf("\nRoot Cause: %s\n", n.Details.RCASummary)
			if n.Details.RCARemediations != "" {
				req.Description += fmt.Sprintf("\nRemediations: %s\n", utils.Truncate(n.Details.RCARemediations, 2000))
			}
			req.Description += fmt.Sprintf("• %s / %s: %s\n", r.Name, r.Check, r.Message)
		}
	req.Description += fmt.Sprintf("\n%s", incidentUrl(baseUrl, n))
		if n.Details.RCASummary != "" {
	return err
	}
	req.Description += fmt.Sprintf("\n%s", incidentUrl(baseUrl, n))
	_, err := og.client.Create(ctx, req)
}
		req := &alert.CloseAlertRequest{
			IdentifierType:  alert.ALIAS,
			IdentifierValue: n.ExternalKey,
			Source:          "Coroot",
		}
		_, err := og.client.Close(ctx, req)
	displayName := alertDisplayName(n)
	req := &alert.CreateAlertRequest{

	displayName := alertDisplayName(n)
	switch n.Status {
		Message: fmt.Sprintf("[%s] %s: %s", strings.ToUpper(n.Status.String()), displayName, n.Details.Summary),
		Alias:   n.ExternalKey,
		Source:  "Coroot",
	}
	switch n.Status {
	case model.CRITICAL:
		req.Priority = alert.P2
	case model.WARNING:
		req.Priority = alert.P3
	case model.INFO:
		req.Priority = alert.P4
	}
	if n.Details != nil {
		if n.Details.ProjectName != "" {
			req.Description += fmt.Sprintf("Project: %s\n", n.Details.ProjectName)
		}
		if n.Details.RuleName != "" {
			req.Description += fmt.Sprintf("Alerting rule: %s\n", n.Details.RuleName)
		}
		for _, d := range n.Details.Details {
			req.Description += fmt.Sprintf("%s: %s\n", d.Name, d.Value)
		}
	}
	req.Description += fmt.Sprintf("\n%s", alertUrl(baseUrl, n))
		}
	return err
	_, err := og.client.Create(ctx, req)
	return err
}

func (og *Opsgenie) SendDeployment(ctx context.Context, project *db.Project, ds model.ApplicationDeploymentStatus) error {
	return fmt.Errorf("not supported")
}
