// CRUD for pending capture jobs that survive container restarts.
package store

import "database/sql"

type PendingStore struct {
	DB *sql.DB
}

func NewPendingStore(database *sql.DB) *PendingStore {
	return &PendingStore{DB: database}
}

// SaveAll inserts pending captures in a single transaction.
func (s *PendingStore) SaveAll(jobs []PendingCapture) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT OR REPLACE INTO pending_captures (id, device_id, image_data, captured_at, app_name, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, j := range jobs {
		if _, err := stmt.Exec(j.ID, j.DeviceID, j.ImageData, j.CapturedAt.UTC(), j.AppName, j.CreatedAt.UTC()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// LoadAll retrieves all pending captures and deletes them in one transaction.
func (s *PendingStore) LoadAll() ([]PendingCapture, error) {
	rows, err := s.DB.Query(
		`SELECT id, device_id, image_data, captured_at, COALESCE(app_name, ''), created_at
		 FROM pending_captures ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []PendingCapture
	for rows.Next() {
		var j PendingCapture
		if err := rows.Scan(&j.ID, &j.DeviceID, &j.ImageData, &j.CapturedAt, &j.AppName, &j.CreatedAt); err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}

	if _, err := s.DB.Exec(`DELETE FROM pending_captures`); err != nil {
		return nil, err
	}

	return jobs, nil
}
