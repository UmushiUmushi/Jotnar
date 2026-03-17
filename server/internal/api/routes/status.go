// GET /status — server health, model status, version.
package routes

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

type StatusResponse struct {
	Version        string `json:"version"`
	ModelAvailable bool   `json:"model_available"`
	DeviceCount    int    `json:"device_count"`
}

type StatusHandler struct {
	inferenceClient *inference.Client
	deviceStore     *store.DeviceStore
	version         string
}

func NewStatusHandler(client *inference.Client, devices *store.DeviceStore, version string) *StatusHandler {
	return &StatusHandler{inferenceClient: client, deviceStore: devices, version: version}
}

func (h *StatusHandler) GetStatus(c *gin.Context) {
	count, err := h.deviceStore.Count()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database unavailable"})
		return
	}

	c.JSON(http.StatusOK, StatusResponse{
		Version:        h.version,
		ModelAvailable: h.inferenceClient.IsAvailable(),
		DeviceCount:    count,
	})
}
