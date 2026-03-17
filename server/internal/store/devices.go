// CRUD for paired devices.
package store

import (
	"database/sql"
)

type DeviceStore struct {
	DB *sql.DB
}

func NewDeviceStore(database *sql.DB) *DeviceStore {
	return &DeviceStore{DB: database}
}

func (s *DeviceStore) Count() (int, error) {
	var count int
	err := s.DB.QueryRow("SELECT COUNT(*) FROM devices").Scan(&count)
	return count, err
}

func (s *DeviceStore) List() ([]Device, error) {
	rows, err := s.DB.Query("SELECT id, name, paired_at, token_hash, last_seen FROM devices ORDER BY paired_at")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var devices []Device
	for rows.Next() {
		var d Device
		if err := rows.Scan(&d.ID, &d.Name, &d.PairedAt, &d.TokenHash, &d.LastSeen); err != nil {
			return nil, err
		}
		devices = append(devices, d)
	}
	return devices, nil
}

func (s *DeviceStore) Delete(id string) error {
	_, err := s.DB.Exec("DELETE FROM devices WHERE id = ?", id)
	return err
}
