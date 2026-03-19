package processing

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

func testDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.InitializeSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func testConfig(t *testing.T) *config.Manager {
	t.Helper()
	mgr, err := config.NewManager(filepath.Join(t.TempDir(), "config.yml"))
	if err != nil {
		t.Fatalf("create config manager: %v", err)
	}
	return mgr
}

func mockInferenceServer(response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
			return
		}
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": response}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func insertTestDevice(t *testing.T, db *sql.DB) string {
	t.Helper()
	id := uuid.New().String()
	_, err := db.Exec(
		"INSERT INTO devices (id, name, paired_at, token_hash, last_seen) VALUES (?, ?, ?, ?, ?)",
		id, "Test Device", time.Now().UTC(), "hash", time.Now().UTC(),
	)
	if err != nil {
		t.Fatalf("insert test device: %v", err)
	}
	return id
}

func insertMetadata(t *testing.T, metaStore *store.MetadataStore, deviceID string, capturedAt time.Time, entryID *string) store.Metadata {
	t.Helper()
	m := store.Metadata{
		ID:             uuid.New().String(),
		DeviceID:       deviceID,
		CapturedAt:     capturedAt,
		Interpretation: "Test interpretation",
		Category:       "browsing",
		AppName:        "TestApp",
		EntryID:        entryID,
		CreatedAt:      time.Now().UTC(),
	}
	if err := metaStore.Create(m); err != nil {
		t.Fatalf("insert metadata: %v", err)
	}
	return m
}

// --- groupByWindow tests ---

func TestGroupByWindow_Empty(t *testing.T) {
	result := groupByWindow(nil, 30*time.Minute)
	if result != nil {
		t.Errorf("expected nil, got %d batches", len(result))
	}
}

func TestGroupByWindow_SingleBatch(t *testing.T) {
	now := time.Now()
	rows := []store.Metadata{
		{CapturedAt: now},
		{CapturedAt: now.Add(5 * time.Minute)},
		{CapturedAt: now.Add(10 * time.Minute)},
	}
	batches := groupByWindow(rows, 30*time.Minute)
	if len(batches) != 1 {
		t.Errorf("batches = %d, want 1", len(batches))
	}
	if len(batches[0]) != 3 {
		t.Errorf("batch[0] size = %d, want 3", len(batches[0]))
	}
}

func TestGroupByWindow_MultipleBatches(t *testing.T) {
	now := time.Now()
	rows := []store.Metadata{
		{CapturedAt: now},
		{CapturedAt: now.Add(10 * time.Minute)},
		{CapturedAt: now.Add(40 * time.Minute)}, // > 30min from first, starts new batch
		{CapturedAt: now.Add(50 * time.Minute)},
	}
	batches := groupByWindow(rows, 30*time.Minute)
	if len(batches) != 2 {
		t.Errorf("batches = %d, want 2", len(batches))
	}
	if len(batches[0]) != 2 {
		t.Errorf("batch[0] size = %d, want 2", len(batches[0]))
	}
	if len(batches[1]) != 2 {
		t.Errorf("batch[1] size = %d, want 2", len(batches[1]))
	}
}

func TestGroupByWindow_ExactBoundary(t *testing.T) {
	now := time.Now()
	rows := []store.Metadata{
		{CapturedAt: now},
		// Exactly at window boundary — should stay in same batch (uses >)
		{CapturedAt: now.Add(30 * time.Minute)},
		// Just past boundary from first row — but window resets, so this is relative to batch start
		{CapturedAt: now.Add(31 * time.Minute)},
	}
	batches := groupByWindow(rows, 30*time.Minute)
	// Row at 30min: 30-0 = 30min, not > 30min, so stays in batch 1.
	// Row at 31min: 31-0 = 31min, > 30min, starts batch 2.
	if len(batches) != 2 {
		t.Errorf("batches = %d, want 2", len(batches))
	}
}

// --- formatMetadataForPrompt tests ---

