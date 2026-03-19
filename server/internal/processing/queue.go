// Background processing queue for screenshots.
// Accepts screenshots immediately and processes them via a pool of workers
// so the HTTP handler can return without waiting for inference.
//
// The queue supports:
//   - Pause/Resume for hot-reconfiguration without losing buffered jobs
//   - Persist/Restore for surviving container restarts
package processing

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jotnar/server/internal/store"
)

// CaptureJob represents a screenshot waiting to be interpreted.
type CaptureJob struct {
	ImageData  []byte
	DeviceID   string
	CapturedAt time.Time
}

// Queue accepts capture jobs and processes them in the background.
type Queue struct {
	jobs        chan CaptureJob
	interpreter *Interpreter

	mu           sync.Mutex
	workers      int
	parentCtx    context.Context
	workerCancel context.CancelFunc
	wg           sync.WaitGroup
}

// NewQueue creates a processing queue with the given buffer and worker count.
func NewQueue(interpreter *Interpreter, bufferSize int, workers int) *Queue {
	if bufferSize <= 0 {
		bufferSize = 100
	}
	if workers <= 0 {
		workers = 1
	}
	return &Queue{
		jobs:        make(chan CaptureJob, bufferSize),
		interpreter: interpreter,
		workers:     workers,
	}
}

// Enqueue adds a screenshot to the processing queue.
// Returns false if the queue is full.
func (q *Queue) Enqueue(job CaptureJob) bool {
	select {
	case q.jobs <- job:
		return true
	default:
		return false
	}
}

// Pending returns the number of jobs waiting in the queue.
func (q *Queue) Pending() int {
	return len(q.jobs)
}

// Workers returns the current worker count.
func (q *Queue) Workers() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.workers
}

// Run starts workers and blocks until the parent context is cancelled.
// Call this in a goroutine.
func (q *Queue) Run(ctx context.Context) {
	q.mu.Lock()
	q.parentCtx = ctx
	q.mu.Unlock()

	q.spawnWorkers()

	// Block until shutdown signal.
	<-ctx.Done()

	// Cancel workers and wait for in-flight jobs to finish.
	q.mu.Lock()
	if q.workerCancel != nil {
		q.workerCancel()
	}
	q.mu.Unlock()
	q.wg.Wait()
	log.Printf("Processing queue shut down (%d jobs remaining)", len(q.jobs))
}

// Pause stops all workers after their current job finishes.
// Buffered jobs stay in the channel and are not lost.
// Blocks until all workers have exited.
func (q *Queue) Pause() {
	q.mu.Lock()
	cancel := q.workerCancel
	q.workerCancel = nil
	q.mu.Unlock()

	if cancel != nil {
		cancel()
	}
	q.wg.Wait()
	log.Printf("Processing queue paused (%d jobs buffered)", len(q.jobs))
}

// Resume restarts the worker pool. No-op if the parent context is already done.
func (q *Queue) Resume() {
	q.mu.Lock()
	if q.parentCtx == nil || q.parentCtx.Err() != nil {
		q.mu.Unlock()
		return
	}
	q.mu.Unlock()

	q.spawnWorkers()
}

// SetWorkers changes the worker count. Takes effect on the next Resume().
func (q *Queue) SetWorkers(n int) {
	if n <= 0 {
		n = 1
	}
	q.mu.Lock()
	q.workers = n
	q.mu.Unlock()
}

// Drain removes all buffered jobs from the channel and returns them.
// Call after Pause() to persist jobs before shutdown.
func (q *Queue) Drain() []CaptureJob {
	var jobs []CaptureJob
	for {
		select {
		case job := <-q.jobs:
			jobs = append(jobs, job)
		default:
			return jobs
		}
	}
}

// Persist drains the queue and saves pending jobs to the database.
// Call during graceful shutdown to survive container restarts.
func (q *Queue) Persist(pendingStore *store.PendingStore) int {
	jobs := q.Drain()
	if len(jobs) == 0 {
		return 0
	}

	pending := make([]store.PendingCapture, len(jobs))
	for i, j := range jobs {
		pending[i] = store.PendingCapture{
			ID:         uuid.New().String(),
			DeviceID:   j.DeviceID,
			ImageData:  j.ImageData,
			CapturedAt: j.CapturedAt,
			CreatedAt:  time.Now().UTC(),
		}
	}

	if err := pendingStore.SaveAll(pending); err != nil {
		log.Printf("Failed to persist %d pending jobs: %v", len(jobs), err)
		return 0
	}
	return len(pending)
}

// Restore loads persisted jobs from the database back into the queue.
// Call on startup before Run().
func (q *Queue) Restore(pendingStore *store.PendingStore) int {
	jobs, err := pendingStore.LoadAll()
	if err != nil {
		log.Printf("Failed to restore pending jobs: %v", err)
		return 0
	}

	restored := 0
	for _, j := range jobs {
		if q.Enqueue(CaptureJob{
			ImageData:  j.ImageData,
			DeviceID:   j.DeviceID,
			CapturedAt: j.CapturedAt,
		}) {
			restored++
		}
	}
	return restored
}

func (q *Queue) spawnWorkers() {
	q.mu.Lock()
	if q.parentCtx == nil || q.parentCtx.Err() != nil {
		q.mu.Unlock()
		return
	}
	ctx, cancel := context.WithCancel(q.parentCtx)
	q.workerCancel = cancel
	n := q.workers
	q.mu.Unlock()

	log.Printf("Processing queue started (workers: %d, buffer: %d)", n, cap(q.jobs))
	for i := range n {
		q.wg.Add(1)
		go q.runWorker(ctx, i)
	}
}

func (q *Queue) runWorker(ctx context.Context, id int) {
	defer q.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case job := <-q.jobs:
			meta, err := q.interpreter.Interpret(job.ImageData, job.DeviceID, job.CapturedAt)
			if err != nil {
				log.Printf("Queue[%d]: interpretation failed for device %s (image size: %d bytes): %v", id, job.DeviceID, len(job.ImageData), err)
				continue
			}
			log.Printf("Queue[%d]: device %s — %s (%s) [%d pending]", id, job.DeviceID, meta.AppName, meta.Category, len(q.jobs))
		}
	}
}
