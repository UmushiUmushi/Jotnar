// POST /capture — receive screenshot from device.
package routes

import (
	"bytes"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/api/middleware"
	"github.com/jotnar/server/internal/processing"
)

type CaptureAcceptedResponse struct {
	Accepted int `json:"accepted"`
}

type BatchCaptureResult struct {
	Index int    `json:"index"`
	Error string `json:"error,omitempty"`
}

type BatchCaptureAcceptedResponse struct {
	Accepted int                  `json:"accepted"`
	Rejected int                  `json:"rejected"`
	Results  []BatchCaptureResult `json:"results,omitempty"`
}

type CaptureHandler struct {
	queue *processing.Queue
}

func NewCaptureHandler(queue *processing.Queue) *CaptureHandler {
	return &CaptureHandler{queue: queue}
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

	if !h.queue.Enqueue(processing.CaptureJob{
		ImageData:  imageData,
		DeviceID:   deviceID.(string),
		CapturedAt: capturedAt,
	}) {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "processing queue is full"})
		return
	}

	log.Printf("Capture: queued screenshot from device %s (%d pending)", deviceID, h.queue.Pending())
	c.JSON(http.StatusAccepted, CaptureAcceptedResponse{Accepted: 1})
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

	accepted, rejected := 0, 0
	var rejectedResults []BatchCaptureResult

	for i, fh := range files {
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

		imageData, err := readFileHeader(fh)
		if err != nil {
			rejected++
			rejectedResults = append(rejectedResults, BatchCaptureResult{Index: i, Error: err.Error()})
			continue
		}

		if !h.queue.Enqueue(processing.CaptureJob{
			ImageData:  imageData,
			DeviceID:   deviceID.(string),
			CapturedAt: capturedAt,
		}) {
			rejected++
			rejectedResults = append(rejectedResults, BatchCaptureResult{Index: i, Error: "processing queue is full"})
			continue
		}

		accepted++
	}

	log.Printf("Batch capture: device %s — %d accepted, %d rejected (%d pending)", deviceID, accepted, rejected, h.queue.Pending())
	c.JSON(http.StatusAccepted, BatchCaptureAcceptedResponse{
		Accepted: accepted,
		Rejected: rejected,
		Results:  rejectedResults,
	})
}

// readFileHeader reads and validates a multipart file.
func readFileHeader(fh *multipart.FileHeader) ([]byte, error) {
	file, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer file.Close()

	imageData, err := io.ReadAll(io.LimitReader(file, maxScreenshotSize+1))
	if err != nil {
		return nil, err
	}
	if len(imageData) > maxScreenshotSize {
		return nil, io.ErrUnexpectedEOF
	}
	if !isValidImage(imageData) {
		return nil, bytes.ErrTooLarge // reuse stdlib error for "invalid format"
	}
	return imageData, nil
}
