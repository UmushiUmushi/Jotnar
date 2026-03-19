package api

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/jotnar/server/internal/auth"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/processing"
	"github.com/jotnar/server/internal/store"
)

type testEnv struct {
	server       *Server
	db           *sql.DB
	token        string
	deviceID     string
	journalStore *store.JournalStore
	metadataStore *store.MetadataStore
	pairingService *auth.PairingService
	recoveryService *auth.RecoveryService
}

func setupTestEnv(t *testing.T) *testEnv {
	t.Helper()

	db, err := sql.Open("sqlite3", ":memory:?_foreign_keys=on")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := store.InitializeSchema(db); err != nil {
		t.Fatalf("init schema: %v", err)
	}
	t.Cleanup(func() { db.Close() })

	// Mock inference server returning valid interpretation JSON.
	mockInference := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		content := `{"interpretation":"Test activity","category":"browsing","app_name":"TestApp"}`
		resp := map[string]any{
			"choices": []map[string]any{
				{"message": map[string]string{"content": content}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(mockInference.Close)

	cfgMgr, _ := config.NewManager(filepath.Join(t.TempDir(), "config.yml"))
	infClient := inference.NewOpenAIClient(inference.ClientConfig{Host: mockInference.URL, MaxRetries: 1})

	tokenService := auth.NewTokenService(db)
	pairingService := auth.NewPairingService(db)
	recoveryService := auth.NewRecoveryService(db)

	deviceStore := store.NewDeviceStore(db)
	journalStore := store.NewJournalStore(db)
	metadataStore := store.NewMetadataStore(db)

	interpreter := processing.NewInterpreter(infClient, cfgMgr, metadataStore)
	captureQueue := processing.NewQueue(interpreter, 100, 1)
	consolidator := processing.NewConsolidator(infClient, cfgMgr, metadataStore, journalStore)
	reconsolidator := processing.NewReconsolidator(consolidator, cfgMgr, metadataStore, journalStore)

	srv := NewServer(Dependencies{
		TokenService:    tokenService,
		PairingService:  pairingService,
		RecoveryService: recoveryService,
		ConfigManager:   cfgMgr,
		InferenceClient: infClient,
		JournalStore:    journalStore,
		MetadataStore:   metadataStore,
		DeviceStore:     deviceStore,
		Queue:           captureQueue,
		Interpreter:     interpreter,
		Consolidator:    consolidator,
		Reconsolidator:  reconsolidator,
		Version:         "0.1.0-test",
	})

	// Pre-pair a test device.
	code, _ := pairingService.GenerateCode()
	deviceID, token, _ := pairingService.RedeemCode(code, "Test Device")

	return &testEnv{
		server:          srv,
		db:              db,
		token:           token,
		deviceID:        deviceID,
		journalStore:    journalStore,
		metadataStore:   metadataStore,
		pairingService:  pairingService,
		recoveryService: recoveryService,
	}
}

func (e *testEnv) doRequest(method, path string, body io.Reader, auth bool) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, body)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	w := httptest.NewRecorder()
	e.server.Router.ServeHTTP(w, req)
	return w
}

func (e *testEnv) doMultipart(path string, fieldName string, imageData []byte, extraFields map[string]string) *httptest.ResponseRecorder {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, _ := writer.CreateFormFile(fieldName, "screenshot.png")
	part.Write(imageData)

	for k, v := range extraFields {
		writer.WriteField(k, v)
	}
	writer.Close()

	req := httptest.NewRequest("POST", path, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+e.token)
	w := httptest.NewRecorder()
	e.server.Router.ServeHTTP(w, req)
	return w
}

func parseJSON(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var result map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatalf("parse JSON response: %v\nbody: %s", err, w.Body.String())
	}
	return result
}

// Valid PNG header bytes.
var validPNG = []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
	0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
	0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
	0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53}

// --- Status ---

func TestStatus_OK(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/status", nil, false)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	body := parseJSON(t, w)
	if body["version"] != "0.1.0-test" {
		t.Errorf("version = %v", body["version"])
	}
	// device_count should be 1 (the pre-paired device).
	if int(body["device_count"].(float64)) != 1 {
		t.Errorf("device_count = %v, want 1", body["device_count"])
	}
}

// --- Auth ---

func TestAuth_Pair_Success(t *testing.T) {
	env := setupTestEnv(t)

	code, _ := env.pairingService.GenerateCode()
	body := fmt.Sprintf(`{"code":"%s","device_name":"New Device"}`, code)
	w := env.doRequest("POST", "/auth/pair", strings.NewReader(body), false)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["device_id"] == nil || resp["device_id"] == "" {
		t.Error("response should include device_id")
	}
	if resp["token"] == nil || resp["token"] == "" {
		t.Error("response should include token")
	}
}

