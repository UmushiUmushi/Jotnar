// GET/PUT/DELETE /journal — journal entry CRUD.
package routes

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/store"
)

type JournalEntryResponse struct {
	ID        string     `json:"id"`
	Narrative string     `json:"narrative"`
	TimeStart time.Time  `json:"time_start"`
	TimeEnd   time.Time  `json:"time_end"`
	Edited    bool       `json:"edited"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt *time.Time `json:"updated_at,omitempty"`
}

type JournalListResponse struct {
	Entries []JournalEntryResponse `json:"entries"`
	Total   int                    `json:"total"`
	Limit   int                    `json:"limit"`
	Offset  int                    `json:"offset"`
}

type UpdateJournalRequest struct {
	Narrative string `json:"narrative" binding:"required"`
}

type JournalHandler struct {
	journal *store.JournalStore
}

func NewJournalHandler(journal *store.JournalStore) *JournalHandler {
	return &JournalHandler{journal: journal}
}

func (h *JournalHandler) List(c *gin.Context) {
	limit := 20
	offset := 0

	if l, err := strconv.Atoi(c.DefaultQuery("limit", "20")); err == nil && l > 0 {
		limit = l
	}
	if limit > 100 {
		limit = 100
	}
	if o, err := strconv.Atoi(c.DefaultQuery("offset", "0")); err == nil && o >= 0 {
		offset = o
	}

	total, err := h.journal.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to count entries"})
		return
	}

	entries, err := h.journal.ListPaginated(limit, offset)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list entries"})
		return
	}

	resp := make([]JournalEntryResponse, len(entries))
	for i, e := range entries {
		resp[i] = JournalEntryResponse{
			ID:        e.ID,
			Narrative: e.Narrative,
			TimeStart: e.TimeStart,
			TimeEnd:   e.TimeEnd,
			Edited:    e.Edited,
			CreatedAt: e.CreatedAt,
			UpdatedAt: e.UpdatedAt,
		}
	}

	c.JSON(http.StatusOK, JournalListResponse{
		Entries: resp,
		Total:   total,
		Limit:   limit,
		Offset:  offset,
	})
}

func (h *JournalHandler) Get(c *gin.Context) {
	id := c.Param("id")
	entry, err := h.journal.GetByID(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	c.JSON(http.StatusOK, JournalEntryResponse{
		ID:        entry.ID,
		Narrative: entry.Narrative,
		TimeStart: entry.TimeStart,
		TimeEnd:   entry.TimeEnd,
		Edited:    entry.Edited,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	})
}

func (h *JournalHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req UpdateJournalRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.journal.UpdateNarrative(id, req.Narrative, true); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "entry not found"})
		return
	}

	entry, _ := h.journal.GetByID(id)
	c.JSON(http.StatusOK, JournalEntryResponse{
		ID:        entry.ID,
		Narrative: entry.Narrative,
		TimeStart: entry.TimeStart,
		TimeEnd:   entry.TimeEnd,
		Edited:    entry.Edited,
		CreatedAt: entry.CreatedAt,
		UpdatedAt: entry.UpdatedAt,
	})
}

func (h *JournalHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.journal.Delete(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete entry"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}
