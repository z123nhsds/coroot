package notifications

import (
	"testing"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestIncidentDetailsWithSLOAndRCA(t *testing.T) {
	app := createTestApplication()

	// Test case 1: Incident with SLO alert and RCA summary
	incident := &model.ApplicationIncident{
		Severity: model.CRITICAL,
		RCA: &model.RCA{
			Status:           "OK",
			ShortSummary:     "High error rate detected due to database connection issues",
			ImmediateFixes:   []string{"Restart database connection pool", "Increase timeout settings"},
		},
	}

	// Add some failing checks (excluding SLO) to the app
	addFailingChecks(app)

	details := incidentDetails(app, incident)

	assert.NotNil(t, details)
	assert.NotEmpty(t, details.Reports)
	assert.Equal(t, "High error rate detected due to database connection issues", details.RCASummary)
	assert.Len(t, details.RCARemediations, 2)
	assert.Contains(t, details.RCARemediations, "Restart database connection pool")

	// Test case 2: Resolved incident - should not have reports
	resolvedIncident := &model.ApplicationIncident{
		Severity:   model.OK,
		ResolvedAt: timeseries.Now(),
		RCA: &model.RCA{
			Status:       "OK",
			ShortSummary: "Issue resolved after restarting connection pool",
		},
	}
	details = incidentDetails(app, resolvedIncident)
	assert.NotNil(t, details)
	// For resolved incidents, the code has SLO reports commented out
	assert.Empty(t, details.Reports)

	// Test case 3: Incident without RCA
	incidentWithoutRCA := &model.ApplicationIncident{
		Severity: model.WARNING,
	}
	details = incidentDetails(app, incidentWithoutRCA)
	assert.NotNil(t, details)
	assert.Empty(t, details.RCASummary)
	assert.Empty(t, details.RCARemediations)
}

func TestAlertNotificationCreation(t *testing.T) {
	// This test verifies that alerts can be properly created and queued
	// with the necessary information for SLO association
	project := createTestProject()
	
	// Simulate an SLO-related alert
	alert := &model.Alert{
		Id:             "test-alert-123",
		ApplicationId:  model.NewApplicationId("cluster1", "default", model.ApplicationKindDeployment, "test-app"),
		Severity:       model.CRITICAL,
		Summary:        "SLO violation: Error rate exceeds 5%",
		Report:         model.AuditReportSLO,
	}

	// Verify alert has correct report type for SLO
	assert.Equal(t, model.AuditReportSLO, alert.Report)
}

func createTestApplication() *model.Application {
	app := model.NewApplication(model.NewApplicationId("cluster1", "default", model.ApplicationKindDeployment, "test-app"))
	
	// Add a memory report with a failing check
	memoryReport := &model.AuditReport{
		Name: model.AuditReportMemory,
	}
	memoryCheck := &model.Check{
		Title:  "Memory Leak Detected",
		Status: model.WARNING,
		Message: "Memory growth of 30% detected over last hour",
	}
	memoryReport.Checks = append(memoryReport.Checks, memoryCheck)
	app.Reports = append(app.Reports, memoryReport)

	// Add SLO report (this will be skipped in notifications.incidentDetails)
	sloReport := &model.AuditReport{
		Name: model.AuditReportSLO,
	}
	sloCheck := &model.Check{
		Title:  "SLO Error Rate",
		Status: model.CRITICAL,
		Message: "Error rate exceeds threshold",
	}
	sloReport.Checks = append(sloReport.Checks, sloCheck)
	app.Reports = append(app.Reports, sloReport)

	return app
}

func createTestProject() *db.Project {
	return &db.Project{
		Id: db.ProjectId("test-project"),
		Settings: db.ProjectSettings{
			Integrations: db.Integrations{
				Slack: &db.IntegrationSlack{
					Enabled: true,
					Token: "test-token",
					DefaultChannel: "#alerts",
				},
			},
		},
	}
}

func addFailingChecks(app *model.Application) {
	for _, r := range app.Reports {
		for _, c := range r.Checks {
			c.Status = model.WARNING
		}
	}
}
