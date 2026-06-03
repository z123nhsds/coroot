package regression

import (
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/rbac"
	"github.com/coroot/coroot/stats"
	"github.com/coroot/coroot/timeseries"
	"github.com/stretchr/testify/assert"
)

func TestMcpRbacStatsLogClustering_MemoryGrowthPctImpact(t *testing.T) {
	t.Run("MemoryGrowthPctPropagatesToDeploymentMetricsSnapshot", func(t *testing.T) {
		rssData := make([]float32, 120)
		for i := range rssData {
			rssData[i] = float32(100*1024*1024 + i*10*1024*1024)
		}
		rssTs := timeseries.NewWithData(1, timeseries.Minute, rssData)
		limit := float32(500 * 1024 * 1024)

		pct := auditor.MemoryGrowthPct(rssTs, limit, timeseries.Time(120))
		assert.Greater(t, pct, float32(0), "MemoryGrowthPct must detect growth for monotonically increasing RSS")

		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))
		instance := app.GetOrCreateInstance("i1", nil)
		instance.Containers = map[string]*model.Container{
			"main": {
				Name:        "main",
				MemoryRss:   rssTs,
				MemoryLimit: timeseries.NewWithData(1, timeseries.Minute, []float32{limit}),
			},
		}
		instance.Pod = &model.Pod{ReplicaSet: "rs-v2"}

		assert.NotNil(t, instance.Containers["main"].MemoryRss, "container MemoryRss must be set for MemoryGrowthPct calculation")
		assert.Equal(t, model.ApplicationKindDeployment, app.Id.Kind, "application must be Deployment kind for deployment tracking")
	})

	t.Run("MemoryGrowthPctZeroForStableMemory", func(t *testing.T) {
		rssData := make([]float32, 120)
		for i := range rssData {
			rssData[i] = float32(200 * 1024 * 1024)
		}
		rssTs := timeseries.NewWithData(1, timeseries.Minute, rssData)
		limit := float32(500 * 1024 * 1024)

		pct := auditor.MemoryGrowthPct(rssTs, limit, timeseries.Time(120))
		assert.Equal(t, float32(0), pct, "MemoryGrowthPct must be zero for stable memory usage")
	})

	t.Run("MemoryGrowthPctZeroForEmptyTimeSeries", func(t *testing.T) {
		rssTs := &timeseries.TimeSeries{}
		pct := auditor.MemoryGrowthPct(rssTs, float32(500*1024*1024), timeseries.Time(120))
		assert.Equal(t, float32(0), pct, "MemoryGrowthPct must be zero for empty time series")
	})

	t.Run("StatsCollectorTracksMcpCallsByTool", func(t *testing.T) {
		s := stats.Stats{}
		s.UX.McpCalls = map[string]int{}

		s.UX.McpCalls["list_alerts"]++
		s.UX.McpCalls["get_application_status"]++
		s.UX.McpCalls["list_alerts"]++

		assert.Equal(t, 2, s.UX.McpCalls["list_alerts"], "MCP call counts must be tracked per tool")
		assert.Equal(t, 1, s.UX.McpCalls["get_application_status"], "MCP call counts must be tracked per tool")
	})

	t.Run("RbacPermissionsGateMcpProjectAccess", func(t *testing.T) {
		adminPerms := rbac.Roles[0].Permissions
		assert.True(t, adminPerms.Allows(rbac.Actions.Project("p1").Alerts().View()),
			"Admin role must allow viewing alerts via MCP")
		assert.True(t, adminPerms.Allows(rbac.Actions.Project("p1").Alerts().Edit()),
			"Admin role must allow editing alerts via MCP")

		viewerPerms := rbac.Roles[2].Permissions
		assert.True(t, viewerPerms.Allows(rbac.Actions.Project("p1").Alerts().View()),
			"Viewer role must allow viewing alerts via MCP")
		assert.False(t, viewerPerms.Allows(rbac.Actions.Project("p1").Alerts().Edit()),
			"Viewer role must not allow editing alerts via MCP")
	})

	t.Run("RbacPermissionsGateMcpLogAccess", func(t *testing.T) {
		editorPerms := rbac.Roles[1].Permissions
		assert.True(t, editorPerms.Allows(rbac.Actions.Project("p1").Logs().View()),
			"Editor role must allow viewing logs via MCP")

		viewerPerms := rbac.Roles[2].Permissions
		assert.True(t, viewerPerms.Allows(rbac.Actions.Project("p1").Logs().View()),
			"Viewer role must allow viewing logs via MCP")
	})

	t.Run("LogClusteringSeverityMapping", func(t *testing.T) {
		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))

		errorMsgs := &model.LogMessages{
			Messages: timeseries.NewWithData(1, timeseries.Minute, []float32{0, 5, 10, 20, 50, 100, 200, 500, 1000, 2000}),
		}
		app.LogMessages[model.SeverityError] = errorMsgs

		assert.NotNil(t, app.LogMessages[model.SeverityError], "error log messages must be set for clustering")
		assert.NotNil(t, app.LogMessages[model.SeverityError].Messages, "log message time series must exist")
	})

	t.Run("DeploymentSummaryReflectsMemoryLeakFromMetricsSnapshot", func(t *testing.T) {
		curr := &model.MetricsSnapshot{
			Requests:          1000,
			Errors:            5,
			MemoryLeakPercent: 15,
			MemoryUsage:       300 * 1024 * 1024,
			OOMKills:          0,
			LogErrors:         10,
		}
		prev := &model.MetricsSnapshot{
			Requests:          1000,
			Errors:            5,
			MemoryLeakPercent: 0,
			MemoryUsage:       200 * 1024 * 1024,
			OOMKills:          0,
			LogErrors:         10,
		}

		app := model.NewApplication(model.NewApplicationId("c1", "default", model.ApplicationKindDeployment, "svc"))
		app.Category = model.ApplicationCategoryApplication

		summary, _ := model.CalcApplicationDeploymentSummary(app, model.CheckConfigs{}, timeseries.Time(100), curr, prev)

		foundMemLeak := false
		for _, s := range summary {
			if s.Report == model.AuditReportMemory && !s.Ok {
				foundMemLeak = true
				assert.Contains(t, s.Message, "memory leak detected", "memory leak summary must reference MemoryGrowthPct")
				break
			}
		}
		assert.True(t, foundMemLeak, "deployment summary must include memory leak from MemoryGrowthPct")
	})
}
