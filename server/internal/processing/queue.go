// Background processing queue for screenshots.
// Accepts screenshots immediately and processes them sequentially
// so the HTTP handler can return without waiting for inference.
package processing

import (
	"context"
	"log"
	"sync"
	"time"
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
	workers     int
}

// NewQueue creates a processing queue with the given buffer and worker count.
// Workers controls how many images are interpreted concurrently.
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

// Run starts worker goroutines that process jobs until the context is cancelled.
// Call this in a goroutine.
func (q *Queue) Run(ctx context.Context) {
	log.Printf("Processing queue started (workers: %d, buffer: %d)", q.workers, cap(q.jobs))
	var wg sync.WaitGroup
	for i := range q.workers {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case job := <-q.jobs:
					meta, err := q.interpreter.Interpret(job.ImageData, job.DeviceID, job.CapturedAt)
					if err != nil {
						log.Printf("Queue[%d]: interpretation failed for device %s (image size: %d bytes): %v", workerID, job.DeviceID, len(job.ImageData), err)
						continue
					}
					log.Printf("Queue[%d]: device %s — %s (%s) [%d pending]", workerID, job.DeviceID, meta.AppName, meta.Category, len(q.jobs))
				}
			}
		}(i)
	}
	wg.Wait()
	log.Printf("Processing queue shut down (%d jobs remaining)", len(q.jobs))
}
