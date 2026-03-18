// OpenAI-compatible API client for SGLang.
package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	model      string
}

// modelsResponse is the OpenAI-compatible /v1/models response.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// Message represents a chat completion message.
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

// ChatRequest is the OpenAI-compatible chat completion request.
type ChatRequest struct {
	Model       string    `json:"model,omitempty"`
	Messages    []Message `json:"messages"`
	Temperature float64   `json:"temperature,omitempty"`
	MaxTokens   int       `json:"max_tokens,omitempty"`
}

// ChatResponse is the OpenAI-compatible chat completion response.
type ChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
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
//	INFERENCE_TIMEOUT_SEC (default: 120)
//	INFERENCE_MAX_RETRIES (default: 3)
func DefaultClientConfig() ClientConfig {
	host := os.Getenv("INFERENCE_HOST")
	if host == "" {
		host = "http://localhost:8000"
	}

	timeout := 120 * time.Second
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

func NewClient(cfg ClientConfig) *Client {
	retries := cfg.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	c := &Client{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    cfg.Host,
		maxRetries: retries,
	}
	// Auto-detect model from the inference backend
	c.model = c.detectModel()
	return c
}

// detectModel queries /v1/models and returns the first available model ID.
// Uses a short timeout so it doesn't block server startup.
func (c *Client) detectModel() string {
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(c.baseURL + "/v1/models")
	if err != nil {
		return ""
	}
	defer res.Body.Close()

	var models modelsResponse
	if err := json.NewDecoder(res.Body).Decode(&models); err != nil {
		return ""
	}
	if len(models.Data) > 0 {
		return models.Data[0].ID
	}
	return ""
}

// isRetryable returns true for errors that are likely transient.
func isRetryable(statusCode int) bool {
	switch statusCode {
	case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// Complete sends a chat completion request and returns the response text.
// Retries transient failures with exponential backoff.
func (c *Client) Complete(req ChatRequest) (string, error) {
	if req.Model == "" && c.model != "" {
		req.Model = c.model
	}
	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := range c.maxRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second) // 1s, 2s, 4s...
		}

		res, err := c.httpClient.Post(
			c.baseURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			lastErr = fmt.Errorf("inference request: %w", err)
			continue // network error — retry
		}

		if res.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(res.Body)
			res.Body.Close()
			lastErr = fmt.Errorf("inference returned %d: %s", res.StatusCode, string(respBody))
			if isRetryable(res.StatusCode) {
				continue
			}
			return "", lastErr // non-retryable HTTP error
		}

		var chatResp ChatResponse
		decodeErr := json.NewDecoder(res.Body).Decode(&chatResp)
		res.Body.Close()
		if decodeErr != nil {
			return "", fmt.Errorf("decode response: %w", decodeErr)
		}

		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		return chatResp.Choices[0].Message.Content, nil
	}

	return "", fmt.Errorf("inference failed after %d attempts: %w", c.maxRetries, lastErr)
}

// IsAvailable checks if the inference server is reachable.
func (c *Client) IsAvailable() bool {
	res, err := c.httpClient.Get(c.baseURL + "/v1/models")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
