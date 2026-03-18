// GET/PUT /config — server configuration management.
package routes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/config"
)

type ConfigResponse struct {
	ConsolidationWindowMin int    `json:"consolidation_window_min"`
	InterpretationDetail   string `json:"interpretation_detail"`
	JournalTone            string `json:"journal_tone"`
	MetadataRetentionDays  *int   `json:"metadata_retention_days"`
	Timezone               string `json:"timezone"`
}

type UpdateConfigRequest struct {
	ConsolidationWindowMin *int    `json:"consolidation_window_min,omitempty"`
	InterpretationDetail   *string `json:"interpretation_detail,omitempty"`
	JournalTone            *string `json:"journal_tone,omitempty"`
	MetadataRetentionDays  *int    `json:"metadata_retention_days,omitempty"`
	Timezone               *string `json:"timezone,omitempty"`
}

type ConfigHandler struct {
	config *config.Manager
}

func NewConfigHandler(cfg *config.Manager) *ConfigHandler {
	return &ConfigHandler{config: cfg}
}

func (h *ConfigHandler) GetConfig(c *gin.Context) {
	cfg := h.config.Get()
	c.JSON(http.StatusOK, ConfigResponse{
		ConsolidationWindowMin: cfg.ConsolidationWindowMin,
		InterpretationDetail:   cfg.InterpretationDetail,
		JournalTone:            cfg.JournalTone,
		MetadataRetentionDays:  cfg.MetadataRetentionDays,
		Timezone:               cfg.Timezone,
	})
}

func (h *ConfigHandler) UpdateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := h.config.Get()

	if req.ConsolidationWindowMin != nil {
		cfg.ConsolidationWindowMin = *req.ConsolidationWindowMin
	}
	if req.InterpretationDetail != nil {
		cfg.InterpretationDetail = *req.InterpretationDetail
	}
	if req.JournalTone != nil {
		cfg.JournalTone = *req.JournalTone
	}
	if req.MetadataRetentionDays != nil {
		cfg.MetadataRetentionDays = req.MetadataRetentionDays
	}
	if req.Timezone != nil {
		cfg.Timezone = *req.Timezone
	}

	if err := h.config.Update(cfg); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to save config"})
		return
	}

	log.Printf("Config: updated — window=%dmin, detail=%s, tone=%s, tz=%s",
		cfg.ConsolidationWindowMin, cfg.InterpretationDetail, cfg.JournalTone, cfg.Timezone)
	c.JSON(http.StatusOK, ConfigResponse{
		ConsolidationWindowMin: cfg.ConsolidationWindowMin,
		InterpretationDetail:   cfg.InterpretationDetail,
		JournalTone:            cfg.JournalTone,
		MetadataRetentionDays:  cfg.MetadataRetentionDays,
		Timezone:               cfg.Timezone,
	})
}
