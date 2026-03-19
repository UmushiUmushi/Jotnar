// Admin provides a Unix-socket control plane for hot-reconfiguring the
// running Jotnar server without restarting. The `jotnar updateinference`
// subcommand connects to the socket and triggers a reconfiguration.
//
// The socket lives at /data/admin.sock (or $JOTNAR_ADMIN_SOCK) inside the
// container, so `docker exec jotnar jotnar updateinference` can reach it.
package admin

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/processing"
)

// SocketPath returns the admin socket path. Checks JOTNAR_ADMIN_SOCK first,
// then places the socket next to the database (same directory as JOTNAR_DB_PATH).
func SocketPath() string {
	if p := os.Getenv("JOTNAR_ADMIN_SOCK"); p != "" {
		return p
	}
	if dbPath := os.Getenv("JOTNAR_DB_PATH"); dbPath != "" {
		return filepath.Join(filepath.Dir(dbPath), "admin.sock")
	}
	return "./admin.sock"
}

// Request is the JSON message sent from CLI → running server.
// The CLI reads the .env and sends the config values — the server process
// can't re-read .env itself since its environment is already loaded.
type Request struct {
	Action     string `json:"action"`
	Host       string `json:"host,omitempty"`
	TimeoutSec int    `json:"timeout_sec,omitempty"`
	MaxRetries int    `json:"max_retries,omitempty"`
	Workers    int    `json:"workers,omitempty"`
}

// Response is the JSON message sent from running server → CLI.
// The server may send multiple responses for long-running operations.
type Response struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Server listens on a Unix socket and handles admin commands.
type Server struct {
	socketPath string
	listener   net.Listener

	swappable *inference.SwappableClient
	queue     *processing.Queue
}

// NewServer creates an admin server. Call ListenAndServe to start.
func NewServer(swappable *inference.SwappableClient, queue *processing.Queue) *Server {
	return &Server{
		socketPath: SocketPath(),
		swappable:  swappable,
		queue:      queue,
	}
}

// ListenAndServe starts the admin socket. Blocks until ctx is cancelled.
func (s *Server) ListenAndServe(ctx context.Context) error {
	// Remove stale socket from a previous run.
	os.Remove(s.socketPath)

	var err error
	s.listener, err = net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("admin listen: %w", err)
	}
	os.Chmod(s.socketPath, 0660)

	log.Printf("[admin] listening on %s", s.socketPath)

	go func() {
		<-ctx.Done()
		s.listener.Close()
	}()

	for {
		conn, err := s.listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			log.Printf("[admin] accept error: %v", err)
			continue
		}
		go s.handle(conn)
	}
}

func (s *Server) handle(conn net.Conn) {
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(15 * time.Minute))

	var req Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		writeResponse(conn, Response{OK: false, Message: "invalid request: " + err.Error()})
		return
	}

	switch req.Action {
	case "update_inference":
		s.handleUpdateInference(conn, req)
	case "toggle_debug_log":
		s.handleToggleDebugLog(conn)
	default:
		writeResponse(conn, Response{OK: false, Message: "unknown action: " + req.Action})
	}
}

func (s *Server) handleUpdateInference(conn net.Conn, req Request) {
	// Start from the server's current config, then overlay any fields the
	// CLI explicitly sent (non-zero/non-empty means "change this").
	cfg := inference.DefaultClientConfig()
	workers := inference.DefaultWorkers()

	if req.Host != "" {
		cfg.Host = req.Host
		os.Setenv("INFERENCE_HOST", req.Host)
	}
	if req.TimeoutSec > 0 {
		cfg.Timeout = time.Duration(req.TimeoutSec) * time.Second
		os.Setenv("INFERENCE_TIMEOUT_SEC", fmt.Sprintf("%d", req.TimeoutSec))
	}
	if req.MaxRetries > 0 {
		cfg.MaxRetries = req.MaxRetries
		os.Setenv("INFERENCE_MAX_RETRIES", fmt.Sprintf("%d", req.MaxRetries))
	}
	if req.Workers > 0 {
		workers = req.Workers
		os.Setenv("INFERENCE_WORKERS", fmt.Sprintf("%d", req.Workers))
	}

	log.Printf("[admin] update_inference: host=%s workers=%d timeout=%s", cfg.Host, workers, cfg.Timeout)

	// Step 1: Pause workers — waits for in-flight jobs to finish.
	s.queue.Pause()
	msg := fmt.Sprintf("Workers paused (%d jobs buffered). Applying new config: host=%s, workers=%d, timeout=%s",
		s.queue.Pending(), cfg.Host, workers, cfg.Timeout)
	log.Printf("[admin] %s", msg)
	writeResponse(conn, Response{OK: true, Message: msg})

	// Step 2: Resize worker pool.
	s.queue.SetWorkers(workers)

	// Step 3: Wait for the inference server and detect backend type.
	log.Printf("[admin] Waiting for inference server at %s ...", cfg.Host)
	writeResponse(conn, Response{OK: true, Message: "Waiting for inference server..."})

	backendCh := make(chan string, 1)
	go func() {
		backendCh <- inference.DetectBackend(cfg.Host)
	}()

	select {
	case backend := <-backendCh:
		// Step 4: Create new client with detected backend and swap it in.
		var newClient inference.Client
		if backend == "ollama" {
			newClient = inference.NewOllamaClient(cfg)
		} else {
			newClient = inference.NewOpenAIClient(cfg)
		}
		s.swappable.Swap(newClient)

		s.queue.Resume()
		msg := fmt.Sprintf("Inference updated and healthy (backend: %s). Workers resumed (%d workers, %d buffered jobs).",
			backend, s.queue.Workers(), s.queue.Pending())
		log.Printf("[admin] %s", msg)
		writeResponse(conn, Response{OK: true, Message: msg})

	case <-time.After(10 * time.Minute):
		// Resume anyway so the queue isn't stuck forever.
		s.queue.Resume()
		msg := "Timed out waiting for inference server (10m). Workers resumed — jobs will fail until the server is back."
		log.Printf("[admin] %s", msg)
		writeResponse(conn, Response{OK: false, Message: msg})
	}
}

func (s *Server) handleToggleDebugLog(conn net.Conn) {
	was := inference.DebugLog()
	inference.SetDebugLog(!was)
	state := "off"
	if !was {
		state = "on"
	}
	msg := fmt.Sprintf("Inference debug logging toggled %s", state)
	log.Printf("[admin] %s", msg)
	writeResponse(conn, Response{OK: true, Message: msg})
}

func writeResponse(conn net.Conn, resp Response) {
	json.NewEncoder(conn).Encode(resp)
}
