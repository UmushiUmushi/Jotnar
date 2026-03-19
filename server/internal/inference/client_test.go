package inference

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func newMockInferenceServer(handler http.HandlerFunc) (*httptest.Server, Client) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reject Ollama probe so auto-detection picks OpenAI
		if r.URL.Path == "/api/tags" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		// Handle model detection during NewClient without hitting the test handler
		if r.URL.Path == "/v1/models" {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(`{"data":[{"id":"test-model"}]}`))
			return
		}
		handler(w, r)
	}))
	client := NewClient(ClientConfig{
		Host:       ts.URL,
		Timeout:    0,
		MaxRetries: 3,
	})
	return ts, client
}

func chatResponseJSON(content string) []byte {
	resp := openAIChatResponse{
		Choices: []struct {
			Message struct {
				Content   string `json:"content"`
				Reasoning string `json:"reasoning"`
			} `json:"message"`
		}{
			{Message: struct {
				Content   string `json:"content"`
				Reasoning string `json:"reasoning"`
			}{Content: content}},
		},
	}
	b, _ := json.Marshal(resp)
	return b
}

func TestComplete_Success(t *testing.T) {
	ts, client := newMockInferenceServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatResponseJSON("hello world"))
	})
	defer ts.Close()

	result, err := client.Complete(ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "hello world" {
		t.Errorf("result = %q, want %q", result, "hello world")
	}
}

func TestComplete_RetryOnTransientError(t *testing.T) {
	var attempts atomic.Int32
	ts, client := newMockInferenceServer(func(w http.ResponseWriter, r *http.Request) {
		n := attempts.Add(1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			w.Write([]byte("temporarily unavailable"))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(chatResponseJSON("recovered"))
	})
	defer ts.Close()

	result, err := client.Complete(ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "recovered" {
		t.Errorf("result = %q, want %q", result, "recovered")
	}
	if attempts.Load() != 2 {
		t.Errorf("attempts = %d, want 2", attempts.Load())
	}
}

func TestComplete_NonRetryableError(t *testing.T) {
	var attempts atomic.Int32
	ts, client := newMockInferenceServer(func(w http.ResponseWriter, r *http.Request) {
		attempts.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("bad request"))
	})
	defer ts.Close()

	_, err := client.Complete(ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for 400 response, got nil")
	}
	if attempts.Load() != 1 {
		t.Errorf("attempts = %d, want 1 (no retries for 400)", attempts.Load())
	}
}

func TestComplete_EmptyChoices(t *testing.T) {
	ts, client := newMockInferenceServer(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[]}`))
	})
	defer ts.Close()

	_, err := client.Complete(ChatRequest{
		Messages: []Message{{Role: "user", Content: "test"}},
	})
	if err == nil {
		t.Fatal("expected error for empty choices, got nil")
	}
	if !strings.Contains(err.Error(), "no choices") {
		t.Errorf("error = %q, want it to contain 'no choices'", err.Error())
	}
}

func TestIsAvailable_True(t *testing.T) {
	ts, client := newMockInferenceServer(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":[]}`))
	})
	defer ts.Close()

	if !client.IsAvailable() {
		t.Error("expected IsAvailable() = true")
	}
}

func TestIsAvailable_False(t *testing.T) {
	client := NewClient(ClientConfig{
		Host:       "http://127.0.0.1:1",
		MaxRetries: 1,
	})
	if client.IsAvailable() {
		t.Error("expected IsAvailable() = false for unreachable server")
	}
}
