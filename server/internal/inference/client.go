// Inference client interface and factory.
package inference

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

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

// NewClient creates the appropriate Client implementation.
// Uses INFERENCE_BACKEND env var ("ollama" or "openai") if set,
// otherwise auto-detects by probing the server.
func NewClient(cfg ClientConfig) Client {
	backend := strings.ToLower(os.Getenv("INFERENCE_BACKEND"))

	if backend == "" {
		backend = detectBackend(cfg.Host)
	}

	switch backend {
	case "ollama":
		log.Printf("[inference] using Ollama backend (%s)", cfg.Host)
		return NewOllamaClient(cfg)
	default:
		log.Printf("[inference] using OpenAI-compatible backend (%s)", cfg.Host)
		return NewOpenAIClient(cfg)
	}
}

// detectBackend probes the server to determine its type.
// If Ollama's /api/tags responds, it's Ollama; otherwise OpenAI-compatible.
func detectBackend(host string) string {
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(host + "/api/tags")
	if err == nil {
		res.Body.Close()
		if res.StatusCode == http.StatusOK {
			return "ollama"
		}
	}
	return "openai"
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
