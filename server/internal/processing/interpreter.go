// Stage 1: screenshot → inference → metadata row.
package processing

import (
	"encoding/base64"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

type Interpreter struct {
	client   *inference.Client
	config   *config.Manager
	metadata *store.MetadataStore
}

func NewInterpreter(client *inference.Client, cfg *config.Manager, metadata *store.MetadataStore) *Interpreter {
	return &Interpreter{client: client, config: cfg, metadata: metadata}
}

// Interpret processes a screenshot and returns the resulting metadata.
func (i *Interpreter) Interpret(imageData []byte, deviceID string, capturedAt time.Time) (*store.Metadata, error) {
	cfg := i.config.Get()

	b64Image := base64.StdEncoding.EncodeToString(imageData)

	req := inference.ChatRequest{
		Messages: []inference.Message{
			{Role: "system", Content: inference.InterpretationSystemPrompt(cfg.InterpretationDetail)},
			{Role: "user", Content: []map[string]any{
				{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + b64Image}},
				{"type": "text", "text": fmt.Sprintf("Device: %s, Captured at: %s", deviceID, capturedAt.Format(time.RFC3339))},
			}},
		},
		Temperature: 0.3,
		MaxTokens:   500,
	}

	raw, err := i.client.Complete(req)
	if err != nil {
		return nil, fmt.Errorf("inference: %w", err)
	}

	result, err := inference.ParseInterpretation(raw)
	if err != nil {
		return nil, fmt.Errorf("parse interpretation: %w", err)
	}

	meta := store.Metadata{
		ID:             uuid.New().String(),
		DeviceID:       deviceID,
		CapturedAt:     capturedAt,
		Interpretation: result.Interpretation,
		Category:       result.Category,
		AppName:        result.AppName,
		CreatedAt:      time.Now().UTC(),
	}

	if err := i.metadata.Create(meta); err != nil {
		return nil, fmt.Errorf("store metadata: %w", err)
	}

	return &meta, nil
}
