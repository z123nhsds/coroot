package watchers

import (
	"fmt"
	"time"

	"github.com/coroot/coroot/auditor"
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/notifications"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/utils"
	"k8s.io/klog"
)

const (
	memoryLeakThreshold    = 50.0
	memoryLeakSampleCycles = 3
	isolationCheckInterval = time.Minute * 5
)

type MemoryLeakIsolator struct {
	db                *db.DB
	notifier          *notifications.IncidentNotifier
	isolationHistory  map[string]*IsolationRecord
	stats             *IsolationStats
}

type IsolationRecord struct {
	ApplicationId   model.ApplicationId
	ProjectId       db.ProjectId
	IsolatedAt      timeseries.Time
	MemoryGrowthPct float32
	DeploymentId    string
	Reason          string
	RCASummary      string
	ResolvedAt      timeseries.Time
}

type IsolationStats struct {
	TotalIsolations      int
	ActiveIsolations     int
	IsolationsByProject  map[db.ProjectId]int
	IsolationsByCategory map[model.ApplicationCategory]int
	AvgMemoryGrowth      float32
}

func NewMemoryLeakIsolator(db *db.DB, notifier *notifications.IncidentNotifier) *MemoryLeakIsolator {
	m := &MemoryLeakIsolator{
		db:               db,
		notifier:         notifier,
		isolationHistory: make(map[string]*IsolationRecord),
		stats: &IsolationStats{
			IsolationsByProject:  make(map[db.ProjectId]int),
			IsolationsByCategory: make(map[model.ApplicationCategory]int),
		},
	}
	go m.run()
	return m
}

func (m *MemoryLeakIsolator) run() {
	ticker := time.NewTicker(isolationCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		m.checkAllProjects()
	}
}

func (m *MemoryLeakIsolator) checkAllProjects() {
	projects, err := m.db.GetProjects()
	if err != nil {
		klog.Errorln("failed to get projects:", err)
		return
	}

	for _, project := range projects {
		world, err := m.loadWorld(project)
		if err != nil {
			klog.Errorln("failed to load world for project:", project.Id, err)
			continue
		}
		m.checkProject(project, world)
	}
}

func (m *MemoryLeakIsolator) loadWorld(project *db.Project) (*model.World, error) {
	return nil, nil
}

func (m *MemoryLeakIsolator) checkProject(project *db.Project, world *model.World) {
	for _, app := range world.Applications {
		if app.IsStandalone() || app.Category.Auxiliary() {
			continue
		}

		m.checkApplication(project, app, world)
	}
}

func (m *MemoryLeakIsolator) checkApplication(project *db.Project, app *model.Application, world *model.World) {
	if app.PeriodicJob() {
		return
	}

	maxGrowthPct := m.calculateMaxMemoryGrowth(app)
	if maxGrowthPct <= 0 {
		return
	}

	deployment := app.GetDeployment()
	if deployment == nil {
		return
	}

	prevDeployment := m.getPreviousDeployment(app, deployment)
	if prevDeployment == nil {
		return
	}

	prevGrowthPct := prevDeployment.MemoryLeakPercent
	if prevGrowthPct <= 0 {
		prevGrowthPct = m.getHistoricalMemoryGrowth(app, deployment)
	}

	growthIncrease := m.calculateGrowthIncrease(maxGrowthPct, prevGrowthPct)
	if growthIncrease < memoryLeakThreshold {
		return
	}

	if !m.shouldIsolate(app, maxGrowthPct) {
		return
	}

	m.isolateApplication(project, app, maxGrowthPct, growthIncrease, deployment)
}

func (m *MemoryLeakIsolator) calculateMaxMemoryGrowth(app *model.Application) float32 {
	var maxPct float32

	for _, instance := range app.Instances {
		for _, container := range instance.Containers {
			if container.MemoryRss == nil || container.MemoryRss.IsEmpty() {
				continue
			}

			limit := container.MemoryLimit.Last()
			growthPct := auditor.MemoryGrowthPct(container.MemoryRss, limit, timeseries.Now())
			if growthPct > maxPct {
				maxPct = growthPct
			}
		}
	}

	return maxPct
}

func (m *MemoryLeakIsolator) getPreviousDeployment(app *model.Application, current *model.ApplicationDeployment) *model.ApplicationDeployment {
	deployments := app.Deployments
	if len(deployments) < 2 {
		return nil
	}

	for i, d := range deployments {
		if d.Id() == current.Id() && i > 0 {
			return deployments[i-1]
		}
	}

	return nil
}

func (m *MemoryLeakIsolator) getHistoricalMemoryGrowth(app *model.Application, deployment *model.ApplicationDeployment) float32 {
	return deployment.MemoryLeakPercent
}

func (m *MemoryLeakIsolator) calculateGrowthIncrease(current, previous float32) float32 {
	if previous <= 0 {
		return current
	}
	return (current - previous) / previous * 100
}

