package auditor

import (
	"cmp"
	"fmt"
	"math"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

type DeploymentMemoryResult struct {
	DeploymentId    string
	MemoryLeakPct   float32
	MemoryUsage     int64
	OOMKills        int64
	ContainerCount  int
	InstanceCount   int
	HasLimit        bool
	MemoryLimitMax  float32
	MemoryGrowthRss map[string]float32
}

func AnalyzeDeploymentMemory(app *model.Application, to timeseries.Time) DeploymentMemoryResult {
	result := DeploymentMemoryResult{
		MemoryGrowthRss: map[string]float32{},
	}

	if app == nil {
		return result
	}

	result.InstanceCount = len(app.Instances)

	for _, i := range app.Instances {
		for _, c := range i.Containers {
			result.ContainerCount++

			limit := c.MemoryLimit.Reduce(timeseries.Max)
			if !timeseries.IsNaN(limit) && limit > 0 {
				result.HasLimit = true
				if limit > result.MemoryLimitMax {
					result.MemoryLimitMax = limit
				}
			}

			rss := c.MemoryRss
			if !rss.IsEmpty() {
				avgRss := rss.Reduce(timeseries.NanSum)
				count := rss.Map(timeseries.Defined).Reduce(timeseries.NanSum)
				if count > 0 && !timeseries.IsNaN(avgRss) {
					result.MemoryUsage += int64(avgRss / count)
				}
			}

			pct := MemoryGrowthPct(rss, limit, to)
			if pct > result.MemoryLeakPct {
				result.MemoryLeakPct = pct
			}

			key := cmp.Or(i.Name, "unknown") + "/" + cmp.Or(c.Name, "unknown")
			result.MemoryGrowthRss[key] = pct

			oomKills := c.OOMKills.Reduce(timeseries.NanSum)
			if !timeseries.IsNaN(oomKills) {
				result.OOMKills += int64(oomKills)
			}
		}
	}

	return result
}

func FormatMemoryLeakStatus(pct float32) (severity string, message string) {
	switch {
	case timeseries.IsNaN(pct) || pct <= 0:
		severity = "ok"
		message = "no memory leak detected"
	case pct < 1:
		severity = "warning"
		message = "minor memory growth detected"
	case pct < 5:
		severity = "warning"
		message = "moderate memory leak possible"
	case pct < 15:
		severity = "critical"
		message = "significant memory leak detected"
	default:
		severity = "critical"
		message = "severe memory leak detected"
	}

	if pct > 0 && !timeseries.IsNaN(pct) {
		dailyGrowth := pct * 24
		daysToDouble := float32(math.Log(2) / math.Log(1+float64(dailyGrowth)/100))
		if daysToDouble > 0 && daysToDouble < 365 {
			message += fmt.Sprintf(" (%.1f%% per hour, ~%.0f days to double)", pct, daysToDouble)
		} else {
			message += fmt.Sprintf(" (%.1f%% per hour)", pct)
		}
	}

	return severity, message
}