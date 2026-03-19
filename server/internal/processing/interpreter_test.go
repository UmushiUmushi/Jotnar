package processing

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

func mockInterpretationServer(interpretation, category, appName string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		result := map[string]string{
			"interpretation": interpretation,
			"category":       category,
			"app_name":       appName,
		}
		content, _ := json.Marshal(result)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": string(content)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
}

func TestInterpret_Success(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)
	ts := mockInterpretationServer("Playing Genshin Impact", "gaming", "Genshin Impact")
	defer ts.Close()

	client := inference.NewOpenAIClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	deviceID := insertTestDevice(t, db)

	interp := NewInterpreter(client, cfg, metaStore)
	capturedAt := time.Date(2024, 6, 15, 10, 30, 0, 0, time.UTC)

	// Minimal valid PNG (magic bytes + minimal data).
	imageData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}

	meta, err := interp.Interpret(imageData, deviceID, capturedAt, "")
	if err != nil {
		t.Fatalf("Interpret: %v", err)
	}
	if meta.Interpretation != "Playing Genshin Impact" {
		t.Errorf("interpretation = %q", meta.Interpretation)
	}
	if meta.Category != "gaming" {
		t.Errorf("category = %q", meta.Category)
	}
	if meta.AppName != "Genshin Impact" {
		t.Errorf("app_name = %q", meta.AppName)
	}
	if meta.DeviceID != deviceID {
		t.Errorf("device_id = %q, want %q", meta.DeviceID, deviceID)
	}
	if !meta.CapturedAt.Equal(capturedAt) {
		t.Errorf("captured_at = %v, want %v", meta.CapturedAt, capturedAt)
	}

	// Verify it was persisted.
	rows, _ := metaStore.GetUnconsolidated()
	if len(rows) != 1 {
		t.Fatalf("stored metadata = %d, want 1", len(rows))
	}
	if rows[0].ID != meta.ID {
		t.Error("stored metadata ID should match returned metadata")
	}
}

func TestInterpret_InferenceError(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("model error"))
	}))
	defer ts.Close()

	client := inference.NewOpenAIClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	deviceID := insertTestDevice(t, db)

	interp := NewInterpreter(client, cfg, metaStore)
	_, err := interp.Interpret([]byte{0x89, 0x50, 0x4E, 0x47}, deviceID, time.Now(), "")
	if err == nil {
		t.Fatal("expected error from inference failure, got nil")
	}
	if !strings.Contains(err.Error(), "inference") {
		t.Errorf("error = %q, should mention inference", err.Error())
	}
}

func TestInterpret_ParseError(t *testing.T) {
	db := testDB(t)
	cfg := testConfig(t)

	// Return non-JSON from inference.
	ts := mockInferenceServer("this is not JSON")
	defer ts.Close()

	client := inference.NewOpenAIClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	deviceID := insertTestDevice(t, db)

	interp := NewInterpreter(client, cfg, metaStore)
	_, err := interp.Interpret([]byte{0x89, 0x50, 0x4E, 0x47}, deviceID, time.Now(), "")
	if err == nil {
		t.Fatal("expected error from parse failure, got nil")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("error = %q, should mention parse", err.Error())
	}
}

func TestInterpret_UsesConfigDetail(t *testing.T) {
	db := testDB(t)
	cfgMgr, _ := config.NewManager(filepath.Join(t.TempDir(), "config.yml"))

	// Set detailed level.
	cfg := cfgMgr.Get()
	cfg.InterpretationDetail = "detailed"
	cfgMgr.Update(cfg)

	var receivedBody []byte
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Capture the request to verify prompt content.
		buf := make([]byte, 4096)
		n, _ := r.Body.Read(buf)
		receivedBody = buf[:n]

		result := map[string]string{
			"interpretation": "test",
			"category":       "other",
			"app_name":       "Test",
		}
		content, _ := json.Marshal(result)
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": string(content)}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer ts.Close()

	client := inference.NewOpenAIClient(inference.ClientConfig{Host: ts.URL, MaxRetries: 1})
	metaStore := store.NewMetadataStore(db)
	deviceID := insertTestDevice(t, db)

	interp := NewInterpreter(client, cfgMgr, metaStore)
	interp.Interpret([]byte{0x89, 0x50, 0x4E, 0x47}, deviceID, time.Now(), "")

	// Verify the request contained the detailed prompt keywords.
	if !strings.Contains(string(receivedBody), "key topics") {
		t.Error("detailed config should produce prompt containing 'key topics'")
	}
}
