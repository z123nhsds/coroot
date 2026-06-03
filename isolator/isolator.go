package isolator

import (
	"context"
	"fmt"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

const (
	memoryGrowthThresholdPct float32 = 50.0
	consecutiveCyclesRequired int    = 3
)

type Isolator struct {
	db       *db.DB
	mesh     MeshClient
	notifier *notifications.IncidentNotifier

	pendingConfirmations map[string]int
}

func NewIsolator(database *db.DB, mesh MeshClient) *Isolator {
	if mesh == nil {
		mesh = &NoOpMeshClient{}
	}
	return &Isolator{
		db:                   database,
		mesh:                 mesh,
		notifier:             notifications.NewIncidentNotifier(database),
		pendingConfirmations: map[string]int{},
	}
}

func (is *Isolator) Check(project *db.Project, world *model.World) {
	start := time.Now()
	now := timeseries.Now()

	var appsChecked int
	for _, app := range world.Applications {
		if app.Id.Kind != model.ApplicationKindDeployment {
			continue
		}
		if app.PeriodicJob() {
			continue
		}
		if len(app.Deployments) < 2 {
			continue
		}

		appsChecked++

		currDeploy := app.Deployments[len(app.Deployments)-1]
		prevDeploy := app.Deployments[len(app.Deployments)-2]

		currLeak := calcMemoryLeakPct(app, world.Ctx.To)
		if currLeak <= 0 {
			continue
		}

		prevLeak := float32(0)
		if prevDeploy.MetricsSnapshot != nil {
			prevLeak = prevDeploy.MetricsSnapshot.MemoryLeakPercent
		}

		appKey := app.Id.String()

		is.handleIsolationDecision(project, app, currDeploy, currLeak, prevLeak, appKey, now, world.Ctx.To)
	}

	klog.Infof("%s: checked %d apps for memory isolation in %s", project.Id, appsChecked, time.Since(start).Truncate(time.Millisecond))
}

func (is *Isolator) handleIsolationDecision(project *db.Project, app *model.Application, deploy *model.ApplicationDeployment, currLeak, prevLeak float32, appKey string, now, ctxTo timeseries.Time) {
	openRecord, err := is.db.GetOpenIsolationRecord(project.Id, app.Id)
	if err != nil {
		openRecord = nil
	}

	// 1. Check if we should trigger isolation
	if openRecord == nil {
		if prevLeak > 0 && currLeak > prevLeak*(1+memoryGrowthThresholdPct/100) {
			confirmations := is.pendingConfirmations[appKey]
			confirmations++
			is.pendingConfirmations[appKey] = confirmations

			if confirmations >= consecutiveCyclesRequired {
				delete(is.pendingConfirmations, appKey)
				is.triggerIsolation(project, app, deploy, currLeak, prevLeak, now, ctxTo)
			}
			return
		}
		delete(is.pendingConfirmations, appKey)
		return
	}

	// 2. Already isolated - check if we should restore
	if openRecord.State == model.IsolationStateIsolated {
		if currLeak <= prevLeak {
			is.restoreFromIsolation(project, app, openRecord, currLeak, now)
			return
		}
	}

	// 3. Pending - check if the condition still holds
	if openRecord.State == model.IsolationStatePending {
		if currLeak > prevLeak*(1+memoryGrowthThresholdPct/100) {
			confirmations := openRecord.Consecutive + 1
			openRecord.Consecutive = confirmations
			if confirmations >= consecutiveCyclesRequired {
				is.executeIsolation(project, app, openRecord, deploy, currLeak, now, ctxTo)
				return
			}
			openRecord.MemoryLeakPct = currLeak
			_ = is.db.UpdateIsolationRecord(project.Id, openRecord)
			return
		}
		openRecord.State = model.IsolationStateCancelled
		openRecord.ResolvedAt = now
		_ = is.db.UpdateIsolationRecord(project.Id, openRecord)
		is.pendingConfirmations[appKey] = 0
	}
}

func (is *Isolator) triggerIsolation(project *db.Project, app *model.Application, deploy *model.ApplicationDeployment, currLeak, prevLeak float32, now, ctxTo timeseries.Time) {
	rcaSummary := generateRCASummary(app, currLeak, prevLeak, ctxTo)

	record := &model.IsolationRecord{
		ApplicationId: app.Id,
		OpenedAt:      now,
		State:         model.IsolationStatePending,
		Deployment:    deploy.Name,
		Reason:        fmt.Sprintf("Memory leak detected: current growth %.1f%% exceeds previous %.1f%% by more than %.0f%%", currLeak, prevLeak, memoryGrowthThresholdPct),
		RCASummary:    rcaSummary,
		MemoryLeakPct: currLeak,
		PrevLeakPct:   prevLeak,
		Consecutive:   1,
	}

	if err := is.db.CreateIsolationRecord(project.Id, record); err != nil {
		klog.Errorln("failed to create isolation record:", err)
		return
	}

	is.notifyIsolation(project, app, record, now)
}

func (is *Isolator) executeIsolation(project *db.Project, app *model.Application, record *model.IsolationRecord, deploy *model.ApplicationDeployment, currLeak float32, now, ctxTo timeseries.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	isRemoved, err := is.mesh.IsRemoved(ctx, app)
	if err != nil {
		klog.Errorf("failed to check mesh status for %s: %s", app.Id, err)
		return
	}
	if isRemoved {
		klog.Infof("%s is already removed from mesh", app.Id)
		record.State = model.IsolationStateIsolated
		record.MemoryLeakPct = currLeak
		_ = is.db.UpdateIsolationRecord(project.Id, record)
		return
	}

	if err := is.mesh.RemoveFromMesh(ctx, app); err != nil {
		klog.Errorf("failed to remove %s from mesh: %s", app.Id, err)
		return
	}

	record.State = model.IsolationStateIsolated
	record.MemoryLeakPct = currLeak
	record.RCASummary = generateRCASummary(app, currLeak, record.PrevLeakPct, ctxTo)
	_ = is.db.UpdateIsolationRecord(project.Id, record)

	klog.Infof("%s has been isolated from service mesh due to memory leak (%.1f%% growth)", app.Id, currLeak)

	is.notifyIsolation(project, app, record, now)
}

func (is *Isolator) restoreFromIsolation(project *db.Project, app *model.Application, record *model.IsolationRecord, currLeak float32, now timeseries.Time) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := is.mesh.RestoreToMesh(ctx, app); err != nil {
		klog.Errorf("failed to restore %s to mesh: %s", app.Id, err)
		return
	}

	record.State = model.IsolationStateRestored
	record.ResolvedAt = now
	record.MemoryLeakPct = currLeak
	_ = is.db.UpdateIsolationRecord(project.Id, record)

	klog.Infof("%s has been restored to service mesh (memory leak resolved)", app.Id)

	is.notifyIsolation(project, app, record, now)
}