func TestFormatMetadataForPrompt(t *testing.T) {
	rows := []store.Metadata{
		{
			CapturedAt:     time.Date(2024, 1, 15, 14, 30, 0, 0, time.UTC),
			AppName:        "Discord",
			Category:       "communication",
			Interpretation: "Chatting with friends",
		},
		{
			CapturedAt:     time.Date(2024, 1, 15, 14, 35, 0, 0, time.UTC),
			AppName:        "Reddit",
			Category:       "browsing",
			Interpretation: "Reading r/golang",
		},
	}
	result := formatMetadataForPrompt(rows, time.UTC)
	if !strings.Contains(result, "14:30:00") {
		t.Error("should contain timestamp 14:30:00")
	}
	if !strings.Contains(result, "Discord") {
		t.Error("should contain app name Discord")
	}
	if !strings.Contains(result, "communication") {
		t.Error("should contain category")
	}
	if !strings.Contains(result, "Chatting with friends") {
		t.Error("should contain interpretation")
	}
	if !strings.Contains(result, "Reddit") {
		t.Error("should contain second app name")
	}
}

// --- Consolidator.Run tests ---

func TestConsolidator_Run_CreatesEntry(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("Spent some time browsing the web.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	now := time.Now().UTC()
	m1 := insertMetadata(t, metaStore, deviceID, now, nil)
	m2 := insertMetadata(t, metaStore, deviceID, now.Add(5*time.Minute), nil)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	if err := consolidator.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// Verify journal entry was created.
	entries, err := journalStore.List()
	if err != nil {
		t.Fatalf("list entries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(entries))
	}
	if entries[0].Narrative != "Spent some time browsing the web." {
		t.Errorf("narrative = %q", entries[0].Narrative)
	}

	// Verify metadata is linked to the entry.
	linked, _ := metaStore.GetByEntryID(entries[0].ID)
	if len(linked) != 2 {
		t.Errorf("linked metadata = %d, want 2", len(linked))
	}

	// Verify the linked metadata IDs match.
	linkedIDs := map[string]bool{}
	for _, m := range linked {
		linkedIDs[m.ID] = true
	}
	if !linkedIDs[m1.ID] || !linkedIDs[m2.ID] {
		t.Error("linked metadata IDs should include both inserted rows")
	}
}

func TestConsolidator_Run_NoData(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("should not be called")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	if err := consolidator.Run(); err != nil {
		t.Fatalf("Run with no data: %v", err)
	}

	count, _ := journalStore.Count()
	if count != 0 {
		t.Errorf("entries = %d, want 0", count)
	}
}

func TestConsolidator_Run_MultipleWindows(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("A journal entry.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	now := time.Now().UTC()
	// Two metadata in first window, one in second window (>30 min gap).
	insertMetadata(t, metaStore, deviceID, now, nil)
	insertMetadata(t, metaStore, deviceID, now.Add(10*time.Minute), nil)
	insertMetadata(t, metaStore, deviceID, now.Add(60*time.Minute), nil)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	if err := consolidator.Run(); err != nil {
		t.Fatalf("Run: %v", err)
	}

	count, _ := journalStore.Count()
	if count != 2 {
		t.Errorf("entries = %d, want 2", count)
	}
}

func TestConsolidator_SoftConsolidate(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("Preview narrative only.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)

	rows := []store.Metadata{
		{CapturedAt: time.Now(), AppName: "TestApp", Category: "browsing", Interpretation: "test"},
	}
	narrative, err := consolidator.SoftConsolidate(rows, "casual")
	if err != nil {
		t.Fatalf("SoftConsolidate: %v", err)
	}
	if narrative != "Preview narrative only." {
		t.Errorf("narrative = %q", narrative)
	}

	// Verify nothing was saved.
	count, _ := journalStore.Count()
	if count != 0 {
		t.Errorf("entries = %d, want 0 (SoftConsolidate should not save)", count)
	}
}
