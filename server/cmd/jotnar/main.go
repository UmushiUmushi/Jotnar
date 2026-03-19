// Entry point for the Jotnar server. Starts the API server and background worker.
package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/jotnar/server/internal/admin"
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

func cmdDebugLog() {
	req := admin.Request{Action: "toggle_debug_log"}
	if err := admin.SendCommand(req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func cmdUpdateInference(args []string) {
	fs := flag.NewFlagSet("updateinference", flag.ExitOnError)
	host := fs.String("host", "", "Inference server URL")
	workers := fs.Int("workers", 0, "Concurrent interpretation workers")
	timeout := fs.Int("timeout", 0, "Inference timeout in seconds")
	retries := fs.Int("retries", 0, "Max retry attempts for transient failures")
	fs.Parse(args)

	// Track which flags were explicitly set on the CLI.
	flagsSet := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) { flagsSet[f.Name] = true })

	var req admin.Request
	req.Action = "update_inference"

	if len(flagsSet) > 0 {
		// Selective mode: only send fields the user explicitly passed.
		// Zero/empty values in the request mean "don't change".
		req.Host = *host
		req.Workers = *workers
		req.TimeoutSec = *timeout
		req.MaxRetries = *retries

		fmt.Println("Updating inference configuration (from flags)...")
	} else {
		// No flags: reload everything from environment.
		cfg := inference.DefaultClientConfig()
		req.Host = cfg.Host
		req.TimeoutSec = int(cfg.Timeout.Seconds())
		req.MaxRetries = cfg.MaxRetries
		req.Workers = inference.DefaultWorkers()

		fmt.Println("Updating inference configuration (from environment)...")
	}

	if req.Host != "" {
		fmt.Printf("  Host:       %s\n", req.Host)
	}
	if req.Workers > 0 {
		fmt.Printf("  Workers:    %d\n", req.Workers)
	}
	if req.TimeoutSec > 0 {
		fmt.Printf("  Timeout:    %ds\n", req.TimeoutSec)
	}
	if req.MaxRetries > 0 {
		fmt.Printf("  MaxRetries: %d\n", req.MaxRetries)
	}

	if err := admin.SendCommand(req); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func main() {
	// Load .env file if present (no error if missing)
	_ = godotenv.Load()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "pairingcode":
			cmdPairingCode()
			return
		case "updateinference":
			cmdUpdateInference(os.Args[2:])
			return
		case "debuglog":
			cmdDebugLog()
			return
		default:
			fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
			fmt.Fprintf(os.Stderr, "Usage: jotnar [pairingcode|updateinference|debuglog]\n")
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

	// Inference client — wrapped in SwappableClient for hot-reload.
	// WaitAndCreateClient blocks until the backend is reachable, then
	// auto-detects the backend type by probing the server.
	rawClient := inference.WaitAndCreateClient(inference.DefaultClientConfig())
	infClient := inference.NewSwappableClient(rawClient)

	// Core services
	tokenService := auth.NewTokenService(database)
	pairingService := auth.NewPairingService(database)
	recoveryService := auth.NewRecoveryService(database)

	deviceStore := store.NewDeviceStore(database)
	journalStore := store.NewJournalStore(database)
	metadataStore := store.NewMetadataStore(database)
	pendingStore := store.NewPendingStore(database)

	interpreter := processing.NewInterpreter(infClient, cfgManager, metadataStore)
	captureQueue := processing.NewQueue(interpreter, 200, inference.DefaultWorkers())
	consolidator := processing.NewConsolidator(infClient, cfgManager, metadataStore, journalStore)
	reconsolidator := processing.NewReconsolidator(consolidator, cfgManager, metadataStore, journalStore)

	// Restore any pending jobs from a previous shutdown.
	if restored := captureQueue.Restore(pendingStore); restored > 0 {
		log.Printf("Restored %d pending capture jobs from previous run", restored)
	}

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

	// The queue gets its own context so we control its shutdown explicitly
	// (pause workers → persist → then let it exit) rather than racing with ctx.
	queueCtx, queueStop := context.WithCancel(context.Background())

	// Admin socket — started early so `updateinference` works even while
	// waiting for the inference server on first boot.
	adminServer := admin.NewServer(infClient, captureQueue)
	go func() {
		if err := adminServer.ListenAndServe(ctx); err != nil {
			log.Printf("Admin server error: %v", err)
		}
	}()

	// Background processing queue (interprets screenshots concurrently)
	go captureQueue.Run(queueCtx)
	_ = queueStop // used in shutdown sequence below

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

	// 1. Stop accepting new HTTP requests (no new captures enqueued).
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("Server shutdown error: %v", err)
	}

	// 2. Pause workers — waits for in-flight interpretation to finish.
	captureQueue.Pause()

	// 3. Persist remaining buffered jobs so they survive the restart.
	if saved := captureQueue.Persist(pendingStore); saved > 0 {
		log.Printf("Persisted %d pending capture jobs for next startup", saved)
	}

	// 4. Let the queue goroutine exit cleanly.
	queueStop()

	log.Printf("Server stopped")
}

