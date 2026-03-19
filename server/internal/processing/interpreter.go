// Stage 1: screenshot → inference → metadata row.
package processing

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	_ "image/jpeg"
	"image/png"
	"log"
	"time"

	"golang.org/x/image/draw"

	"github.com/google/uuid"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

// maxImageWidth is the maximum width before downscaling.
// 720px is enough detail for the model to read text and identify apps.
const maxImageWidth = 720

type Interpreter struct {
	client   inference.Client
	config   *config.Manager
	metadata *store.MetadataStore
}

func NewInterpreter(client inference.Client, cfg *config.Manager, metadata *store.MetadataStore) *Interpreter {
	return &Interpreter{client: client, config: cfg, metadata: metadata}
}

// Interpret processes a screenshot and returns the resulting metadata.
func (i *Interpreter) Interpret(imageData []byte, deviceID string, capturedAt time.Time) (*store.Metadata, error) {
	cfg := i.config.Get()

	imageData = downscaleImage(imageData)

	b64Image := base64.StdEncoding.EncodeToString(imageData)

	req := inference.ChatRequest{
		Messages: []inference.Message{
			{Role: "system", Content: inference.InterpretationSystemPrompt(cfg.InterpretationDetail)},
			{Role: "user", Content: []map[string]any{
				{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + b64Image}},
				{"type": "text", "text": fmt.Sprintf("Device: %s, Captured at: %s", deviceID, capturedAt.In(cfg.Location()).Format(time.RFC3339))},
			}},
		},
		Temperature: 0.3,
		MaxTokens:   1000,
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

// downscaleImage resizes the image to maxImageWidth if it's wider,
// preserving aspect ratio. Returns the original bytes if decoding
// fails or the image is already small enough.
func downscaleImage(data []byte) []byte {
	src, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data
	}

	bounds := src.Bounds()
	srcW := bounds.Dx()
	if srcW <= maxImageWidth {
		return data
	}

	ratio := float64(maxImageWidth) / float64(srcW)
	dstW := maxImageWidth
	dstH := int(float64(bounds.Dy()) * ratio)

	dst := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	draw.BiLinear.Scale(dst, dst.Bounds(), src, bounds, draw.Over, nil)

	var buf bytes.Buffer
	if err := png.Encode(&buf, dst); err != nil {
		return data
	}

	log.Printf("Downscaled image: %dx%d → %dx%d (%d → %d bytes)", srcW, bounds.Dy(), dstW, dstH, len(data), buf.Len())
	return buf.Bytes()
}
