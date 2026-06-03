package db

type Isolation struct {
	ProjectId     ProjectId
	ApplicationId string
	Timestamp     int64
}

func (i *Isolation) Migrate(m *Migrator) error {
	return m.Exec(`
	CREATE TABLE IF NOT EXISTS isolation (
		project_id TEXT NOT NULL REFERENCES project(id),
		application_id TEXT NOT NULL,
		timestamp INT NOT NULL,
		PRIMARY KEY (project_id, application_id, timestamp)
	)`)
}

func (db *DB) RecordIsolation(projectId ProjectId, appId string, timestamp int64) error {
	_, err := db.db.Exec("INSERT INTO isolation(project_id, application_id, timestamp) VALUES ($1, $2, $3)", projectId, appId, timestamp)
	return err
}

func (db *DB) GetIsolationsCount() (int, error) {
	var count int
	err := db.db.QueryRow("SELECT COUNT(*) FROM isolation").Scan(&count)
	return count, err
}
