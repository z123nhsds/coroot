package model

import "github.com/coroot/coroot/timeseries"

type MemoryIsolationRecord struct {
	Id            string             `json:"id"`
	ApplicationId ApplicationId       `json:"application_id"`
	DeploymentId  string             `json:"deployment_id"`
	StartedAt     timeseries.Time    `json:"started_at"`
	ResolvedAt    timeseries.Time    `json:"resolved_at"`
	GrowthPercent float32            `json:"growth_percent"`
	Reason        string             `json:"reason"`
	Rca           *RcaSummary        `json:"rca,omitempty"`
}

type RcaSummary struct {
	Summary     string `json:"summary"`
	RootCause   string `json:"root_cause"`
	FixRecommendation string `json:"fix_recommendation"`
}

type MemoryIsolationState int

const (
	MemoryIsolationStateActive MemoryIsolationState = iota
	MemoryIsolationStateResolved
)
