// Inference client interface and factory.
package inference

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// debugLog controls whether raw inference responses are logged.
// Off by default; toggled at runtime via the `debuglog` subcommand.
var debugLog atomic.Bool

// SetDebugLog enables or disables inference debug logging.
func SetDebugLog(on bool) {
	debugLog.Store(on)
}

// DebugLog returns the current state of inference debug logging.
func DebugLog() bool {
	return debugLog.Load()
}

// Client is the interface for all inference backends.
// Any server that can take a chat completion request and return text
// (SGLang, vLLM, Ollama, OpenAI, Anthropic proxy, etc.) implements this.
type Client interface {
	// Complete sends a chat request and returns the model's text response.
	Complete(req ChatRequest) (string, error)

	// IsAvailable checks whether the inference server is reachable.
	IsAvailable() bool
}

// Message represents a chat completion message.
// Content is `any` to support both plain strings and the OpenAI
// multimodal format ([]map[string]any with image_url / text parts).
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatRequest is the backend-agnostic request that callers build.
// Each Client implementation translates it to its native wire format.
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ClientConfig holds connection settings for the inference server.
type ClientConfig struct {
	Host       string
	Timeout    time.Duration
	MaxRetries int
}

// DefaultClientConfig returns a ClientConfig populated from environment
// variables, falling back to sensible defaults:
//
//	INFERENCE_HOST        (default: http://localhost:8000)
//	INFERENCE_TIMEOUT_SEC (default: 300)
//	INFERENCE_MAX_RETRIES (default: 3)
func DefaultClientConfig() ClientConfig {
	host := os.Getenv("INFERENCE_HOST")
	if host == "" {
		host = "http://localhost:8000"
	}

	timeout := 300 * time.Second
	if v := os.Getenv("INFERENCE_TIMEOUT_SEC"); v != "" {
		if sec, err := strconv.Atoi(v); err == nil && sec > 0 {
			timeout = time.Duration(sec) * time.Second
		}
	}

	maxRetries := 3
	if v := os.Getenv("INFERENCE_MAX_RETRIES"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxRetries = n
		}
	}

	return ClientConfig{
		Host:       host,
		Timeout:    timeout,
		MaxRetries: maxRetries,
	}
}

// DefaultWorkers returns the INFERENCE_WORKERS value from the environment,
// falling back to 1.
func DefaultWorkers() int {
	if v := os.Getenv("INFERENCE_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return 1
}

// WaitAndCreateClient blocks until the inference server is reachable, then
// creates the appropriate client. Always auto-detects the backend type by
// probing both Ollama and OpenAI health endpoints.
func WaitAndCreateClient(cfg ClientConfig) Client {
	log.Printf("Waiting for inference server at %s ...", cfg.Host)
	backend := DetectBackend(cfg.Host)
	log.Printf("Inference server at %s is ready (backend: %s)", cfg.Host, backend)

	switch backend {
	case "ollama":
		return NewOllamaClient(cfg)
	default:
		return NewOpenAIClient(cfg)
	}
}

// DetectBackend polls both Ollama and OpenAI endpoints until one
// responds, and returns which backend type it found ("ollama" or "openai").
func DetectBackend(host string) string {
	c := &http.Client{Timeout: 5 * time.Second}
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	// Check immediately, then on each tick.
	for ; ; <-ticker.C {
		if res, err := c.Get(host + "/api/tags"); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return "ollama"
			}
		}
		if res, err := c.Get(host + "/v1/models"); err == nil {
			res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return "openai"
			}
		}
	}
}

// isRetryable returns true for HTTP status codes that are likely transient.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// stripThinking removes <think>...</think> blocks from model output.
// Reasoning models may wrap chain-of-thought in these tags; we only
// want the actual answer that follows. Kept as a safety net even when
// thinking is explicitly disabled.
func stripThinking(s string) string {
	if idx := strings.Index(s, "</think>"); idx != -1 {
		return strings.TrimSpace(s[idx+len("</think>"):])
	}
	return strings.TrimSpace(s)
}