func TestAuth_Pair_InvalidCode(t *testing.T) {
	env := setupTestEnv(t)
	body := `{"code":"BADCOD","device_name":"Device"}`
	w := env.doRequest("POST", "/auth/pair", strings.NewReader(body), false)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want 401", w.Code)
	}
}

func TestAuth_PairNew_Authenticated(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/auth/pair/new", nil, true)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["code"] == nil || resp["code"] == "" {
		t.Error("response should include pairing code")
	}
}

func TestAuth_PairNew_Unauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("POST", "/auth/pair/new", nil, false)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want 401", w.Code)
	}
}

func TestAuth_Recover_Success(t *testing.T) {
	env := setupTestEnv(t)

	key, _ := env.recoveryService.GenerateRecoveryKey()
	body := fmt.Sprintf(`{"recovery_key":"%s"}`, key)
	w := env.doRequest("POST", "/auth/recover", strings.NewReader(body), false)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200\nbody: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if resp["pairing_code"] == nil || resp["pairing_code"] == "" {
		t.Error("response should include pairing_code")
	}
}

func TestAuth_Recover_InvalidKey(t *testing.T) {
	env := setupTestEnv(t)
	env.recoveryService.GenerateRecoveryKey()

	body := `{"recovery_key":"wrong_key_value"}`
	w := env.doRequest("POST", "/auth/recover", strings.NewReader(body), false)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want 401", w.Code)
	}
}

// --- Journal ---

func TestJournal_List_Empty(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/journal", nil, true)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d, want 200", w.Code)
	}
	resp := parseJSON(t, w)
	entries := resp["entries"].([]any)
	if len(entries) != 0 {
		t.Errorf("entries = %d, want 0", len(entries))
	}
	if int(resp["total"].(float64)) != 0 {
		t.Errorf("total = %v, want 0", resp["total"])
	}
}

func TestJournal_List_Paginated(t *testing.T) {
	env := setupTestEnv(t)

	// Insert 3 entries.
	for i := 0; i < 3; i++ {
		env.journalStore.Create(store.JournalEntry{
			ID:        uuid.New().String(),
			Narrative: fmt.Sprintf("Entry %d", i),
			TimeStart: time.Now().UTC().Add(time.Duration(-i) * time.Hour),
			TimeEnd:   time.Now().UTC().Add(time.Duration(-i)*time.Hour + 30*time.Minute),
			CreatedAt: time.Now().UTC(),
		})
	}

	w := env.doRequest("GET", "/journal?limit=2&offset=0", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	resp := parseJSON(t, w)
	entries := resp["entries"].([]any)
	if len(entries) != 2 {
		t.Errorf("entries = %d, want 2", len(entries))
	}
	if int(resp["total"].(float64)) != 3 {
		t.Errorf("total = %v, want 3", resp["total"])
	}
}

func TestJournal_Get_NotFound(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/journal/nonexistent-id", nil, true)

	if w.Code != http.StatusNotFound {
		t.Errorf("status code = %d, want 404", w.Code)
	}
}

func TestJournal_CRUD(t *testing.T) {
	env := setupTestEnv(t)

	entry := store.JournalEntry{
		ID:        uuid.New().String(),
		Narrative: "Original narrative",
		TimeStart: time.Now().UTC().Add(-30 * time.Minute),
		TimeEnd:   time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
	env.journalStore.Create(entry)

	// GET
	w := env.doRequest("GET", "/journal/"+entry.ID, nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("GET status = %d", w.Code)
	}
	resp := parseJSON(t, w)
	if resp["narrative"] != "Original narrative" {
		t.Errorf("narrative = %v", resp["narrative"])
	}

	// PUT
	body := `{"narrative":"Updated narrative"}`
	w = env.doRequest("PUT", "/journal/"+entry.ID, strings.NewReader(body), true)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d\nbody: %s", w.Code, w.Body.String())
	}
	resp = parseJSON(t, w)
	if resp["narrative"] != "Updated narrative" {
		t.Errorf("updated narrative = %v", resp["narrative"])
	}
	if resp["edited"] != true {
		t.Error("edited should be true after PUT")
	}

	// DELETE
	w = env.doRequest("DELETE", "/journal/"+entry.ID, nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("DELETE status = %d", w.Code)
	}

	// Verify deleted.
	w = env.doRequest("GET", "/journal/"+entry.ID, nil, true)
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", w.Code)
	}
}

