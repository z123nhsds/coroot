package auditor

import (
	"fmt"
	"sync"

	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/google/uuid"
	"k8s.io/klog"
)

var (
	memoryGrowthHistory = make(map[string][]float32)
	memoryGrowthHistoryMu sync.Mutex
	globalDb *db.DB
)

func SetGlobalDb(d *db.DB) {
	globalDb = d
}

func (a *appAuditor) memoryIsolation() {
	if a.app.PeriodicJob() {
		return
	}
	if len(a.app.Deployments) < 2 {
		return
	}
	if globalDb == nil {
		return
	}

	report := a.addReport(model.AuditReportMemory)
	check := report.CreateCheck(model.Checks.MemoryIsolation)

	// 获取当前和上一个部署
	currentDeployment := a.app.Deployments[len(a.app.Deployments)-1]
	prevDeployment := a.app.Deployments[len(a.app.Deployments)-2]

	// 检查是否有 MetricsSnapshot
	if currentDeployment.MetricsSnapshot == nil || prevDeployment.MetricsSnapshot == nil {
		return
	}

	// 计算内存增长百分比
	var growthPercent float32
	if prevDeployment.MetricsSnapshot.MemoryUsage > 0 {
		growthPercent = (float32(currentDeployment.MetricsSnapshot.MemoryUsage) - float32(prevDeployment.MetricsSnapshot.MemoryUsage)) / float32(prevDeployment.MetricsSnapshot.MemoryUsage) * 100
	} else if currentDeployment.MetricsSnapshot.MemoryUsage > 0 {
		growthPercent = 100
	}

	// 获取配置的阈值
	config := a.w.CheckConfigs.GetSimple(model.Checks.MemoryIsolation.Id, a.app.Id)
	threshold := config.Threshold
	if threshold <= 0 {
		threshold = 50 // 默认 50%
	}

	// 记录增长历史，用于判断是否持续 3 个采样周期
	appKey := fmt.Sprintf("%s:%s", a.p.Id, a.app.Id)
	memoryGrowthHistoryMu.Lock()
	defer memoryGrowthHistoryMu.Unlock()

	history := memoryGrowthHistory[appKey]
	history = append(history, growthPercent)
	if len(history) > 10 {
		history = history[len(history)-10:]
	}
	memoryGrowthHistory[appKey] = history

	// 检查是否有活跃的隔离
	activeIsolations, err := globalDb.GetActiveMemoryIsolations(a.p.Id)
	if err != nil {
		klog.Errorln("Failed to get active memory isolations:", err)
		return
	}

	// 检查当前应用是否已被隔离
	var currentIsolation *model.MemoryIsolationRecord
	for _, iso := range activeIsolations {
		if iso.ApplicationId.String() == a.app.Id.String() && iso.DeploymentId == currentDeployment.Id() {
			currentIsolation = iso
			break
		}
	}

	// 判断是否需要隔离
	needsIsolation := false
	if growthPercent > threshold {
		// 检查是否持续 3 个采样周期都超过阈值
		consecutiveHigh := 0
		for _, h := range history {
			if h > threshold {
				consecutiveHigh++
			} else {
				consecutiveHigh = 0
			}
		}
		if consecutiveHigh >= 3 {
			needsIsolation = true
		}
	}

	if needsIsolation && currentIsolation == nil {
		// 执行隔离
		check.SetValue(growthPercent)
		check.AddItem(fmt.Sprintf("Memory growth: %.2f%%, threshold: %.2f%%", growthPercent, threshold))
		check.SetStatus(model.CRITICAL, "Service automatically isolated due to high memory growth")

		// 创建 RCA 摘要
		rca := &model.RcaSummary{
			Summary: fmt.Sprintf("Memory usage grew %.2f%% compared to previous deployment", growthPercent),
			RootCause: "Potential memory leak in the new deployment",
			FixRecommendation: "Rollback to the previous version and investigate the memory issue",
		}

		// 创建隔离记录
		record := &model.MemoryIsolationRecord{
			Id:             uuid.New().String(),
			ApplicationId:  a.app.Id,
			DeploymentId:   currentDeployment.Id(),
			StartedAt:      timeseries.Now(),
			GrowthPercent:  growthPercent,
			Reason:         fmt.Sprintf("Memory growth exceeded threshold (%.2f%% > %.2f%%) for 3 consecutive periods", growthPercent, threshold),
			Rca:            rca,
		}

		if err := globalDb.CreateMemoryIsolationRecord(a.p.Id, record); err != nil {
			klog.Errorln("Failed to create memory isolation record:", err)
		} else {
			klog.Infof("Created memory isolation record for %s, growth: %.2f%%", a.app.Id, growthPercent)
		}

		// TODO: 这里需要实现实际的服务网格隔离逻辑，比如更新配置、调用 API 等
	} else if currentIsolation != nil && !needsIsolation {
		// 检查是否可以恢复
		// 如果内存增长正常，可以考虑自动恢复或等待人工干预
		check.SetStatus(model.WARNING, "Service currently isolated, but memory growth has normalized")
	}
}
