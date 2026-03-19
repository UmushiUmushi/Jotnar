CREATE TABLE IF NOT EXISTS devices (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    paired_at  DATETIME NOT NULL,
    token_hash TEXT NOT NULL,
    last_seen  DATETIME
);

CREATE TABLE IF NOT EXISTS journal_entries (
    id         TEXT PRIMARY KEY,
    narrative  TEXT NOT NULL,
    time_start DATETIME NOT NULL,
    time_end   DATETIME NOT NULL,
    edited     BOOLEAN DEFAULT FALSE,
    created_at DATETIME NOT NULL,
    updated_at DATETIME
);

CREATE TABLE IF NOT EXISTS metadata (
    id              TEXT PRIMARY KEY,
    device_id       TEXT NOT NULL,
    captured_at     DATETIME NOT NULL,
    interpretation  TEXT NOT NULL,
    category        TEXT,
    app_name        TEXT,
    entry_id        TEXT,
    created_at      DATETIME NOT NULL,
    FOREIGN KEY (device_id) REFERENCES devices(id),
    FOREIGN KEY (entry_id) REFERENCES journal_entries(id) ON DELETE SET NULL
);

CREATE TABLE IF NOT EXISTS recovery (
    id       INTEGER PRIMARY KEY CHECK (id = 1),
    key_hash TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS pairing_codes (
    code       TEXT PRIMARY KEY,
    expires_at DATETIME NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_metadata_entry_id ON metadata(entry_id);
CREATE INDEX IF NOT EXISTS idx_metadata_captured_at ON metadata(captured_at);
CREATE INDEX IF NOT EXISTS idx_metadata_device_id ON metadata(device_id);
CREATE INDEX IF NOT EXISTS idx_metadata_unconsolidated ON metadata(captured_at) WHERE entry_id IS NULL;

CREATE TABLE IF NOT EXISTS pending_captures (
    id          TEXT PRIMARY KEY,
    device_id   TEXT NOT NULL,
    image_data  BLOB NOT NULL,
    captured_at DATETIME NOT NULL,
    app_name    TEXT DEFAULT '',
    created_at  DATETIME NOT NULL
);
