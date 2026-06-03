package db

import (
	"database/sql"
	"encoding/json"
	"errors"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type MemoryIsolationRecord model.MemoryIsolationRecord

func (r *MemoryIsolationRecord) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS memory_isolation_record (
		id TEXT NOT NULL,
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		deployment_id TEXT NOT NULL,
		started_at INT NOT NULL,
		resolved_at INT NOT NULL DEFAULT 0,
		growth_percent REAL NOT NULL,
		reason TEXT NOT NULL,
		rca TEXT,
		PRIMARY KEY (id, project_id)
	);
	CREATE INDEX IF NOT EXISTS memory_isolation_record_project_id_resolved ON memory_isolation_record (project_id, (resolved_at = 0));
	CREATE INDEX IF NOT EXISTS memory_isolation_record_project_id_application_id ON memory_isolation_record (project_id, application_id);
	`)
}

func (db *DB) CreateMemoryIsolationRecord(projectId ProjectId, r *model.MemoryIsolationRecord) error {
	rca, err := json.Marshal(r.Rca)
	if err != nil {
		rca = nil
	}
	_, err = db.db.Exec(
		"INSERT INTO memory_isolation_record (id, project_id, application_id, deployment_id, started_at, resolved_at, growth_percent, reason, rca) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)",
		r.Id, projectId, r.ApplicationId.String(), r.DeploymentId, r.StartedAt, r.ResolvedAt, r.GrowthPercent, r.Reason, string(rca),
	)
	return err
}

func (db *DB) UpdateMemoryIsolationRecord(projectId ProjectId, r *model.MemoryIsolationRecord) error {
	rca, err := json.Marshal(r.Rca)
	if err != nil {
		rca = nil
	}
	_, err = db.db.Exec(
		"UPDATE memory_isolation_record SET resolved_at = $1, rca = $2 WHERE id = $3 AND project_id = $4",
		r.ResolvedAt, string(rca), r.Id, projectId,
	)
	return err
}

func (db *DB) GetActiveMemoryIsolations(projectId ProjectId) ([]*model.MemoryIsolationRecord, error) {
	rows, err := db.db.Query(
		"SELECT id, application_id, deployment_id, started_at, resolved_at, growth_percent, reason, rca FROM memory_isolation_record WHERE project_id = $1 AND resolved_at = 0",
		projectId,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryIsolationRecords(rows, projectId)
}

func (db *DB) GetMemoryIsolationRecords(projectId ProjectId, from, to timeseries.Time) ([]*model.MemoryIsolationRecord, error) {
	rows, err := db.db.Query(
		"SELECT id, application_id, deployment_id, started_at, resolved_at, growth_percent, reason, rca FROM memory_isolation_record WHERE project_id = $1 AND started_at <= $2 AND (resolved_at = 0 OR resolved_at >= $3) ORDER BY started_at DESC",
		projectId, to, from,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMemoryIsolationRecords(rows, projectId)
}

func (db *DB) GetMemoryIsolationStats(projectId ProjectId, from, to timeseries.Time) (int, error) {
	var count int
	err := db.db.QueryRow(
		"SELECT COUNT(*) FROM memory_isolation_record WHERE project_id = $1 AND started_at >= $2 AND started_at <= $3",
		projectId, from, to,
	).Scan(&count)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return count, err
}

func scanMemoryIsolationRecords(rows *sql.Rows, projectId ProjectId) ([]*model.MemoryIsolationRecord, error) {
	var res []*model.MemoryIsolationRecord
	for rows.Next() {
		var r model.MemoryIsolationRecord
		var appIdStr string
		var rcaStr sql.NullString
		if err := rows.Scan(&r.Id, &appIdStr, &r.DeploymentId, &r.StartedAt, &r.ResolvedAt, &r.GrowthPercent, &r.Reason, &rcaStr); err != nil {
			return nil, err
		}
		appId, err := model.NewApplicationIdFromString(appIdStr, string(projectId))
		if err != nil {
			klog.Warningln(err)
			continue
		}
		r.ApplicationId = appId
		if rcaStr.Valid {
			var rca model.RcaSummary
			if err := json.Unmarshal([]byte(rcaStr.String), &rca); err == nil {
				r.Rca = &rca
			}
		}
		res = append(res, &r)
	}
	return res, rows.Err()
}
