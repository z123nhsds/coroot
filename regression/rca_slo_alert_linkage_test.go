package regression

import (
	"math"
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestRcaSummarySloAlertLinkage_AfterGoVet(t *testing.T) {
	t.Run("SLOAvailabilityCheckProducesValidSummaryForAlert", func(t *testing.T) {
		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))

		sli := &model.AvailabilitySLI{
			Config: model.CheckConfigSLOAvailability{ObjectivePercentage: 99.9},
		}
		totalData := make([]float32, 120)
		failedData := make([]float32, 120)
		for i := range totalData {
			totalData[i] = 100
			if i > 100 {
				failedData[i] = 50
			}
		}
		sli.TotalRequestsRaw = timeseries.NewWithData(1, timeseries.Minute, totalData)
		sli.FailedRequestsRaw = timeseries.NewWithData(1, timeseries.Minute, failedData)
		sli.TotalRequests = timeseries.NewWithData(1, timeseries.Minute, totalData)
		sli.FailedRequests = timeseries.NewWithData(1, timeseries.Minute, failedData)
		app.AvailabilitySLIs = []*model.AvailabilitySLI{sli}

		world := model.NewWorld(1, 120, timeseries.Minute, timeseries.Minute)
		world.CheckConfigs = model.CheckConfigs{}
		world.Applications[app.Id] = app

		project := &db.Project{Id: "p1", Name: "test"}

		auditor.Audit(world, project, nil, nil)

		var sloReport *model.AuditReport
		for _, r := range app.Reports {
			if r.Name == model.AuditReportSLO {
				sloReport = r
				break
			}
		}
		assert.NotNil(t, sloReport, "SLO audit report must exist after audit")

		var availCheck *model.Check
		for _, ch := range sloReport.Checks {
			if ch.Id == model.Checks.SLOAvailability.Id {
				availCheck = ch
				break
			}
		}
		assert.NotNil(t, availCheck, "SLO availability check must exist")
		assert.NotEmpty(t, availCheck.Message, "availability check must have a summary message for alert linkage")
	})

	t.Run("SLOLatencyCheckProducesValidSummaryForAlert", func(t *testing.T) {
		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))

		sli := &model.LatencySLI{
			Config: model.CheckConfigSLOLatency{
				ObjectiveBucket:     0.1,
				ObjectivePercentage: 99,
			},
		}
		totalData := make([]float32, 120)
		fastData := make([]float32, 120)
		for i := range totalData {
			totalData[i] = 100
			fastData[i] = 50
		}
		sli.HistogramRaw = []model.HistogramBucket{
			{Le: 0.1, TimeSeries: timeseries.NewWithData(1, timeseries.Minute, fastData)},
			{Le: float32(math.Inf(1)), TimeSeries: timeseries.NewWithData(1, timeseries.Minute, totalData)},
		}
		sli.Histogram = sli.HistogramRaw
		app.LatencySLIs = []*model.LatencySLI{sli}

		world := model.NewWorld(1, 120, timeseries.Minute, timeseries.Minute)
		world.CheckConfigs = model.CheckConfigs{}
		world.Applications[app.Id] = app

		project := &db.Project{Id: "p1", Name: "test"}

		auditor.Audit(world, project, nil, nil)

		var sloReport *model.AuditReport
		for _, r := range app.Reports {
			if r.Name == model.AuditReportSLO {
				sloReport = r
				break
			}
		}
		assert.NotNil(t, sloReport, "SLO audit report must exist after audit")

		var latencyCheck *model.Check
		for _, ch := range sloReport.Checks {
			if ch.Id == model.Checks.SLOLatency.Id {
				latencyCheck = ch
				break
			}
		}
		assert.NotNil(t, latencyCheck, "SLO latency check must exist")
		assert.NotEmpty(t, latencyCheck.Message, "latency check must have a summary message for alert linkage")
	})

	t.Run("SLOCheckStatusFlowsToAlertingRuleMatch", func(t *testing.T) {
		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))
		app.Category = model.ApplicationCategoryApplication

		rule := &model.AlertingRule{
			Id:       model.AlertingRuleId("slo-availability"),
			Name:     "SLO Availability",
			Source:   model.AlertSource{Type: model.AlertSourceTypeCheck, Check: &model.CheckSource{CheckId: model.Checks.SLOAvailability.Id}},
			Selector: model.AppSelector{Type: model.AppSelectorTypeAll},
			Severity: model.WARNING,
			Enabled:  true,
		}

		assert.True(t, rule.Matches(app), "alerting rule must match application for SLO check alert")
		assert.Equal(t, model.AlertSourceTypeCheck, rule.Source.Type, "alert source must be check type for SLO linkage")
		assert.Equal(t, model.Checks.SLOAvailability.Id, rule.Source.Check.CheckId, "check ID must correspond to SLO availability")
	})

	t.Run("AlertDetailPromQLExcludedFromNotification", func(t *testing.T) {
		details := []model.AlertDetail{
			{Name: "Description", Value: "SLO burn rate exceeded"},
			{Name: "PromQL", Value: "rate(http_requests_total[5m])"},
			{Name: "PromQLChart", Value: "rate(http_requests_total[5m])"},
		}

		filtered := filterAlertDetails(details)
		assert.Len(t, filtered, 1, "PromQL and PromQLChart details must be filtered from notifications")
		assert.Equal(t, "Description", filtered[0].Name)
	})

	t.Run("BurnRateFormatSLOStatusConsistent", func(t *testing.T) {
		br := model.BurnRate{
			LongWindow:         timeseries.Hour,
			ShortWindow:        5 * timeseries.Minute,
			LongWindowBurnRate: 14.4,
			Severity:           model.CRITICAL,
		}

		status := br.FormatSLOStatus()
		assert.Contains(t, status, "14.4x", "burn rate must appear in SLO status for RCA summary")
		assert.Contains(t, status, "1 hour", "window must appear in SLO status for RCA summary")
	})

	t.Run("SLOCheckIdConsistentWithAlertingRuleSource", func(t *testing.T) {
		assert.Equal(t, model.CheckId("SLOAvailability"), model.Checks.SLOAvailability.Id,
			"SLO availability check ID must be consistent for alerting rule source mapping")
		assert.Equal(t, model.CheckId("SLOLatency"), model.Checks.SLOLatency.Id,
			"SLO latency check ID must be consistent for alerting rule source mapping")
	})
}

func filterAlertDetails(details []model.AlertDetail) []model.AlertDetail {
	var filtered []model.AlertDetail
	for _, d := range details {
		if d.Name == "PromQL" || d.Name == "PromQLChart" {
			continue
		}
		filtered = append(filtered, d)
	}
	return filtered
}
