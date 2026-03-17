// POST /capture — receive screenshot from device.
package routes

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/api/middleware"
	"github.com/jotnar/server/internal/processing"
)

type CaptureResponse struct {
	ID             string `json:"id"`
	Interpretation string `json:"interpretation"`
	Category       string `json:"category"`
	AppName        string `json:"app_name"`
}

type BatchCaptureResult struct {
	Index          int    `json:"index"`
	ID             string `json:"id,omitempty"`
	Interpretation string `json:"interpretation,omitempty"`
	Category       string `json:"category,omitempty"`
	AppName        string `json:"app_name,omitempty"`
	Error          string `json:"error,omitempty"`
}

type BatchCaptureResponse struct {
	Results   []BatchCaptureResult `json:"results"`
	Succeeded int                  `json:"succeeded"`
	Failed    int                  `json:"failed"`
}

type CaptureHandler struct {
	interpreter *processing.Interpreter
}

func NewCaptureHandler(interpreter *processing.Interpreter) *CaptureHandler {
	return &CaptureHandler{interpreter: interpreter}
}

// maxScreenshotSize is the maximum allowed screenshot size (10 MB).
const maxScreenshotSize = 10 << 20

// pngMagic and jpegMagic are the file header magic bytes for PNG and JPEG.
var pngMagic = []byte{0x89, 0x50, 0x4E, 0x47}
var jpegMagic = []byte{0xFF, 0xD8, 0xFF}

// isValidImage checks whether the data starts with a known image header.
func isValidImage(data []byte) bool {
	if len(data) >= 4 && bytes.Equal(data[:4], pngMagic) {
		return true
	}
	if len(data) >= 3 && bytes.Equal(data[:3], jpegMagic) {
		return true
	}
	return false
}

func (h *CaptureHandler) Capture(c *gin.Context) {
	deviceID, _ := c.Get(middleware.DeviceIDKey)

	file, _, err := c.Request.FormFile("screenshot")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing screenshot file"})
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(io.LimitReader(file, maxScreenshotSize+1))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read screenshot"})
		return
	}
	if len(imageData) > maxScreenshotSize {
		c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "screenshot exceeds 10 MB limit"})
		return
	}
	if !isValidImage(imageData) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid image format, expected PNG or JPEG"})
		return
	}

	capturedAtStr := c.PostForm("captured_at")
	capturedAt, err := time.Parse(time.RFC3339, capturedAtStr)
	if err != nil {
		capturedAt = time.Now().UTC()
	}

	meta, err := h.interpreter.Interpret(imageData, deviceID.(string), capturedAt)
	if err != nil {
		log.Printf("Capture: interpretation failed for device %s: %v", deviceID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "interpretation failed"})
		return
	}

	log.Printf("Capture: device %s — %s (%s)", deviceID, meta.AppName, meta.Category)
	c.JSON(http.StatusOK, CaptureResponse{
		ID:             meta.ID,
		Interpretation: meta.Interpretation,
		Category:       meta.Category,
		AppName:        meta.AppName,
	})
}

func (h *CaptureHandler) BatchCapture(c *gin.Context) {
	deviceID, _ := c.Get(middleware.DeviceIDKey)

	if err := c.Request.ParseMultipartForm(50 << 20); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to parse multipart form"})
		return
	}

	form := c.Request.MultipartForm
	files := form.File["screenshots"]
	timestamps := form.Value["captured_at"]

	if len(files) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no screenshots provided"})
		return
	}
	if len(files) > 50 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "maximum 50 screenshots per batch"})
		return
	}

	// Limit concurrent inference calls — the model processes sequentially anyway.
	const maxConcurrentInference = 4
	sem := make(chan struct{}, maxConcurrentInference)

	results := make([]BatchCaptureResult, len(files))
	var wg sync.WaitGroup
	var mu sync.Mutex
	succeeded, failed := 0, 0

	for i, fh := range files {
		// Parse timestamp for this file, fall back to now
		var capturedAt time.Time
		if i < len(timestamps) {
			if t, err := time.Parse(time.RFC3339, timestamps[i]); err == nil {
				capturedAt = t
			} else {
				capturedAt = time.Now().UTC()
			}
		} else {
			capturedAt = time.Now().UTC()
		}

		wg.Add(1)
		go func(idx int, fileHeader *multipart.FileHeader, ts time.Time) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			file, err := fileHeader.Open()
			if err != nil {
				mu.Lock()
				results[idx] = BatchCaptureResult{Index: idx, Error: "failed to open file"}
				failed++
				mu.Unlock()
				return
			}
			defer file.Close()

			imageData, err := io.ReadAll(io.LimitReader(file, maxScreenshotSize+1))
			if err != nil {
				mu.Lock()
				results[idx] = BatchCaptureResult{Index: idx, Error: "failed to read file"}
				failed++
				mu.Unlock()
				return
			}
			if len(imageData) > maxScreenshotSize {
				mu.Lock()
				results[idx] = BatchCaptureResult{Index: idx, Error: "screenshot exceeds 10 MB limit"}
				failed++
				mu.Unlock()
				return
			}
			if !isValidImage(imageData) {
				mu.Lock()
				results[idx] = BatchCaptureResult{Index: idx, Error: "invalid image format, expected PNG or JPEG"}
				failed++
				mu.Unlock()
				return
			}

			meta, err := h.interpreter.Interpret(imageData, deviceID.(string), ts)
			if err != nil {
				mu.Lock()
				results[idx] = BatchCaptureResult{Index: idx, Error: err.Error()}
				failed++
				mu.Unlock()
				return
			}

			mu.Lock()
			results[idx] = BatchCaptureResult{
				Index:          idx,
				ID:             meta.ID,
				Interpretation: meta.Interpretation,
				Category:       meta.Category,
				AppName:        meta.AppName,
			}
			succeeded++
			mu.Unlock()
		}(i, fh, capturedAt)
	}

	wg.Wait()

	log.Printf("Batch capture: device %s — %d screenshots, %d succeeded, %d failed", deviceID, len(files), succeeded, failed)

	status := http.StatusOK
	if failed > 0 && succeeded > 0 {
		status = http.StatusMultiStatus
	} else if failed > 0 && succeeded == 0 {
		status = http.StatusInternalServerError
	}

	c.JSON(status, BatchCaptureResponse{
		Results:   results,
		Succeeded: succeeded,
		Failed:    failed,
	})
}
