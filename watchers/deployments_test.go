package watchers

import (
	"fmt"
	"strings"
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
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

func TestCalcMetricsSnapshotReusesAppMemoryGrowthPct(t *testing.T) {
	app := model.NewApplication(model.NewApplicationId("", "default", model.ApplicationKindDeployment, "catalog"))
	first := app.GetOrCreateInstance("i1", nil)
	second := app.GetOrCreateInstance("i2", nil)

	leakingRSS := timeseries.NewWithData(0, 10*timeseries.Minute, []float32{
		1 << 30,
		1<<30 + 60<<20,
		1<<30 + 120<<20,
		1<<30 + 180<<20,
		1<<30 + 240<<20,
		1<<30 + 300<<20,
		1<<30 + 360<<20,
		1<<30 + 420<<20,
		1<<30 + 480<<20,
		1<<30 + 540<<20,
		1<<30 + 600<<20,
		1<<30 + 660<<20,
	})
	firstLimit := timeseries.NewWithData(0, 10*timeseries.Minute, []float32{1.5 * 1024 * 1024 * 1024})
	secondLimit := timeseries.NewWithData(0, 10*timeseries.Minute, []float32{4 * 1024 * 1024 * 1024})

	first.Containers = []*model.Container{{Name: "api", MemoryRss: leakingRSS, MemoryLimit: firstLimit}}
	second.Containers = []*model.Container{{Name: "api", MemoryRss: timeseries.NewWithData(0, 10*timeseries.Minute, []float32{1 << 30}), MemoryLimit: secondLimit}}

	to := timeseries.Time(11 * 10 * timeseries.Minute)
	snapshot := calcMetricsSnapshot(app, 0, to, 10*timeseries.Minute)
	expected := auditor.MemoryGrowthPct(leakingRSS, secondLimit.Reduce(timeseries.Max), to)

	assert.InDelta(t, expected, snapshot.MemoryLeakPercent, 0.001)
}
