package isolation

import (
	"github.com/coroot/coroot/db"
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"github.com/coroot/coroot/watchers"
)

type View struct {
	Isolations     []IsolationRecord `json:"isolations"`
	Stats          *Stats            `json:"stats"`
}

type IsolationRecord struct {
	Id              string              `json:"id"`
	ApplicationId   model.ApplicationId `json:"application_id"`
	ProjectId       db.ProjectId        `json:"project_id"`
	IsolatedAt      timeseries.Time     `json:"isolated_at"`
	MemoryGrowthPct float32             `json:"memory_growth_pct"`
	DeploymentId    string              `json:"deployment_id"`
	Reason          string              `json:"reason"`
	RCASummary      string              `json:"rca_summary"`
	ResolvedAt      timeseries.Time     `json:"resolved_at"`
	IsActive        bool                `json:"is_active"`
}

type Stats struct {
	TotalIsolations      int                           `json:"total_isolations"`
	ActiveIsolations     int                           `json:"active_isolations"`
	IsolationsByProject  map[db.ProjectId]int          `json:"isolations_by_project"`
	IsolationsByCategory map[model.ApplicationCategory]int `json:"isolations_by_category"`
	AvgMemoryGrowth      float32                       `json:"avg_memory_growth"`
}

func Render(isolator *watchers.MemoryLeakIsolator, projectId db.ProjectId) *View {
	v := &View{}

	stats := isolator.GetStats()
	v.Stats = &Stats{
		TotalIsolations:      stats.TotalIsolations,
		ActiveIsolations:     stats.ActiveIsolations,
		IsolationsByProject:  stats.IsolationsByProject,
		IsolationsByCategory: stats.IsolationsByCategory,
		AvgMemoryGrowth:      stats.AvgMemoryGrowth,
	}

	records := isolator.GetIsolationHistory(projectId)
	for _, record := range records {
		v.Isolations = append(v.Isolations, IsolationRecord{
			Id:              record.ApplicationId.String() + "-" + record.IsolatedAt.String(),
			ApplicationId:   record.ApplicationId,
			ProjectId:       record.ProjectId,
			IsolatedAt:      record.IsolatedAt,
			MemoryGrowthPct: record.MemoryGrowthPct,
			DeploymentId:    record.DeploymentId,
			Reason:          record.Reason,
			RCASummary:      record.RCASummary,
			ResolvedAt:      record.ResolvedAt,
			IsActive:        record.ResolvedAt.IsZero(),
		})
	}

	return v
}
