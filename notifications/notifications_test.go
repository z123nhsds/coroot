package notifications

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIncidentDetailsAndWebhookPayloadIncludeRCASummaryForSLOIncident(t *testing.T) {
	appId := model.NewApplicationId("cluster-a", "default", model.ApplicationKindDeployment, "checkout")
	app := model.NewApplication(appId)
	app.Reports = []*model.AuditReport{
		{
			Name: model.AuditReportSLO,
			Checks: []*model.Check{{
				Title:   "Availability",
				Status:  model.CRITICAL,
				Message: "error budget burn rate is 26x within 1 hour",
			}},
		},
		{
			Name: model.AuditReportNetwork,
			Checks: []*model.Check{{
				Title:   "Network round-trip time (RTT)",
				Status:  model.WARNING,
				Message: "high network latency to 2 upstream services",
			}},
		},
	}
	incident := &model.ApplicationIncident{
		Key:      "incident-1",
		Severity: model.WARNING,
		RCA: &model.RCA{
			Status:         "OK",
			ShortSummary:   "Dropped index on postgres-products caused CPU saturation on node2",
			ImmediateFixes: "Recreate the dropped index and roll back the offending deployment",
		},
	}

	details := incidentDetails(app, incident)
	require.NotNil(t, details)
	require.Len(t, details.Reports, 1)
	assert.Equal(t, model.AuditReportNetwork, details.Reports[0].Name)
	assert.Equal(t, "Network round-trip time (RTT)", details.Reports[0].Check)
	assert.Equal(t, "high network latency to 2 upstream services", details.Reports[0].Message)
	assert.Equal(t, incident.RCA.ShortSummary, details.RCASummary)
	assert.Equal(t, incident.RCA.ImmediateFixes, details.RCARemediations)

	var payload []byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		payload, err = io.ReadAll(r.Body)
		require.NoError(t, err)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	wh := NewWebhook(&db.IntegrationWebhook{
		Url:              server.URL,
		IncidentTemplate: `{{json .}}`,
	})
	n := &db.IncidentNotification{
		ProjectId:     db.ProjectId("project-1"),
		ApplicationId: appId,
		IncidentKey:   incident.Key,
		Status:        model.WARNING,
		Details:       details,
	}
	require.NoError(t, wh.SendIncident(context.Background(), "https://coroot.example", n))

	var body IncidentTemplateValues
	require.NoError(t, json.Unmarshal(payload, &body))
	assert.Equal(t, "WARNING", body.Status)
	assert.Equal(t, appId, body.Application)
	assert.Equal(t, details.Reports, body.Reports)
	assert.Equal(t, details.RCASummary, body.RCASummary)
	assert.Equal(t, details.RCARemediations, body.RCARemediations)
	assert.Equal(t, "https://coroot.example/p/project-1/incidents?incident=incident-1", body.URL)
}
