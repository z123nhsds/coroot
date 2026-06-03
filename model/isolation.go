package model

import (
	"github.com/coroot/coroot/timeseries"
)

type IsolationState string

const (
	IsolationStatePending   IsolationState = "pending"
	IsolationStateIsolated  IsolationState = "isolated"
	IsolationStateRestored  IsolationState = "restored"
	IsolationStateCancelled IsolationState = "cancelled"
)

type IsolationRecord struct {
	ApplicationId ApplicationId     `json:"application_id"`
	OpenedAt      timeseries.Time   `json:"opened_at"`
	ResolvedAt    timeseries.Time   `json:"resolved_at"`
	State         IsolationState    `json:"state"`
	Deployment    string            `json:"deployment"`
	Reason        string            `json:"reason"`
	RCASummary    string            `json:"rca_summary"`
	MemoryLeakPct float32           `json:"memory_leak_pct"`
	PrevLeakPct   float32           `json:"prev_leak_pct"`
	Consecutive   int               `json:"consecutive"`
}

func (r *IsolationRecord) IsOpen() bool {
	return r.State == IsolationStatePending || r.State == IsolationStateIsolated
}

func (r *IsolationRecord) IsResolved() bool {
	return r.State == IsolationStateRestored || r.State == IsolationStateCancelled
}

type IsolationSummary struct {
	ApplicationId ApplicationId   `json:"application_id"`
	State         IsolationState  `json:"state"`
	OpenedAt      timeseries.Time `json:"opened_at"`
	ResolvedAt    timeseries.Time `json:"resolved_at"`
	MemoryLeakPct float32         `json:"memory_leak_pct"`
	RCASummary    string          `json:"rca_summary"`
}

type IsolationStats struct {
	TotalIsolations    int64 `json:"total_isolations"`
	ActiveIsolations   int64 `json:"active_isolations"`
	TotalRestored      int64 `json:"total_restored"`
	TotalCancelled     int64 `json:"total_cancelled"`
	IsolationsByKind   map[ApplicationKind]int64 `json:"isolations_by_kind"`
}