func (is *Isolator) notifyIsolation(project *db.Project, app *model.Application, record *model.IsolationRecord, now timeseries.Time) {
	incident := &model.ApplicationIncident{
		ApplicationId: app.Id,
		Key:           fmt.Sprintf("memory-isolation-%s", app.Id),
		OpenedAt:      record.OpenedAt,
		Severity:      model.CRITICAL,
		RCA: &model.RCA{
			Status:         "OK",
			ShortSummary:   record.RCASummary,
			ImmediateFixes: fmt.Sprintf("Memory leak detected in deployment %s. Growth rate: %.1f%% (previous: %.1f%%). The service has been %s from the service mesh.", record.Deployment, record.MemoryLeakPct, record.PrevLeakPct, record.State),
		},
	}

	if record.State == model.IsolationStateRestored || record.State == model.IsolationStateCancelled {
		incident.ResolvedAt = now
		incident.Severity = model.OK
	}

	is.notifier.Enqueue(project, app, incident, now)
}

func generateRCASummary(app *model.Application, currLeak, prevLeak float32, to timeseries.Time) string {
	if currLeak > 0 && prevLeak > 0 {
		growth := (currLeak - prevLeak) / prevLeak * 100
		return fmt.Sprintf("Memory leak escalation in %s/%s: estimated growth rate accelerated from %.1f%% to %.1f%% (+%.0f%%). Suspect latest deployment introduced unbounded memory retention. Inspect heap profiles and GC metrics for the affected containers.",
			app.Id.Namespace, app.Id.Name, prevLeak, currLeak, growth)
	}
	return fmt.Sprintf("Memory leak detected in %s/%s: estimated growth rate %.1f%%. The application exhibits progressive RSS increase without stabilization. Recommend reviewing recent code changes and analyzing heap dumps.",
		app.Id.Namespace, app.Id.Name, currLeak)
}

func calcMemoryLeakPct(app *model.Application, to timeseries.Time) float32 {
	var maxPct float32
	for _, instance := range app.Instances {
		for _, container := range instance.Containers {
			limit := container.MemoryLimit.Reduce(timeseries.Max)
			if pct := auditor.MemoryGrowthPct(container.MemoryRss, limit, to); pct > maxPct {
				maxPct = pct
			}
		}
	}
	return maxPct
}