// CRUD for metadata rows.
package store

import (
	"database/sql"
	"time"
)

type MetadataStore struct {
	DB *sql.DB
}

func NewMetadataStore(database *sql.DB) *MetadataStore {
	return &MetadataStore{DB: database}
}

func (s *MetadataStore) Create(m Metadata) error {
	_, err := s.DB.Exec(
		`INSERT INTO metadata (id, device_id, captured_at, interpretation, category, app_name, entry_id, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		m.ID, m.DeviceID, m.CapturedAt, m.Interpretation, m.Category, m.AppName, m.EntryID, m.CreatedAt,
	)
	return err
}

// GetUnconsolidated returns metadata rows not yet linked to a journal entry,
// ordered by captured_at ascending.
func (s *MetadataStore) GetUnconsolidated() ([]Metadata, error) {
	return s.query("SELECT id, device_id, captured_at, interpretation, category, app_name, entry_id, created_at FROM metadata WHERE entry_id IS NULL ORDER BY captured_at ASC")
}

// GetByEntryID returns metadata linked to a specific journal entry.
func (s *MetadataStore) GetByEntryID(entryID string) ([]Metadata, error) {
	return s.query("SELECT id, device_id, captured_at, interpretation, category, app_name, entry_id, created_at FROM metadata WHERE entry_id = ? ORDER BY captured_at ASC", entryID)
}

// GetByIDs returns metadata rows matching the given IDs.
func (s *MetadataStore) GetByIDs(ids []string) ([]Metadata, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	query := "SELECT id, device_id, captured_at, interpretation, category, app_name, entry_id, created_at FROM metadata WHERE id IN (?" + repeatParam(len(ids)-1) + ") ORDER BY captured_at ASC"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	return s.query(query, args...)
}

// LinkToEntry sets the entry_id on the given metadata rows.
func (s *MetadataStore) LinkToEntry(metadataIDs []string, entryID string) error {
	if len(metadataIDs) == 0 {
		return nil
	}
	query := "UPDATE metadata SET entry_id = ? WHERE id IN (?" + repeatParam(len(metadataIDs)-1) + ")"
	args := []any{entryID}
	for _, id := range metadataIDs {
		args = append(args, id)
	}
	_, err := s.DB.Exec(query, args...)
	return err
}

// DeleteByIDs deletes metadata rows by their IDs.
func (s *MetadataStore) DeleteByIDs(ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	query := "DELETE FROM metadata WHERE id IN (?" + repeatParam(len(ids)-1) + ")"
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err := s.DB.Exec(query, args...)
	return err
}

// DeleteOlderThan removes metadata older than the cutoff.
func (s *MetadataStore) DeleteOlderThan(cutoff time.Time) (int64, error) {
	result, err := s.DB.Exec("DELETE FROM metadata WHERE created_at < ?", cutoff)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

func (s *MetadataStore) query(q string, args ...any) ([]Metadata, error) {
	rows, err := s.DB.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []Metadata
	for rows.Next() {
		var m Metadata
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.CapturedAt, &m.Interpretation, &m.Category, &m.AppName, &m.EntryID, &m.CreatedAt); err != nil {
			return nil, err
		}
		results = append(results, m)
	}
	return results, nil
}

func repeatParam(n int) string {
	s := ""
	for i := 0; i < n; i++ {
		s += ", ?"
	}
	return s
}
