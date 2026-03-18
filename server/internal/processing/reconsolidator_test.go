package processing

import (
	"sync/atomic"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

func insertJournalEntry(t *testing.T, journalStore *store.JournalStore) store.JournalEntry {
	t.Helper()
	entry := store.JournalEntry{
		ID:        uuid.New().String(),
		Narrative: "Original narrative",
		TimeStart: time.Now().UTC().Add(-30 * time.Minute),
		TimeEnd:   time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
	if err := journalStore.Create(entry); err != nil {
		t.Fatalf("insert journal entry: %v", err)
	}
	return entry
}

func TestPreview_Success(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("Preview narrative text.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	entry := insertJournalEntry(t, journalStore)
	entryID := entry.ID

	now := time.Now().UTC()
	m1 := insertMetadata(t, metaStore, deviceID, now.Add(-20*time.Minute), &entryID)
	m2 := insertMetadata(t, metaStore, deviceID, now.Add(-10*time.Minute), &entryID)

	narrative, err := recon.Preview([]string{m1.ID, m2.ID})
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if narrative != "Preview narrative text." {
		t.Errorf("narrative = %q", narrative)
	}

	// Verify nothing was modified in DB.
	got, _ := journalStore.GetByID(entry.ID)
	if got.Narrative != "Original narrative" {
		t.Error("Preview should not modify the journal entry")
	}
}

func TestPreview_EmptyIDs(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("should not be called")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	_, err := recon.Preview([]string{})
	if err == nil {
		t.Fatal("expected error for empty IDs, got nil")
	}
}

func TestCommit_DeletesExcluded(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("Updated narrative.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	entry := insertJournalEntry(t, journalStore)
	entryID := entry.ID

	now := time.Now().UTC()
	m1 := insertMetadata(t, metaStore, deviceID, now.Add(-20*time.Minute), &entryID)
	m2 := insertMetadata(t, metaStore, deviceID, now.Add(-10*time.Minute), &entryID)
	m3 := insertMetadata(t, metaStore, deviceID, now.Add(-5*time.Minute), &entryID)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	// Include only m1 and m3, exclude m2.
	result, err := recon.Commit(entry.ID, []string{m1.ID, m3.ID}, "")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	// m2 should be deleted.
	remaining, _ := metaStore.GetByEntryID(entry.ID)
	for _, m := range remaining {
		if m.ID == m2.ID {
			t.Error("excluded metadata m2 should have been deleted")
		}
	}

	// Entry should be updated.
	if !result.Edited {
		t.Error("entry should be marked as edited")
	}
}

func TestCommit_UpdatesTimeRange(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInferenceServer("Narrative.")
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	entry := insertJournalEntry(t, journalStore)
	entryID := entry.ID

	t1 := time.Date(2024, 6, 15, 10, 0, 0, 0, time.UTC)
	t2 := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)
	m1 := insertMetadata(t, metaStore, deviceID, t1, &entryID)
	m2 := insertMetadata(t, metaStore, deviceID, t2, &entryID)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	result, err := recon.Commit(entry.ID, []string{m1.ID, m2.ID}, "Custom narrative")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}

	if !result.TimeStart.Equal(t1) {
		t.Errorf("time_start = %v, want %v", result.TimeStart, t1)
	}
	if !result.TimeEnd.Equal(t2) {
		t.Errorf("time_end = %v, want %v", result.TimeEnd, t2)
	}
}

func TestCommit_WithProvidedNarrative(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)

	var inferenceCalled atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
			return
		}
		inferenceCalled.Add(1)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "should not appear"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	entry := insertJournalEntry(t, journalStore)
	entryID := entry.ID

	m1 := insertMetadata(t, metaStore, deviceID, time.Now().UTC(), &entryID)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	result, err := recon.Commit(entry.ID, []string{m1.ID}, "User-provided narrative")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.Narrative != "User-provided narrative" {
		t.Errorf("narrative = %q, want %q", result.Narrative, "User-provided narrative")
	}
	if inferenceCalled.Load() != 0 {
		t.Error("inference should not be called when narrative is provided")
	}
}

func TestCommit_WithoutNarrative_CallsInference(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)

	var inferenceCalled atomic.Int32
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
			return
		}
		inferenceCalled.Add(1)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": "Generated narrative"}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := inference.NewClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	journalStore := store.NewJournalStore(db)
	deviceID := insertTestDevice(t, db)

	entry := insertJournalEntry(t, journalStore)
	entryID := entry.ID

	m1 := insertMetadata(t, metaStore, deviceID, time.Now().UTC(), &entryID)

	consolidator := NewConsolidator(client, cfg, metaStore, journalStore)
	recon := NewReconsolidator(consolidator, cfg, metaStore, journalStore)

	result, err := recon.Commit(entry.ID, []string{m1.ID}, "")
	if err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if result.Narrative != "Generated narrative" {
		t.Errorf("narrative = %q, want %q", result.Narrative, "Generated narrative")
	}
	if inferenceCalled.Load() != 1 {
		t.Errorf("inference calls = %d, want 1", inferenceCalled.Load())
	}
}