func TestJournal_Unauthenticated(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/journal", nil, false)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("status code = %d, want 401", w.Code)
	}
}

// --- Capture ---

func TestCapture_MissingFile(t *testing.T) {
	env := setupTestEnv(t)
	req := httptest.NewRequest("POST", "/capture", nil)
	req.Header.Set("Authorization", "Bearer "+env.token)
	req.Header.Set("Content-Type", "multipart/form-data; boundary=xxx")
	w := httptest.NewRecorder()
	env.server.Router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want 400", w.Code)
	}
}

func TestCapture_InvalidImage(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doMultipart("/capture", "screenshot", []byte("not an image"), nil)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status code = %d, want 400", w.Code)
	}
	body := parseJSON(t, w)
	errMsg, _ := body["error"].(string)
	if !strings.Contains(errMsg, "invalid image") {
		t.Errorf("error = %q, should mention invalid image", errMsg)
	}
}

func TestCapture_TooLarge(t *testing.T) {
	env := setupTestEnv(t)
	// Create data larger than 10 MB with valid PNG header.
	largeData := make([]byte, 10<<20+100)
	copy(largeData, validPNG)

	w := env.doMultipart("/capture", "screenshot", largeData, nil)
	if w.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status code = %d, want 413", w.Code)
	}
}

func TestCapture_Success(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doMultipart("/capture", "screenshot", validPNG, nil)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status code = %d, want 202\nbody: %s", w.Code, w.Body.String())
	}
	resp := parseJSON(t, w)
	if int(resp["accepted"].(float64)) != 1 {
		t.Error("response should have accepted=1")
	}
}

// --- Config ---

func TestConfig_GetDefault(t *testing.T) {
	env := setupTestEnv(t)
	w := env.doRequest("GET", "/config", nil, true)

	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	resp := parseJSON(t, w)
	if int(resp["consolidation_window_min"].(float64)) != 30 {
		t.Errorf("consolidation_window_min = %v, want 30", resp["consolidation_window_min"])
	}
	if resp["interpretation_detail"] != "standard" {
		t.Errorf("interpretation_detail = %v", resp["interpretation_detail"])
	}
	if resp["journal_tone"] != "casual" {
		t.Errorf("journal_tone = %v", resp["journal_tone"])
	}
}

func TestConfig_Update(t *testing.T) {
	env := setupTestEnv(t)

	body := `{"consolidation_window_min":60,"journal_tone":"concise"}`
	w := env.doRequest("PUT", "/config", strings.NewReader(body), true)
	if w.Code != http.StatusOK {
		t.Fatalf("PUT status = %d\nbody: %s", w.Code, w.Body.String())
	}

	// Verify via GET.
	w = env.doRequest("GET", "/config", nil, true)
	resp := parseJSON(t, w)
	if int(resp["consolidation_window_min"].(float64)) != 60 {
		t.Errorf("consolidation_window_min = %v, want 60", resp["consolidation_window_min"])
	}
	if resp["journal_tone"] != "concise" {
		t.Errorf("journal_tone = %v, want concise", resp["journal_tone"])
	}
}

func TestConfig_InvalidValue(t *testing.T) {
	env := setupTestEnv(t)

	body := `{"journal_tone":"poetic"}`
	w := env.doRequest("PUT", "/config", strings.NewReader(body), true)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status code = %d, want 500 for invalid config", w.Code)
	}
}

// --- Metadata ---

func TestMetadata_GetByEntry(t *testing.T) {
	env := setupTestEnv(t)

	entry := store.JournalEntry{
		ID:        uuid.New().String(),
		Narrative: "Test entry",
		TimeStart: time.Now().UTC().Add(-30 * time.Minute),
		TimeEnd:   time.Now().UTC(),
		CreatedAt: time.Now().UTC(),
	}
	env.journalStore.Create(entry)

	meta := store.Metadata{
		ID:             uuid.New().String(),
		DeviceID:       env.deviceID,
		CapturedAt:     time.Now().UTC(),
		Interpretation: "Test",
		Category:       "browsing",
		AppName:        "TestApp",
		EntryID:        &entry.ID,
		CreatedAt:      time.Now().UTC(),
	}
	env.metadataStore.Create(meta)

	w := env.doRequest("GET", "/journal/"+entry.ID+"/metadata", nil, true)
	if w.Code != http.StatusOK {
		t.Fatalf("status code = %d", w.Code)
	}
	resp := parseJSON(t, w)
	metadata := resp["metadata"].([]any)
	if len(metadata) != 1 {
		t.Errorf("metadata count = %d, want 1", len(metadata))
	}
}
