// Table structs and queries.
package store

import "time"

type Device struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	PairedAt  time.Time  `json:"paired_at"`
	TokenHash string     `json:"-"`
	LastSeen  *time.Time `json:"last_seen"`
}

type Metadata struct {
	ID             string    `json:"id"`
	DeviceID       string    `json:"device_id"`
	CapturedAt     time.Time `json:"captured_at"`
	Interpretation string    `json:"interpretation"`
	Category       string    `json:"category"`
	AppName        string    `json:"app_name"`
	EntryID        *string   `json:"entry_id"`
	CreatedAt      time.Time `json:"created_at"`
}

type JournalEntry struct {
	ID        string     `json:"id"`
	Narrative string     `json:"narrative"`
	TimeStart time.Time  `json:"time_start"`
	TimeEnd   time.Time  `json:"time_end"`
	Edited    bool       `json:"edited"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at"`
}

type Recovery struct {
	ID      int    `json:"id"`
	KeyHash string `json:"-"`
}
