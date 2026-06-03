package watchers

import (
	"fmt"
	"sync"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type Isolations struct {
	db       *db.DB
	notifier *notifications.IncidentNotifier
	mu       sync.Mutex
	counts   map[db.ProjectId]map[model.ApplicationId]int
}

func NewIsolations(db *db.DB, notifier *notifications.IncidentNotifier) *Isolations {
	return &Isolations{
		db:       db,
		notifier: notifier,
		counts:   make(map[db.ProjectId]map[model.ApplicationId]int),
	}
}

func (w *Isolations) Check(project *db.Project, world *model.World) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.counts[project.Id] == nil {
		w.counts[project.Id] = make(map[model.ApplicationId]int)
	}
	projCounts := w.counts[project.Id]

	for _, app := range world.Applications {
		if len(app.Deployments) < 2 {
			delete(projCounts, app.Id)
			continue
		}
		
		prevDeployment := app.Deployments[len(app.Deployments)-2]
		if prevDeployment.MetricsSnapshot == nil || prevDeployment.MetricsSnapshot.MemoryUsage <= 0 {
			delete(projCounts, app.Id)
			continue
		}
		
		prevMem := float32(prevDeployment.MetricsSnapshot.MemoryUsage)
		
		// Check current memory growth
		hasLeak := false
		for _, instance := range app.Instances {
			for _, container := range instance.Containers {
				// Reuse MemoryGrowthPct to evaluate if there is an active memory growth trend
				growthPct := auditor.MemoryGrowthPct(container.MemoryRss, prevMem, world.Ctx.To)
				
				currMem := container.MemoryRss.Last()
				if timeseries.IsNaN(currMem) {
					continue
				}
				
				diffPercent := (currMem - prevMem) / prevMem * 100
				
				// Exceeds previous version by 50% AND has a positive growth trend
				if diffPercent > 50 && growthPct > 0 {
					hasLeak = true
					break
				}
			}
			if hasLeak {
				break
			}
		}

		if hasLeak {
			projCounts[app.Id]++
			if projCounts[app.Id] == 3 {
				w.isolateService(project, app, world.Ctx.To)
				// Reset after isolation
				projCounts[app.Id] = 0
			}
		} else {
			projCounts[app.Id] = 0
		}
	}
}

func (w *Isolations) isolateService(project *db.Project, app *model.Application, now timeseries.Time) {
	klog.Infof("Isolating service %s from service mesh due to memory leak", app.Id)
	
	// Record isolation in stats
	_ = w.db.RecordIsolation(project.Id, app.Id.String(), int64(now))
	
	// Generate alert notification with RCA summary (reuse incident format)
	incident := &model.ApplicationIncident{
		Key:           fmt.Sprintf("isolation-%s-%d", app.Id.String(), now),
		ApplicationId: app.Id,
		Severity:      model.CRITICAL,
		OpenedAt:      now,
		RCA: &model.RCA{
			Status:         "OK",
			ShortSummary:   "Service automatically isolated from service mesh due to memory growth exceeding 50% of the previous version.",
			ImmediateFixes: "- Investigate memory leak in the current deployment.\n- Rollback to the previous version.\n- Remove the isolation after fixing the issue.",
		},
	}
	
	w.notifier.Enqueue(project, app, incident, now)
}
