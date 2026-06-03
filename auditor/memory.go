package auditor

import (
	"github.com/coroot/coroot/analysis"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
)

func (a *appAuditor) memory(ncs nodeConsumersByNode) {
	report := a.addReport(model.AuditReportMemory)
	relevantNodes := map[string]*model.Node{}

	oomCheck := report.CreateCheck(model.Checks.MemoryOOM)
	pressureCheck := report.CreateCheck(model.Checks.MemoryPressure)
	leakCheck := report.CreateCheck(model.Checks.MemoryLeakPercent)

	usageChart := report.GetOrCreateChartGroup(
		"Memory usage <selector>, bytes",
		model.NewDocLink("inspections", "memory", "memory-usage"),
	)
	pressureChart := report.GetOrCreateChartGroup(
		"Memory stall time <selector>, seconds per second",
		nil,
	)
	oomChart := report.GetOrCreateChart(
		"Out of memory events",
		model.NewDocLink("inspections", "memory", "out-of-memory-events"),
	).Column()
	nodesChart := report.GetOrCreateChart(
		"Node memory usage (unreclaimable), %",
		model.NewDocLink("inspections", "memory", "node-memory-usage-unreclaimable"),
	)
	consumersChart := report.GetOrCreateChartGroup(
		"Memory consumers <selector>, bytes",
		model.NewDocLink("inspections", "memory", "memory-consumers"),
	)

	oomCheck.AddWidget(oomChart.Widget())
	oomCheck.AddWidget(usageChart.Widget())
	leakCheck.AddWidget(usageChart.Widget())
	pressureCheck.AddWidget(pressureChart.Widget())

	seenContainers := false
	limitByContainer := map[string]*timeseries.Aggregate{}
	rssByContainer := map[string]map[string]*timeseries.TimeSeries{}
	periodicJob := a.app.PeriodicJob()
	for _, i := range a.app.Instances {
		oom := timeseries.NewAggregate(timeseries.NanSum)
		instanceRss := timeseries.NewAggregate(timeseries.NanSum)
		instancePageCache := timeseries.NewAggregate(timeseries.NanSum)
		pressureSome := timeseries.NewAggregate(timeseries.NanSum)
		pressureFull := timeseries.NewAggregate(timeseries.NanSum)

		for _, c := range i.Containers {
			seenContainers = true
			if limitByContainer[c.Name] == nil {
				limitByContainer[c.Name] = timeseries.NewAggregate(timeseries.Max)
			}
			limitByContainer[c.Name].Add(c.MemoryLimit)
			if rssByContainer[c.Name] == nil {
				rssByContainer[c.Name] = map[string]*timeseries.TimeSeries{}
			}
			rssByContainer[c.Name][i.Name] = c.MemoryRss
			if usageChart != nil {
				usageChart.GetOrCreateChart("RSS container: "+c.Name).AddSeries(i.Name, c.MemoryRss)
			}
			oom.Add(c.OOMKills)
			instanceRss.Add(c.MemoryRss)
			instancePageCache.Add(c.MemoryCache)
			pressureSome.Add(c.MemoryPressureSome)
			pressureFull.Add(c.MemoryPressureFull)
			if v := c.MemoryPressureSome.Last(); v > pressureCheck.Threshold {
				pressureCheck.AddItem("%s", i.Name)
			}
		}
		if pressureChart != nil {
			pressureChart.GetOrCreateChart("some").AddSeries(i.Name, pressureSome).Feature()
			pressureChart.GetOrCreateChart("full").AddSeries(i.Name, pressureFull)
		}

		if usageChart != nil {
			usageChart.GetOrCreateChart("RSS").AddSeries(i.Name, instanceRss)
			usageChart.GetOrCreateChart("RSS + PageCache").AddSeries(i.Name, timeseries.Sum(instanceRss.Get(), instancePageCache.Get()))
		}

		oomTs := oom.Get()
		if oomChart != nil {
			oomChart.AddSeries(i.Name, oomTs)
		}
		if ooms := oomTs.Reduce(timeseries.NanSum); ooms > 0 {
			oomCheck.Inc(int64(ooms))
			oomCheck.SetValue(oomCheck.Value() + float32(int(ooms)))
		}

		if node := i.Node; node != nil {
			nodeName := node.GetName()
			if relevantNodes[nodeName] != nil {
				continue
			}
			relevantNodes[nodeName] = node
			if nodesChart != nil {
				nodesChart.AddSeries(
					nodeName,
					timeseries.Aggregate2(
						node.MemoryAvailableBytes, node.MemoryTotalBytes,
						func(avail, total float32) float32 { return (total - avail) / total * 100 },
					),
				)
			}
			if consumersChart != nil {
				consumersChart.GetOrCreateChart(nodeName).
					Stacked().
					SetThreshold("total", node.MemoryTotalBytes).
					AddMany(ncs.get(node).memory, 5, timeseries.Max)
			}
		}
	}

	if usageChart != nil {
		for container, limit := range limitByContainer {
			usageChart.GetOrCreateChart("RSS container: "+container).SetThreshold("limit", limit.Get()).Feature()
		}
	}

	if periodicJob {
		leakCheck.SetStatus(model.UNKNOWN, "not checked for periodic jobs")
	} else {
		var maxPct float32
		for container, byInstance := range rssByContainer {
			limit := limitByContainer[container].Get().Reduce(timeseries.Max)
			for _, rss := range byInstance {
				if pct := analysis.MemoryGrowthPct(rss, limit, a.w.Ctx.To); pct > maxPct {
					maxPct = pct
				}
			}
		}
		if maxPct > 0 {
			leakCheck.SetValue(maxPct)
		}
	}

	if usageChart != nil {
		for _, ch := range usageChart.Charts {
			ch.DrillDownLink = model.NewRouterLink("profile", "overview").
				SetParam("view", "applications").
				SetParam("id", a.app.Id).
				SetParam("report", model.AuditReportProfiling).
				SetArg("query", model.ProfileCategoryMemory)
		}
	}
	if !seenContainers {
		oomCheck.SetStatus(model.UNKNOWN, "no data")
		leakCheck.SetStatus(model.UNKNOWN, "no data")
		pressureCheck.SetStatus(model.UNKNOWN, "no data")
		return
	}
}
