// OpenAI-compatible client for SGLang, vLLM, and any server that
// implements the /v1/chat/completions endpoint.
package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// OpenAIClient talks to any OpenAI-compatible inference server.
// Automatically sets chat_template_kwargs to disable reasoning/thinking.
type OpenAIClient struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	model      string
}

// openAIChatRequest is the wire format for /v1/chat/completions.
type openAIChatRequest struct {
	Model              string         `json:"model,omitempty"`
	Messages           []Message      `json:"messages"`
	Temperature        float64        `json:"temperature,omitempty"`
	MaxTokens          int            `json:"max_tokens,omitempty"`
	ChatTemplateKwargs map[string]any `json:"chat_template_kwargs,omitempty"`
}

// openAIChatResponse is the wire format returned by /v1/chat/completions.
type openAIChatResponse struct {
	Choices []struct {
		Message struct {
			Content   string `json:"content"`
			Reasoning string `json:"reasoning"`
		} `json:"message"`
	} `json:"choices"`
}

// modelsResponse is the /v1/models response used for model auto-detection.
type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

func NewOpenAIClient(cfg ClientConfig) *OpenAIClient {
	retries := cfg.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	c := &OpenAIClient{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    cfg.Host,
		maxRetries: retries,
	}
	c.model = c.detectModel()
	return c
}

func (c *OpenAIClient) detectModel() string {
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

func (c *OpenAIClient) Complete(req ChatRequest) (string, error) {
	wireReq := openAIChatRequest{
		Model:       req.Model,
		Messages:    req.Messages,
		Temperature: req.Temperature,
		MaxTokens:   req.MaxTokens,
		// Disable reasoning for models that support it (e.g. Qwen3.5)
		ChatTemplateKwargs: map[string]any{"enable_thinking": false},
	}
	if wireReq.Model == "" && c.model != "" {
		wireReq.Model = c.model
	}

	body, err := json.Marshal(wireReq)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	var lastErr error
	for attempt := range c.maxRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		res, err := c.httpClient.Post(
			c.baseURL+"/v1/chat/completions",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			lastErr = fmt.Errorf("inference request: %w", err)
			continue
		}

		if res.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(res.Body)
			res.Body.Close()
			lastErr = fmt.Errorf("inference returned %d: %s", res.StatusCode, string(respBody))
			if isRetryable(res.StatusCode) {
				continue
			}
			return "", lastErr
		}

		respBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		var chatResp openAIChatResponse
		if err := json.Unmarshal(respBody, &chatResp); err != nil {
			return "", fmt.Errorf("decode response: %w", err)
		}
		if len(chatResp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		if DebugLog() {
			log.Printf("[inference] raw response: %s", string(respBody))
		}

		content := stripThinking(chatResp.Choices[0].Message.Content)
		if content == "" {
			content = stripThinking(chatResp.Choices[0].Message.Reasoning)
		}
		if content == "" {
			return "", fmt.Errorf("inference returned empty content")
		}
		return content, nil
	}

	return "", fmt.Errorf("inference failed after %d attempts: %w", c.maxRetries, lastErr)
}

func (c *OpenAIClient) IsAvailable() bool {
	res, err := c.httpClient.Get(c.baseURL + "/v1/models")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
