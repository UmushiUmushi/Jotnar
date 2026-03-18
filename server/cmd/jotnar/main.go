// Entry point for the Jotnar server. Starts the API server and background worker.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/jotnar/server/internal/api"
	"github.com/jotnar/server/internal/auth"
	"github.com/jotnar/server/internal/config"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/processing"
	"github.com/jotnar/server/internal/store"
)

const version = "0.1.0"

func openDB() *sql.DB {
	dbPath := os.Getenv("JOTNAR_DB_PATH")
	if dbPath == "" {
		dbPath = "./journal.db"
	}
	database, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	return database
}

func cmdPairingCode() {
	database := openDB()
	defer database.Close()

	pairingService := auth.NewPairingService(database)
	code, err := pairingService.GenerateCode()
	if err != nil {
		log.Fatalf("Failed to generate pairing code: %v", err)
	}
	fmt.Println("============================================")
	fmt.Printf("  Pairing code: %s\n", code)
	fmt.Printf("  (expires in 10 minutes)\n")
	fmt.Println("============================================")
}

func main() {
	// Load .env file if present (no error if missing)
	_ = godotenv.Load()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "pairingcode":
			cmdPairingCode()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\nUsage: jotnar [pairingcode]\n", os.Args[1])
			os.Exit(1)
		}
	}

	// Database
	database := openDB()
	defer database.Close()

	// Config
	configPath := os.Getenv("JOTNAR_CONFIG_PATH")
	if configPath == "" {
		configPath = "./config.yml"
	}
	cfgManager, err := config.NewManager(configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Inference client
	infClient := inference.NewClient(inference.DefaultClientConfig())

	// Core services
	tokenService := auth.NewTokenService(database)
	pairingService := auth.NewPairingService(database)
	recoveryService := auth.NewRecoveryService(database)

	deviceStore := store.NewDeviceStore(database)
	journalStore := store.NewJournalStore(database)
	metadataStore := store.NewMetadataStore(database)

	interpreter := processing.NewInterpreter(infClient, cfgManager, metadataStore)
	inferenceWorkers := 1
	if v := os.Getenv("INFERENCE_WORKERS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			inferenceWorkers = n
		}
	}
	captureQueue := processing.NewQueue(interpreter, 200, inferenceWorkers)
	consolidator := processing.NewConsolidator(infClient, cfgManager, metadataStore, journalStore)
	reconsolidator := processing.NewReconsolidator(consolidator, cfgManager, metadataStore, journalStore)

	// First-time setup: generate pairing code + recovery key if no devices
	hasPaired, err := pairingService.HasPairedDevices()
	if err != nil {
		log.Fatalf("Failed to check paired devices: %v", err)
	}
	if !hasPaired {
		code, err := pairingService.GenerateCode()
		if err != nil {
			log.Fatalf("Failed to generate pairing code: %v", err)
		}
		fmt.Println("============================================")
		fmt.Printf("  FIRST-TIME SETUP\n")
		fmt.Printf("  Pairing code: %s\n", code)
		fmt.Printf("  (expires in 10 minutes)\n")

		recoveryKey, err := recoveryService.GenerateRecoveryKey()
		if err != nil {
			log.Fatalf("Failed to generate recovery key: %v", err)
		}
		fmt.Printf("  Recovery key: %s\n", recoveryKey)
		fmt.Println("  SAVE THIS KEY — you cannot retrieve it later!")
		fmt.Println("============================================")
	}

	// Shutdown context — cancelled on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background processing queue (interprets screenshots sequentially)
	go captureQueue.Run(ctx)

	// Background consolidation worker
	go func() {
		interval := time.Duration(cfgManager.Get().ConsolidationWindowMin) * time.Minute
		log.Printf("Consolidation worker started (interval: %s)", interval)
		timer := time.NewTimer(interval)
		defer timer.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Printf("Consolidation worker shutting down")
				return
			case <-timer.C:
				log.Printf("Consolidation: running cycle")
				if err := consolidator.Run(); err != nil {
					log.Printf("Consolidation error: %v", err)
				}
				// Enforce metadata retention if configured.
				cfg := cfgManager.Get()
				if cfg.MetadataRetentionDays != nil && *cfg.MetadataRetentionDays > 0 {
					cutoff := time.Now().UTC().AddDate(0, 0, -*cfg.MetadataRetentionDays)
					if _, err := metadataStore.DeleteOlderThan(cutoff); err != nil {
						log.Printf("Metadata retention cleanup error: %v", err)
					}
				}
				newInterval := time.Duration(cfg.ConsolidationWindowMin) * time.Minute
				if newInterval != interval {
					log.Printf("Consolidation: interval changed from %s to %s", interval, newInterval)
					interval = newInterval
				}
				timer.Reset(interval)
			}
		}
	}()

	// API server
	server := api.NewServer(api.Dependencies{
		TokenService:    tokenService,
		PairingService:  pairingService,
		RecoveryService: recoveryService,
		ConfigManager:   cfgManager,
		InferenceClient: infClient,
		JournalStore:    journalStore,
		MetadataStore:   metadataStore,
		DeviceStore:     deviceStore,
		Queue:           captureQueue,
		Interpreter:     interpreter,
		Consolidator:    consolidator,
		Reconsolidator:  reconsolidator,
		Version:         version,
	})

	httpServer := &http.Server{
		Addr:    ":8910",
		Handler: server.Router,
	}

	// Start HTTP server in a goroutine.
	go func() {
		log.Printf("Jotnar server v%s starting on :8910", version)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for shutdown signal.
	<-ctx.Done()
	log.Printf("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}
	log.Printf("Server stopped")
}
