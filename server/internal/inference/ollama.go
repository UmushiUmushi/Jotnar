// Ollama client using the native /api/chat endpoint with thinking disabled.
package inference

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// OllamaClient talks to an Ollama server via its native /api/chat endpoint.
type OllamaClient struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	model      string
}

type ollamaChatRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Think    bool            `json:"think"`
	Stream   bool            `json:"stream"`
	Options  *ollamaOptions  `json:"options,omitempty"`
}

type ollamaMessage struct {
	Role    string   `json:"role"`
	Content string   `json:"content"`
	Images  []string `json:"images,omitempty"`
}

type ollamaOptions struct {
	Temperature float64 `json:"temperature,omitempty"`
	NumPredict  int     `json:"num_predict,omitempty"`
}

type ollamaChatResponse struct {
	Message struct {
		Content string `json:"content"`
	} `json:"message"`
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

func NewOllamaClient(cfg ClientConfig) *OllamaClient {
	retries := cfg.MaxRetries
	if retries <= 0 {
		retries = 3
	}
	c := &OllamaClient{
		httpClient: &http.Client{Timeout: cfg.Timeout},
		baseURL:    cfg.Host,
		maxRetries: retries,
	}
	c.model = c.detectModel()
	return c
}

func (c *OllamaClient) detectModel() string {
	client := &http.Client{Timeout: 5 * time.Second}
	res, err := client.Get(c.baseURL + "/api/tags")
	if err != nil {
		return ""
	}
	defer res.Body.Close()

	var tags ollamaTagsResponse
	if err := json.NewDecoder(res.Body).Decode(&tags); err != nil {
		return ""
	}
	if len(tags.Models) > 0 {
		return tags.Models[0].Name
	}
	return ""
}

func (c *OllamaClient) Complete(req ChatRequest) (string, error) {
	model := req.Model
	if model == "" {
		model = c.model
	}

	ollamaReq := ollamaChatRequest{
		Model:    model,
		Messages: convertToOllamaMessages(req.Messages),
		Think:    false,
		Stream:   false,
	}
	if req.Temperature > 0 || req.MaxTokens > 0 {
		ollamaReq.Options = &ollamaOptions{
			Temperature: req.Temperature,
			NumPredict:  req.MaxTokens,
		}
	}

	body, err := json.Marshal(ollamaReq)
	if err != nil {
		return "", fmt.Errorf("marshal ollama request: %w", err)
	}

	var lastErr error
	for attempt := range c.maxRetries {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		res, err := c.httpClient.Post(
			c.baseURL+"/api/chat",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			lastErr = fmt.Errorf("ollama request: %w", err)
			continue
		}

		if res.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(res.Body)
			res.Body.Close()
			lastErr = fmt.Errorf("ollama returned %d: %s", res.StatusCode, string(respBody))
			if isRetryable(res.StatusCode) {
				continue
			}
			return "", lastErr
		}

		respBody, _ := io.ReadAll(res.Body)
		res.Body.Close()

		var ollamaResp ollamaChatResponse
		if err := json.Unmarshal(respBody, &ollamaResp); err != nil {
			return "", fmt.Errorf("decode ollama response: %w", err)
		}

		if DebugLog() {
			log.Printf("[inference] ollama raw response: %s", string(respBody))
		}

		content := stripThinking(ollamaResp.Message.Content)
		if content == "" {
			return "", fmt.Errorf("ollama returned empty content")
		}
		return content, nil
	}

	return "", fmt.Errorf("ollama failed after %d attempts: %w", c.maxRetries, lastErr)
}

// convertToOllamaMessages converts OpenAI-format messages to Ollama format.
// Handles multimodal content (image_url + text) by extracting base64 images
// into Ollama's images field.
func convertToOllamaMessages(messages []Message) []ollamaMessage {
	out := make([]ollamaMessage, 0, len(messages))
	for _, m := range messages {
		om := ollamaMessage{Role: m.Role}
		switch content := m.Content.(type) {
		case string:
			om.Content = content
		case []map[string]any:
			var texts []string
			for _, part := range content {
				switch part["type"] {
				case "text":
					if t, ok := part["text"].(string); ok {
						texts = append(texts, t)
					}
				case "image_url":
					if imgObj, ok := part["image_url"].(map[string]string); ok {
						url := imgObj["url"]
						if idx := strings.Index(url, ","); idx != -1 {
							om.Images = append(om.Images, url[idx+1:])
						} else {
							om.Images = append(om.Images, url)
						}
					}
				}
			}
			om.Content = strings.Join(texts, "\n")
		default:
			if b, err := json.Marshal(content); err == nil {
				om.Content = string(b)
			}
		}
		out = append(out, om)
	}
	return out
}

func (c *OllamaClient) IsAvailable() bool {
	res, err := c.httpClient.Get(c.baseURL + "/api/tags")
	if err != nil {
		return false
	}
	defer res.Body.Close()
	return res.StatusCode == http.StatusOK
}
