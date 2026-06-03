package watchers

import (
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/prom"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestRCAIncidentSLOAlertLinkage(t *testing.T) {
	incident := &model.ApplicationIncident{
		ApplicationId: model.NewApplicationId("cluster-1", "default", model.ApplicationKindDeployment, "api-gateway"),
		Key:           "abc12345",
		OpenedAt:      timeseries.Now(),
		Severity:      model.WARNING,
		Details: model.IncidentDetails{
			AvailabilityBurnRates: []model.BurnRate{
				{Severity: model.WARNING, Threshold: 14.4, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
			},
			LatencyBurnRates: []model.BurnRate{
				{Severity: model.CRITICAL, Threshold: 6, LongWindow: 6 * timeseries.Hour, ShortWindow: 15 * timeseries.Minute},
			},
			AvailabilityImpact: model.Impact{AffectedRequestPercentage: 0},
			LatencyImpact:      model.Impact{AffectedRequestPercentage: 5.2},
		},
	}
	assert.Equal(t, "High latency", incident.ShortDescription())

	incident.Details.AvailabilityImpact.AffectedRequestPercentage = 3.1
	incident.Details.LatencyImpact.AffectedRequestPercentage = 4.0
	assert.Equal(t, "High latency and errors", incident.ShortDescription())

	incident.Details.LatencyImpact.AffectedRequestPercentage = 0
	incident.Details.AvailabilityImpact.AffectedRequestPercentage = 2.5
	assert.Equal(t, "Elevated error rate", incident.ShortDescription())

	incident.Details.AvailabilityImpact.AffectedRequestPercentage = 0
	incident.Details.LatencyImpact.AffectedRequestPercentage = 0
	assert.Equal(t, "SLO violation", incident.ShortDescription())

	incident.RCA = &model.RCA{ShortSummary: "CPU throttling caused by node resource pressure"}
	assert.Equal(t, "CPU throttling caused by node resource pressure", incident.ShortDescription())
}

func TestRCAIncidentResolvedState(t *testing.T) {
	incident := &model.ApplicationIncident{
		ApplicationId: model.NewApplicationId("cluster-1", "default", model.ApplicationKindDeployment, "api-gateway"),
		Key:           "abc12345",
		OpenedAt:      timeseries.Now(),
		Severity:      model.OK,
		ResolvedAt:    timeseries.Now(),
		Details: model.IncidentDetails{
			AvailabilityBurnRates: []model.BurnRate{
				{Severity: model.OK, Threshold: 14.4, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
			},
			LatencyBurnRates: []model.BurnRate{
				{Severity: model.OK, Threshold: 6, LongWindow: 6 * timeseries.Hour, ShortWindow: 15 * timeseries.Minute},
			},
		},
	}
	assert.True(t, incident.Resolved())
	assert.Equal(t, "SLO violation", incident.ShortDescription())
}

func TestRCAOpenResolvedAtZero(t *testing.T) {
	incident := &model.ApplicationIncident{
		ApplicationId: model.NewApplicationId("cluster-1", "default", model.ApplicationKindDeployment, "api-gateway"),
		Key:           "inc-open",
		OpenedAt:      timeseries.Now(),
		Severity:      model.WARNING,
		ResolvedAt:    0,
	}
	assert.False(t, incident.Resolved())
}

func TestPromExtraSelectorSLOQueries(t *testing.T) {
	check := func(src, extraSelector, expected string) {
		actual, err := prom.AddExtraSelector(src, extraSelector)
		assert.NoError(t, err)
		assert.Equal(t, expected, actual)
	}

	check(
		`sum(rate(http_requests_total{status=~"5.."}[5m])) / sum(rate(http_requests_total[5m]))`,
		`{cluster="prod-us-east"}`,
		`sum(rate(http_requests_total{cluster="prod-us-east",status=~"5.."}[5m])) / sum(rate(http_requests_total{cluster="prod-us-east"}[5m]))`)

	check(
		`histogram_quantile(0.99, sum(rate(request_latency_bucket[5m])) by (le))`,
		`{cluster="prod-us-east", namespace="default"}`,
		`histogram_quantile(0.99, sum(rate(request_latency_bucket{cluster="prod-us-east",namespace="default"}[5m])) by (le))`)

	check(
		`rate(http_requests_total{status="200"}[5m])`,
		`{cluster="staging"}`,
		`rate(http_requests_total{cluster="staging",status="200"}[5m])`)
}

func TestRCAPropagationMap(t *testing.T) {
	app := &model.PropagationMapApplication{
		Id:     model.NewApplicationId("c1", "ns", model.ApplicationKindDeployment, "svc"),
		Status: model.OK,
	}
	assert.Equal(t, model.OK, app.Status)
	app.Issue("memory leak detected")
	assert.Equal(t, 1, len(app.Issues))
	app.Issue("memory leak detected")
	assert.Equal(t, 1, len(app.Issues))
	app.Issue("high latency")
	assert.Equal(t, 2, len(app.Issues))

	link := &model.PropagationMapApplicationLink{
		Id:     model.NewApplicationId("c1", "ns", model.ApplicationKindDeployment, "db"),
		Status: model.OK,
	}
	link.AddIssues("connection timeout", "slow query")
	assert.Equal(t, model.CRITICAL, link.Status)
	assert.Equal(t, 2, link.Stats.Len())
}

func TestBurnRateSeverityOrdering(t *testing.T) {
	burnRates := []model.BurnRate{
		{Severity: model.OK, LongWindowBurnRate: 0.5, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
		{Severity: model.WARNING, LongWindowBurnRate: 1.5, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
		{Severity: model.CRITICAL, LongWindowBurnRate: 5.0, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
	}

	var maxSeverity model.Status = model.UNKNOWN
	for _, br := range burnRates {
		if br.Severity > maxSeverity {
			maxSeverity = br.Severity
		}
	}
	assert.Equal(t, model.CRITICAL, maxSeverity)

	allOK := []model.BurnRate{
		{Severity: model.OK, LongWindowBurnRate: 0.1, LongWindow: timeseries.Hour, ShortWindow: 5 * timeseries.Minute},
		{Severity: model.OK, LongWindowBurnRate: 0.2, LongWindow: 6 * timeseries.Hour, ShortWindow: 15 * timeseries.Minute},
	}
	var maxOK model.Status = model.UNKNOWN
	for _, br := range allOK {
		if br.Severity > maxOK {
			maxOK = br.Severity
		}
	}
	assert.Equal(t, model.OK, maxOK)
}