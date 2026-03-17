// HTTP server setup and router configuration.
package api

import (
	"github.com/gin-gonic/gin"
	"github.com/jotnar/server/internal/api/middleware"
	"github.com/jotnar/server/internal/api/routes"
	"github.com/jotnar/server/internal/auth"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/processing"
	"github.com/jotnar/server/internal/store"
)

type Server struct {
	Router *gin.Engine
}

type Dependencies struct {
	TokenService   *auth.TokenService
	PairingService *auth.PairingService
	RecoveryService *auth.RecoveryService
	ConfigManager  *config.Manager
	InferenceClient *inference.Client
	JournalStore   *store.JournalStore
	MetadataStore  *store.MetadataStore
	DeviceStore    *store.DeviceStore
	Interpreter    *processing.Interpreter
	Consolidator   *processing.Consolidator
	Reconsolidator *processing.Reconsolidator
	Version        string
}

func NewServer(deps Dependencies) *Server {
	router := gin.Default()

	// Handlers
	statusHandler := routes.NewStatusHandler(deps.InferenceClient, deps.DeviceStore, deps.Version)
	authHandler := routes.NewAuthHandler(deps.PairingService, deps.RecoveryService)
	journalHandler := routes.NewJournalHandler(deps.JournalStore)
	captureHandler := routes.NewCaptureHandler(deps.Interpreter)
	configHandler := routes.NewConfigHandler(deps.ConfigManager)
	metadataHandler := routes.NewMetadataHandler(deps.MetadataStore, deps.Reconsolidator)

	// Public routes
	router.GET("/status", statusHandler.GetStatus)
	router.POST("/auth/pair", authHandler.Pair)
	router.POST("/auth/recover", authHandler.Recover)

	// Authenticated routes
	authed := router.Group("/")
	authed.Use(middleware.TokenAuth(deps.TokenService))
	{
		authed.POST("/capture", captureHandler.Capture)
		authed.POST("/capture/batch", captureHandler.BatchCapture)

		authed.GET("/journal", journalHandler.List)
		authed.GET("/journal/:id", journalHandler.Get)
		authed.PUT("/journal/:id", journalHandler.Update)
		authed.DELETE("/journal/:id", journalHandler.Delete)

		authed.GET("/journal/:id/metadata", metadataHandler.GetByEntry)
		authed.POST("/journal/:id/preview", metadataHandler.Preview)
		authed.POST("/journal/:id/reconsolidate", metadataHandler.Reconsolidate)

		authed.GET("/config", configHandler.GetConfig)
		authed.PUT("/config", configHandler.UpdateConfig)

		authed.POST("/auth/pair/new", authHandler.PairNew)
	}

	return &Server{Router: router}
}
