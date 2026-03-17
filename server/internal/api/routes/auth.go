// POST /auth/pair, /auth/pair/new, /auth/recover — authentication routes.
package routes

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/auth"
)

type PairRequest struct {
	Code       string `json:"code" binding:"required"`
	DeviceName string `json:"device_name" binding:"required"`
}

type PairResponse struct {
	DeviceID string `json:"device_id"`
	Token    string `json:"token"`
}

type RecoverRequest struct {
	RecoveryKey string `json:"recovery_key" binding:"required"`
}

type RecoverResponse struct {
	PairingCode string `json:"pairing_code"`
}

type AuthHandler struct {
	pairing  *auth.PairingService
	recovery *auth.RecoveryService
}

func NewAuthHandler(pairing *auth.PairingService, recovery *auth.RecoveryService) *AuthHandler {
	return &AuthHandler{pairing: pairing, recovery: recovery}
}

func (h *AuthHandler) Pair(c *gin.Context) {
	var req PairRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	deviceID, token, err := h.pairing.RedeemCode(req.Code, req.DeviceName)
	if err != nil {
		log.Printf("Auth: failed pairing attempt with code %s", req.Code)
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	log.Printf("Auth: device paired — %s (ID: %s)", req.DeviceName, deviceID)
	c.JSON(http.StatusOK, PairResponse{
		DeviceID: deviceID,
		Token:    token,
	})
}

func (h *AuthHandler) PairNew(c *gin.Context) {
	code, err := h.pairing.GenerateCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate pairing code"})
		return
	}

	log.Printf("Auth: new pairing code generated")
	c.JSON(http.StatusOK, gin.H{"code": code})
}

func (h *AuthHandler) Recover(c *gin.Context) {
	var req RecoverRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.recovery.ValidateRecoveryKey(req.RecoveryKey); err != nil {
		log.Printf("Auth: failed recovery attempt")
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid recovery key"})
		return
	}

	code, err := h.pairing.GenerateCode()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to generate pairing code"})
		return
	}

	log.Printf("Auth: recovery successful, new pairing code generated")
	c.JSON(http.StatusOK, RecoverResponse{PairingCode: code})
}