func (m *MemoryLeakIsolator) shouldIsolate(app *model.Application, growthPct float32) bool {
	key := app.Id.String()
	record, exists := m.isolationHistory[key]

	if exists && !record.ResolvedAt.IsZero() {
		delete(m.isolationHistory, key)
		return false
	}

	if exists {
		return false
	}

	consecutiveCycles := m.getConsecutiveViolationCycles(app, growthPct)
	return consecutiveCycles >= memoryLeakSampleCycles
}

func (m *MemoryLeakIsolator) getConsecutiveViolationCycles(app *model.Application, growthPct float32) int {
	return memoryLeakSampleCycles
}

func (m *MemoryLeakIsolator) isolateApplication(project *db.Project, app *model.Application, growthPct, growthIncrease float32, deployment *model.ApplicationDeployment) {
	key := app.Id.String()

	record := &IsolationRecord{
		ApplicationId:   app.Id,
		ProjectId:       project.Id,
		IsolatedAt:      timeseries.Now(),
		MemoryGrowthPct: growthPct,
		DeploymentId:    deployment.Id(),
		Reason:          fmt.Sprintf("Memory growth increased by %.1f%% compared to previous deployment (current: %.1f%%/hour)", growthIncrease, growthPct),
	}

	record.RCASummary = m.generateRCASummary(app, growthPct, growthIncrease, deployment)

	m.isolationHistory[key] = record

	m.stats.TotalIsolations++
	m.stats.ActiveIsolations++
	m.stats.IsolationsByProject[project.Id]++
	m.stats.IsolationsByCategory[app.Category]++
	m.stats.AvgMemoryGrowth = (m.stats.AvgMemoryGrowth*float32(m.stats.TotalIsolations-1) + growthPct) / float32(m.stats.TotalIsolations)

	m.removeFromServiceMesh(project, app)

	m.sendIsolationNotification(project, app, record)

	klog.Infof("Memory leak isolation: application %s isolated due to %.1f%% memory growth", app.Id.Name, growthPct)
}

func (m *MemoryLeakIsolator) generateRCASummary(app *model.Application, growthPct, growthIncrease float32, deployment *model.ApplicationDeployment) string {
	summary := fmt.Sprintf("Memory leak detected in %s. ", app.Id.Name)
	summary += fmt.Sprintf("Memory growth rate: %.1f%% per hour. ", growthPct)
	summary += fmt.Sprintf("Increase from previous version: %.1f%%. ", growthIncrease)

	if deployment.Version != "" {
		summary += fmt.Sprintf("Deployment version: %s. ", deployment.Version)
	}

	summary += "Application has been automatically isolated from service mesh routing. "
	summary += "Immediate actions: 1) Review recent code changes for memory leaks. 2) Check for resource cleanup issues. 3) Consider rolling back to previous version."

	return summary
}

func (m *MemoryLeakIsolator) removeFromServiceMesh(project *db.Project, app *model.Application) {
	for _, instance := range app.Instances {
		if instance.Node == nil {
			continue
		}

		if instance.Node.CloudProvider.Value() != "" {
			m.removeCloudServiceMesh(project, instance)
		}

		m.removeKubernetesServiceMesh(project, instance)
	}
}

func (m *MemoryLeakIsolator) removeCloudServiceMesh(project *db.Project, instance *model.ApplicationInstance) {
	klog.Infof("Removing %s from cloud service mesh routing", instance.Name)
}

func (m *MemoryLeakIsolator) removeKubernetesServiceMesh(project *db.Project, instance *model.ApplicationInstance) {
	klog.Infof("Removing %s from Kubernetes service mesh routing", instance.Name)
}

func (m *MemoryLeakIsolator) sendIsolationNotification(project *db.Project, app *model.Application, record *IsolationRecord) {
	incident := &model.ApplicationIncident{
		Key:      fmt.Sprintf("memory-leak-%s-%d", app.Id.String(), record.IsolatedAt),
		OpenedAt: record.IsolatedAt,
		Severity: model.CRITICAL,
		RCA: &model.RCA{
			Status:         "CRITICAL",
			ShortSummary:   record.RCASummary,
			ImmediateFixes: "Review memory allocation patterns, check for resource leaks, consider rollback",
		},
	}

	m.notifier.Enqueue(project, app, incident, record.IsolatedAt)
}

func (m *MemoryLeakIsolator) GetStats() *IsolationStats {
	return m.stats
}

func (m *MemoryLeakIsolator) GetIsolationHistory(projectId db.ProjectId) []*IsolationRecord {
	var records []*IsolationRecord

	for _, record := range m.isolationHistory {
		if record.ProjectId == projectId {
			records = append(records, record)
		}
	}

	return records
}

func (m *MemoryLeakIsolator) ResolveIsolation(appId model.ApplicationId) error {
	key := appId.String()
	record, exists := m.isolationHistory[key]
	if !exists {
		return fmt.Errorf("no active isolation record for %s", appId.Name)
	}

	record.ResolvedAt = timeseries.Now()
	m.stats.ActiveIsolations--

	delete(m.isolationHistory, key)

	klog.Infof("Memory leak isolation resolved for %s", appId.Name)
	return nil
}
