package db

import (
	"database/sql"
	"errors"

	"github.com/coroot/coroot/model"
	"github.com/coroot/coroot/timeseries"
	"k8s.io/klog"
)

type ServiceIsolationRecord struct {
	ProjectId               ProjectId
	ApplicationId           model.ApplicationId
	DeploymentId            string
	CurrentVersion          string
	PreviousVersion         string
	ConsecutiveSamples      int
	FirstDetectedAt         timeseries.Time
	LastObservedAt          timeseries.Time
	IsolatedAt              timeseries.Time
	NotificationEnqueuedAt  timeseries.Time
	CurrentMemoryGrowthPct  float32
	PreviousMemoryGrowthPct float32
	ThresholdMemoryGrowthPct float32
	IsolationStatus         string
	Details                 *ServiceIsolationDetails
}

type ServiceIsolationDetails struct {
	Services []string                     `json:"services,omitempty"`
	Trigger  string                       `json:"trigger,omitempty"`
	RCA      *IncidentNotificationDetails `json:"rca,omitempty"`
}

type ServiceIsolationStats struct {
	Records       int
	Isolated      int
	Notifications int
}

func (r *ServiceIsolationRecord) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS service_isolation_record (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		deployment_id TEXT NOT NULL,
		current_version TEXT NOT NULL DEFAULT '',
		previous_version TEXT NOT NULL DEFAULT '',
		consecutive_samples INT NOT NULL DEFAULT 0,
		first_detected_at INT NOT NULL DEFAULT 0,
		last_observed_at INT NOT NULL DEFAULT 0,
		isolated_at INT NOT NULL DEFAULT 0,
		notification_enqueued_at INT NOT NULL DEFAULT 0,
		current_memory_growth_pct REAL NOT NULL DEFAULT 0,
		previous_memory_growth_pct REAL NOT NULL DEFAULT 0,
		threshold_memory_growth_pct REAL NOT NULL DEFAULT 0,
		isolation_status TEXT NOT NULL DEFAULT '',
		details TEXT,
		PRIMARY KEY (project_id, application_id, deployment_id)
	);
`)
}

func (db *DB) GetServiceIsolationRecord(projectId ProjectId, applicationId model.ApplicationId, deploymentId string) (*ServiceIsolationRecord, error) {
	row := db.db.QueryRow(`
		SELECT project_id, application_id, deployment_id, current_version, previous_version, consecutive_samples,
			first_detected_at, last_observed_at, isolated_at, notification_enqueued_at,
			current_memory_growth_pct, previous_memory_growth_pct, threshold_memory_growth_pct,
			isolation_status, details
		FROM service_isolation_record
		WHERE project_id = $1 AND application_id = $2 AND deployment_id = $3
	`, projectId, applicationId, deploymentId)
	var r ServiceIsolationRecord
	var details sql.NullString
	if err := row.Scan(
		&r.ProjectId,
		&r.ApplicationId,
		&r.DeploymentId,
		&r.CurrentVersion,
		&r.PreviousVersion,
		&r.ConsecutiveSamples,
		&r.FirstDetectedAt,
		&r.LastObservedAt,
		&r.IsolatedAt,
		&r.NotificationEnqueuedAt,
		&r.CurrentMemoryGrowthPct,
		&r.PreviousMemoryGrowthPct,
		&r.ThresholdMemoryGrowthPct,
		&r.IsolationStatus,
		&details,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	if details.String != "" {
		if err := unmarshal(details.String, &r.Details); err != nil {
			klog.Warningln(err)
		}
	}
	return &r, nil
}

func (db *DB) SaveServiceIsolationRecord(r *ServiceIsolationRecord) error {
	details, err := marshal(r.Details)
	if err != nil {
		return err
	}
	_, err = db.db.Exec(`
		INSERT INTO service_isolation_record (
			project_id, application_id, deployment_id, current_version, previous_version, consecutive_samples,
			first_detected_at, last_observed_at, isolated_at, notification_enqueued_at,
			current_memory_growth_pct, previous_memory_growth_pct, threshold_memory_growth_pct,
			isolation_status, details
		) VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10,
			$11, $12, $13,
			$14, $15
		)
		ON CONFLICT (project_id, application_id, deployment_id) DO UPDATE SET
			current_version = excluded.current_version,
			previous_version = excluded.previous_version,
			consecutive_samples = excluded.consecutive_samples,
			first_detected_at = excluded.first_detected_at,
			last_observed_at = excluded.last_observed_at,
			isolated_at = excluded.isolated_at,
			notification_enqueued_at = excluded.notification_enqueued_at,
			current_memory_growth_pct = excluded.current_memory_growth_pct,
			previous_memory_growth_pct = excluded.previous_memory_growth_pct,
			threshold_memory_growth_pct = excluded.threshold_memory_growth_pct,
			isolation_status = excluded.isolation_status,
			details = excluded.details
	`,
		r.ProjectId,
		r.ApplicationId,
		r.DeploymentId,
		r.CurrentVersion,
		r.PreviousVersion,
		r.ConsecutiveSamples,
		r.FirstDetectedAt,
		r.LastObservedAt,
		r.IsolatedAt,
		r.NotificationEnqueuedAt,
		r.CurrentMemoryGrowthPct,
		r.PreviousMemoryGrowthPct,
		r.ThresholdMemoryGrowthPct,
		r.IsolationStatus,
		details,
	)
	return err
}

func (db *DB) GetServiceIsolationStats() (ServiceIsolationStats, error) {
	row := db.db.QueryRow(`
		SELECT
			COUNT(*),
			COALESCE(SUM(CASE WHEN isolated_at > 0 THEN 1 ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN notification_enqueued_at > 0 THEN 1 ELSE 0 END), 0)
		FROM service_isolation_record
	`)
	var stats ServiceIsolationStats
	if err := row.Scan(&stats.Records, &stats.Isolated, &stats.Notifications); err != nil {
		return ServiceIsolationStats{}, err
	}
	return stats, nil
}
