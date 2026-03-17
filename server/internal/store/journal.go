// CRUD for journal entries.
package store

import (
	"database/sql"
	"fmt"
	"time"
)

type JournalStore struct {
	DB *sql.DB
}

func NewJournalStore(database *sql.DB) *JournalStore {
	return &JournalStore{DB: database}
}

func (s *JournalStore) Create(entry JournalEntry) error {
	_, err := s.DB.Exec(
		`INSERT INTO journal_entries (id, narrative, time_start, time_end, edited, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.ID, entry.Narrative, entry.TimeStart, entry.TimeEnd, entry.Edited, entry.CreatedAt, entry.UpdatedAt,
	)
	return err
}

func (s *JournalStore) GetByID(id string) (*JournalEntry, error) {
	var e JournalEntry
	err := s.DB.QueryRow(
		`SELECT id, narrative, time_start, time_end, edited, created_at, updated_at
		 FROM journal_entries WHERE id = ?`, id,
	).Scan(&e.ID, &e.Narrative, &e.TimeStart, &e.TimeEnd, &e.Edited, &e.CreatedAt, &e.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("journal entry not found")
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

func (s *JournalStore) List() ([]JournalEntry, error) {
	rows, err := s.DB.Query(
		`SELECT id, narrative, time_start, time_end, edited, created_at, updated_at
		 FROM journal_entries ORDER BY time_start DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []JournalEntry
	for rows.Next() {
		var e JournalEntry
		if err := rows.Scan(&e.ID, &e.Narrative, &e.TimeStart, &e.TimeEnd, &e.Edited, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *JournalStore) Count() (int, error) {
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM journal_entries").Scan(&count)
	return count, err
}

func (s *JournalStore) ListPaginated(limit, offset int) ([]JournalEntry, error) {
	rows, err := s.DB.Query(
		`SELECT id, narrative, time_start, time_end, edited, created_at, updated_at
		 FROM journal_entries ORDER BY time_start DESC LIMIT ? OFFSET ?`,
		limit, offset,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []JournalEntry
	for rows.Next() {
		var e JournalEntry
		if err := rows.Scan(&e.ID, &e.Narrative, &e.TimeStart, &e.TimeEnd, &e.Edited, &e.CreatedAt, &e.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, nil
}

func (s *JournalStore) UpdateNarrative(id, narrative string, edited bool) error {
	now := time.Now().UTC()
	result, err := s.DB.Exec(
		"UPDATE journal_entries SET narrative = ?, edited = ?, updated_at = ? WHERE id = ?",
		narrative, edited, now, id,
	)
	if err != nil {
		return err
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("journal entry not found")
	}
	return nil
}

func (s *JournalStore) UpdateTimeRange(id string, timeStart, timeEnd time.Time) error {
	_, err := s.DB.Exec(
		"UPDATE journal_entries SET time_start = ?, time_end = ?, updated_at = ? WHERE id = ?",
		timeStart, timeEnd, time.Now().UTC(), id,
	)
	return err
}

func (s *JournalStore) Delete(id string) error {
	tx, err := s.DB.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Unlink metadata first (ON DELETE SET NULL handles this via FK, but be explicit)
	if _, err := tx.Exec("UPDATE metadata SET entry_id = NULL WHERE entry_id = ?", id); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM journal_entries WHERE id = ?", id); err != nil {
		return err
	}
	return tx.Commit()
}
