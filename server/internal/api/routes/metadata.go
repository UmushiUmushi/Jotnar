// GET /journal/{id}/metadata, POST /journal/{id}/preview, POST /journal/{id}/reconsolidate.
package routes

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/processing"
	"github.com/jotnar/server/internal/store"
)

type MetadataResponse struct {
	ID             string    `json:"id"`
	DeviceID       string    `json:"device_id"`
	CapturedAt     time.Time `json:"captured_at"`
	Interpretation string    `json:"interpretation"`
	Category       string    `json:"category"`
	AppName        string    `json:"app_name"`
	CreatedAt      time.Time `json:"created_at"`
}

type MetadataListResponse struct {
	Metadata []MetadataResponse `json:"metadata"`
}

type ReconsolidateRequest struct {
	IncludeMetadataIDs []string `json:"include_metadata_ids" binding:"required"`
	Narrative          string   `json:"narrative,omitempty"`
}

type PreviewResponse struct {
	Narrative string `json:"narrative"`
}

type MetadataHandler struct {
	metadata       *store.MetadataStore
	reconsolidator *processing.Reconsolidator
}

func NewMetadataHandler(metadata *store.MetadataStore, reconsolidator *processing.Reconsolidator) *MetadataHandler {
	return &MetadataHandler{metadata: metadata, reconsolidator: reconsolidator}
}

func (h *MetadataHandler) GetByEntry(c *gin.Context) {
	entryID := c.Param("id")
	rows, err := h.metadata.GetByEntryID(entryID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch metadata"})
		return
	}

	resp := make([]MetadataResponse, len(rows))
	for i, m := range rows {
		resp[i] = MetadataResponse{
			ID:             m.ID,
			DeviceID:       m.DeviceID,
			CapturedAt:     m.CapturedAt,
			Interpretation: m.Interpretation,
			Category:       m.Category,
			AppName:        m.AppName,
			CreatedAt:      m.CreatedAt,
		}
	}

	c.JSON(http.StatusOK, MetadataListResponse{Metadata: resp})
}

func (h *MetadataHandler) Preview(c *gin.Context) {
	var req ReconsolidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	narrative, err := h.reconsolidator.Preview(req.IncludeMetadataIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "preview failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, PreviewResponse{Narrative: narrative})
}

func (h *MetadataHandler) Reconsolidate(c *gin.Context) {
	entryID := c.Param("id")
	var req ReconsolidateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	entry, err := h.reconsolidator.Commit(entryID, req.IncludeMetadataIDs, req.Narrative)
	if err != nil {
		log.Printf("Reconsolidation: failed for entry %s: %v", entryID, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "reconsolidation failed: " + err.Error()})
		return
	}

	log.Printf("Reconsolidation: entry %s updated with %d metadata rows", entryID, len(req.IncludeMetadataIDs))
	c.JSON(http.StatusOK, JournalEntryResponse{
		ID:        entry.ID,
		Narrative: entry.Narrative,
		TimeStart: entry.TimeStart,
		TimeEnd:   entry.TimeEnd,
		Edited:    entry.Edited,
		UpdatedAt: entry.UpdatedAt,
	})
}
