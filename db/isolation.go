package db

import (
	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type IsolationRecord struct {
	ProjectId     ProjectId
	ApplicationId model.ApplicationId
	State         model.IsolationState
	OpenedAt      timeseries.Time
	ResolvedAt    timeseries.Time
	Deployment    string
	Reason        string
	RCASummary    string
	MemoryLeakPct float32
	PrevLeakPct   float32
	Consecutive   int
}

func (r *IsolationRecord) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS isolation_record (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		state TEXT NOT NULL DEFAULT 'pending',
		opened_at INT NOT NULL,
		resolved_at INT NOT NULL DEFAULT 0,
		deployment TEXT NOT NULL DEFAULT '',
		reason TEXT NOT NULL DEFAULT '',
		rca_summary TEXT NOT NULL DEFAULT '',
		memory_leak_pct REAL NOT NULL DEFAULT 0,
		prev_leak_pct REAL NOT NULL DEFAULT 0,
		consecutive INT NOT NULL DEFAULT 0,
		PRIMARY KEY (project_id, application_id, opened_at)
	);
	CREATE INDEX IF NOT EXISTS isolation_record_state ON isolation_record (project_id, state);
`)
}

func (db *DB) CreateIsolationRecord(projectId ProjectId, r *model.IsolationRecord) error {
	_, err := db.Exec(
		`INSERT INTO isolation_record (project_id, application_id, state, opened_at, deployment, reason, rca_summary, memory_leak_pct, prev_leak_pct, consecutive)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
		projectId, r.ApplicationId, r.State, r.OpenedAt, r.Deployment, r.Reason, r.RCASummary, r.MemoryLeakPct, r.PrevLeakPct, r.Consecutive,
	)
	if err != nil {
		klog.Errorln("failed to create isolation record:", err)
	}
	return err
}

func (db *DB) UpdateIsolationRecord(projectId ProjectId, r *model.IsolationRecord) error {
	_, err := db.Exec(
		`UPDATE isolation_record SET state = $1, resolved_at = $2, rca_summary = $3, memory_leak_pct = $4, consecutive = $5
		 WHERE project_id = $6 AND application_id = $7 AND opened_at = $8`,
		r.State, r.ResolvedAt, r.RCASummary, r.MemoryLeakPct, r.Consecutive, projectId, r.ApplicationId, r.OpenedAt,
	)
	if err != nil {
		klog.Errorln("failed to update isolation record:", err)
	}
	return err
}

func (db *DB) GetOpenIsolationRecord(projectId ProjectId, appId model.ApplicationId) (*model.IsolationRecord, error) {
	var r model.IsolationRecord
	var state string
	err := db.QueryRow(
		`SELECT application_id, state, opened_at, resolved_at, deployment, reason, rca_summary, memory_leak_pct, prev_leak_pct, consecutive
		 FROM isolation_record WHERE project_id = $1 AND application_id = $2 AND state IN ('pending', 'isolated') ORDER BY opened_at DESC LIMIT 1`,
		projectId, appId,
	).Scan(&r.ApplicationId, &state, &r.OpenedAt, &r.ResolvedAt, &r.Deployment, &r.Reason, &r.RCASummary, &r.MemoryLeakPct, &r.PrevLeakPct, &r.Consecutive)
	if err != nil {
		return nil, err
	}
	r.State = model.IsolationState(state)
	if r.ApplicationId.ClusterId == "" {
		r.ApplicationId.ClusterId = string(projectId)
	}
	return &r, nil
}

func (db *DB) GetIsolationRecords(projectId ProjectId) ([]*model.IsolationRecord, error) {
	rows, err := db.Query(
		`SELECT application_id, state, opened_at, resolved_at, deployment, reason, rca_summary, memory_leak_pct, prev_leak_pct, consecutive
		 FROM isolation_record WHERE project_id = $1 ORDER BY opened_at DESC`,
		projectId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []*model.IsolationRecord
	for rows.Next() {
		var r model.IsolationRecord
		var state string
		if err := rows.Scan(&r.ApplicationId, &state, &r.OpenedAt, &r.ResolvedAt, &r.Deployment, &r.Reason, &r.RCASummary, &r.MemoryLeakPct, &r.PrevLeakPct, &r.Consecutive); err != nil {
			return nil, err
		}
		r.State = model.IsolationState(state)
		if r.ApplicationId.ClusterId == "" {
			r.ApplicationId.ClusterId = string(projectId)
		}
		res = append(res, &r)
	}
	return res, nil
}

func (db *DB) GetActiveIsolationCount(projectId ProjectId) (int64, error) {
	var count int64
	err := db.QueryRow(
		`SELECT COUNT(*) FROM isolation_record WHERE project_id = $1 AND state IN ('pending', 'isolated')`,
		projectId,
	).Scan(&count)
	return count, err
}

func (db *DB) GetIsolationStats(projectId ProjectId) (*model.IsolationStats, error) {
	stats := &model.IsolationStats{
		IsolationsByKind: map[model.ApplicationKind]int64{},
	}

	rows, err := db.Query(
		`SELECT state, COUNT(*) as cnt FROM isolation_record WHERE project_id = $1 GROUP BY state`,
		projectId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var state string
		var cnt int64
		if err := rows.Scan(&state, &cnt); err != nil {
			return nil, err
		}
		stats.TotalIsolations += cnt
		switch model.IsolationState(state) {
		case model.IsolationStatePending, model.IsolationStateIsolated:
			stats.ActiveIsolations += cnt
		case model.IsolationStateRestored:
			stats.TotalRestored += cnt
		case model.IsolationStateCancelled:
			stats.TotalCancelled += cnt
		}
	}

	appRows, appErr := db.Query(
		`SELECT i.application_id, COUNT(*) FROM isolation_record i
		 INNER JOIN incident inc ON inc.project_id = i.project_id AND inc.application_id = i.application_id
		 WHERE i.project_id = $1 GROUP BY i.application_id`,
		projectId,
	)
	if appErr == nil {
		defer appRows.Close()
		for appRows.Next() {
			var appId model.ApplicationId
			var cnt int64
			if err := appRows.Scan(&appId, &cnt); err != nil {
				continue
			}
			stats.IsolationsByKind[appId.Kind] += cnt
		}
	}

	return stats, nil
}