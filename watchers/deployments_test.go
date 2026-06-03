package watchers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"github.com/stretchr/testify/assert"
)

func TestCalcDeployments(t *testing.T) {
	var app *model.Application
	createApp := func() {
		app = model.NewApplication(model.NewApplicationId("", "default", model.ApplicationKindDeployment, "catalog"))
	}
	addInstance := func(name string, rs string, lifeSpan ...float32) {
		i := app.GetOrCreateInstance(name, nil)
		i.Pod = &model.Pod{ReplicaSet: rs}
		i.Pod.LifeSpan = timeseries.NewWithData(1, 1, lifeSpan)
	}
	checkDeployments := func(expected string) {
		var actual []string
		for _, d := range calcDeployments(app) {
			actual = append(actual, fmt.Sprintf("%d-%d:%s", d.StartedAt, d.FinishedAt, d.Name))
		}
		assert.Equal(t, expected, strings.Join(actual, ";"))
	}

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs1", 0, 0, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkDeployments("4-4:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 0, 0)
	addInstance("i2", "rs2", 0, 0, 1, 1, 1, 1)
	checkDeployments("3-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 0, 0, 0, 1, 1)
	checkDeployments("5-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 0, 1, 0, 0, 1, 1)
	checkDeployments("2-5:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 1, 1)
	addInstance("i2", "rs2", 0, 0, 1, 1, 0, 0)
	checkDeployments("3-3:rs2;5-5:rs1")

	createApp()
	addInstance("i1", "rs1", 1, 0, 0, 0, 0, 0)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 0, 0)
	addInstance("i2", "rs2", 1, 1, 0, 0, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 0)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 1, 1, 1, 1, 1, 1)
	checkDeployments("")

	createApp()
	addInstance("i1", "rs1", 1, 1, 0, 0, 1, 1)
	addInstance("i2", "rs2", 1, 1, 1, 1, 0, 0)
	checkDeployments("5-5:rs1")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 0, 0, 0, 1, 1, 1)
	checkDeployments("4-0:rs2")

	createApp()
	addInstance("i1", "rs1", 1, 1, 1, 1, 1, 1)
	addInstance("i2", "rs2", 0, 0, 1, 1, 0, 0)
	checkDeployments("3-0:rs2;5-5:rs1")
}

func TestCalcMetricsSnapshotTracksMemoryGrowthWithoutBreakingLogClusters(t *testing.T) {
	const mb = float32(1024 * 1024)

	from := timeseries.Time(0)
	step := 5 * timeseries.Minute
	points := 13
	to := from.Add(step * timeseries.Duration(points-1))

	app := model.NewApplication(model.NewApplicationId("cluster-a", "default", model.ApplicationKindDeployment, "checkout"))
	instance := app.GetOrCreateInstance("checkout-0", nil)
	container := model.NewContainer("/k8s/default/checkout-0/app", "app")

	rss := make([]float32, points)
	memoryLimit := make([]float32, points)
	zeroes := make([]float32, points)
	for i := range rss {
		rss[i] = (400 + float32(i)*16) * mb
		memoryLimit[i] = 1024 * mb
	}
	container.MemoryRss = timeseries.NewWithData(from, step, rss)
	container.MemoryLimit = timeseries.NewWithData(from, step, memoryLimit)
	container.CpuUsage = timeseries.NewWithData(from, step, zeroes)
	container.Restarts = timeseries.NewWithData(from, step, zeroes)
	container.OOMKills = timeseries.NewWithData(from, step, zeroes)
	instance.Containers[container.Id] = container

	app.LogMessages[model.SeverityError] = &model.LogMessages{
		Messages: timeseries.NewWithData(from, step, []float32{1, 0, 2, 0, 1, 0, 1, 0, 2, 0, 1, 0, 1}),
		Patterns: map[string]*model.LogPattern{
			"primary": {SimilarPatternHashes: utils.NewStringSet("primary", "secondary")},
			"secondary": {SimilarPatternHashes: utils.NewStringSet("primary", "secondary")},
		},
	}
	app.LogMessages[model.SeverityWarning] = &model.LogMessages{
		Messages: timeseries.NewWithData(from, step, []float32{0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0, 1, 0}),
		Patterns: map[string]*model.LogPattern{},
	}

	assert.ElementsMatch(t, []string{"primary", "secondary"}, app.SimilarLogPatternHashes("primary"))

	snapshot := calcMetricsSnapshot(app, from, to, step)

	assert.NotNil(t, snapshot)
	assert.Greater(t, snapshot.MemoryLeakPercent, float32(40))
	assert.EqualValues(t, 9, snapshot.LogErrors)
	assert.EqualValues(t, 6, snapshot.LogWarnings)
	assert.ElementsMatch(t, []string{"primary", "secondary"}, app.SimilarLogPatternHashes("primary"))
}
