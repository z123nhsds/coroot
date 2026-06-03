package main

import (
	"testing"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/watchers"
	"github.com/stretchr/testify/assert"
)

func TestMemoryGrowthPctIntegration(t *testing.T) {
	// Test MemoryGrowthPct calculation in isolation
	t.Run("MemoryGrowthPct calculation", func(t *testing.T) {
		ts := timeseries.New(1, 1)
		// Create a time series with steady growth
		for i := 0; i < 60; i++ {
			// Start with 100MB and grow by 1MB each step
			val := float32(100*1024*1024 + i*1024*1024)
			ts = ts.Add(timeseries.Time(i), val)
		}
		limit := float32(1024 * 1024 * 1024) // 1GB limit
		to := timeseries.Time(60)

		growthPct := auditor.MemoryGrowthPct(ts, limit, to)
		assert.Greater(t, growthPct, float32(0))
		t.Logf("Calculated memory growth: %.2f%%", growthPct)
	})

	// Test that MemoryGrowthPct can be used in deployment metrics
	t.Run("Deployment metrics snapshot with MemoryGrowthPct", func(t *testing.T) {
		app := createTestApplicationWithMemoryGrowth()
		from := timeseries.Time(0)
		to := timeseries.Time(100)
		step := timeseries.Duration(1)

		ms := watchers.CalcMetricsSnapshot(app, from, to, step) // 注意：我将这个函数名调整为导出，方便测试，实际代码中是 calcMetricsSnapshot
		assert.NotNil(t, ms)
		assert.GreaterOrEqual(t, ms.MemoryLeakPercent, float32(0))
		t.Logf("Memory leak percent in snapshot: %.2f%%", ms.MemoryLeakPercent)
	})

	// Test log messages and patterns are processed alongside
	t.Run("Log patterns with memory growth", func(t *testing.T) {
		app := createTestApplicationWithMemoryGrowth()
		// Add some log messages
		logPatterns := map[model.LogPatternHash]*model.LogPattern{
			"pattern1": {
				Pattern:  model.NewLogPattern("ERROR", "connection timeout"),
				Messages: timeseries.NewWithData(1, 1, 5, 3, 2, 4, 6),
				Sample:   "ERROR: connection timeout to db",
			},
		}
		app.LogMessages[model.SeverityError] = &model.LogMessages{
			Patterns: logPatterns,
		}

		// Verify log patterns exist
		assert.NotEmpty(t, app.LogMessages)
		assert.NotEmpty(t, app.LogMessages[model.SeverityError].Patterns)
	})
}

func TestDeploymentDetection(t *testing.T) {
	app := createTestApplicationWithMemoryGrowth()
	deployments := watchers.CalcDeployments(app) // 注意：我将这个函数名调整为导出，方便测试，实际代码中是 calcDeployments
	
	// For this test, we're not simulating a real deployment,
	// just checking that the function can be called without errors
	assert.NotPanics(t, func() {
		_ = watchers.CalcDeployments(app)
	})
}

func createTestApplicationWithMemoryGrowth() *model.Application {
	app := model.NewApplication(model.NewApplicationId("cluster1", "default", model.ApplicationKindDeployment, "test-app"))
	
	// Create an instance with memory growth
	instance := model.NewInstance("test-instance-1", nil)
	
	container := &model.Container{
		Name: "test-container",
		MemoryRss: timeseries.New(1, 1),
		MemoryLimit: timeseries.NewWithData(1, 1, float32(1024*1024*1024)), // 1GB limit
	}
	
	// Add memory usage data with growth
	for i := 0; i < 100; i++ {
		// Start with 100MB and grow
		val := float32(100*1024*1024 + i*1024*1024)
		container.MemoryRss = container.MemoryRss.Add(timeseries.Time(i), val)
	}
	
	instance.Containers = append(instance.Containers, container)
	app.Instances = append(app.Instances, instance)
	
	// Add a pod with replica set for deployment detection
	instance.Pod = &model.Pod{
		ReplicaSet: "rs-v1",
		LifeSpan:   timeseries.NewWithData(1, 1, 1, 1, 1, 1),
	}
	
	return app
}

/*
注意：为了使这些测试能够在实际代码中运行，需要做以下修改：
1. 将 watchers 包中的 calcMetricsSnapshot 和 calcDeployments 函数导出为公共函数
2. 或者创建一个测试辅助函数，间接调用这些私有函数
*/
