package processing

import (
	"context"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/jotnar/server/internal/inference"
	"github.com/jotnar/server/internal/store"
)

func TestNewQueue_Defaults(t *testing.T) {
	q := NewQueue(nil, 0, 0)
	if cap(q.jobs) != 100 {
		t.Errorf("buffer = %d, want 100", cap(q.jobs))
	}
	if q.workers != 1 {
		t.Errorf("workers = %d, want 1", q.workers)
	}
}

func TestNewQueue_CustomValues(t *testing.T) {
	q := NewQueue(nil, 50, 8)
	if cap(q.jobs) != 50 {
		t.Errorf("buffer = %d, want 50", cap(q.jobs))
	}
	if q.workers != 8 {
		t.Errorf("workers = %d, want 8", q.workers)
	}
}

func TestEnqueue_Success(t *testing.T) {
	q := NewQueue(nil, 10, 1)
	ok := q.Enqueue(CaptureJob{DeviceID: "d1"})
	if !ok {
		t.Fatal("Enqueue should succeed on empty queue")
	}
	if q.Pending() != 1 {
		t.Errorf("Pending = %d, want 1", q.Pending())
	}
}

func TestEnqueue_Full(t *testing.T) {
	q := NewQueue(nil, 2, 1)
	q.Enqueue(CaptureJob{DeviceID: "d1"})
	q.Enqueue(CaptureJob{DeviceID: "d2"})

	ok := q.Enqueue(CaptureJob{DeviceID: "d3"})
	if ok {
		t.Fatal("Enqueue should return false when queue is full")
	}
	if q.Pending() != 2 {
		t.Errorf("Pending = %d, want 2", q.Pending())
	}
}

func TestDrain(t *testing.T) {
	q := NewQueue(nil, 10, 1)
	q.Enqueue(CaptureJob{DeviceID: "d1"})
	q.Enqueue(CaptureJob{DeviceID: "d2"})
	q.Enqueue(CaptureJob{DeviceID: "d3"})

	jobs := q.Drain()
	if len(jobs) != 3 {
		t.Fatalf("drained = %d, want 3", len(jobs))
	}
	if q.Pending() != 0 {
		t.Errorf("Pending after drain = %d, want 0", q.Pending())
	}
	if jobs[0].DeviceID != "d1" || jobs[1].DeviceID != "d2" || jobs[2].DeviceID != "d3" {
		t.Error("drained jobs should be in FIFO order")
	}
}

func TestDrain_Empty(t *testing.T) {
	q := NewQueue(nil, 10, 1)
	jobs := q.Drain()
	if jobs != nil {
		t.Errorf("drain of empty queue = %v, want nil", jobs)
	}
}

func TestSetWorkers(t *testing.T) {
	q := NewQueue(nil, 10, 2)
	q.SetWorkers(8)
	if q.Workers() != 8 {
		t.Errorf("Workers = %d, want 8", q.Workers())
	}
}

func TestSetWorkers_FloorAtOne(t *testing.T) {
	q := NewQueue(nil, 10, 4)
	q.SetWorkers(0)
	if q.Workers() != 1 {
		t.Errorf("Workers = %d, want 1 (floor)", q.Workers())
	}
	q.SetWorkers(-5)
	if q.Workers() != 1 {
		t.Errorf("Workers = %d, want 1 (floor for negative)", q.Workers())
	}
}

func TestPauseResume(t *testing.T) {
	ts := mockInterpretationServer("test", "other", "TestApp")
	defer ts.Close()

	db := testDB(t)
	cfg := testConfig(t)
	client := newTestInferenceClient(t, ts.URL)
	metaStore := store.NewMetadataStore(db)
	deviceID := insertTestDevice(t, db)
	interp := NewInterpreter(client, cfg, metaStore)

	q := NewQueue(interp, 100, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go q.Run(ctx)
	// Give workers time to start.
	time.Sleep(50 * time.Millisecond)

	// Enqueue a job, let it process.
	q.Enqueue(CaptureJob{
		ImageData:  minimalPNG(),
		DeviceID:   deviceID,
		CapturedAt: time.Now(),
	})
	time.Sleep(200 * time.Millisecond)

	// Pause workers.
	q.Pause()

	// Enqueue while paused — should buffer.
	q.Enqueue(CaptureJob{
		ImageData:  minimalPNG(),
		DeviceID:   deviceID,
		CapturedAt: time.Now(),
	})
	if q.Pending() != 1 {
		t.Errorf("Pending while paused = %d, want 1", q.Pending())
	}

	// Resume — buffered job should get processed.
	q.Resume()
	time.Sleep(200 * time.Millisecond)

	if q.Pending() != 0 {
		t.Errorf("Pending after resume = %d, want 0", q.Pending())
	}

}

func TestResumeAfterShutdown_Noop(t *testing.T) {
	q := NewQueue(nil, 10, 1)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		q.Run(ctx)
		close(done)
	}()
	time.Sleep(50 * time.Millisecond)

	cancel()
	<-done

	// Resume after parent context done should be a no-op.
	q.Resume() // should not panic
}

func TestPersistAndRestore(t *testing.T) {
	db := testDB(t)
	pendingStore := store.NewPendingStore(db)
	q := NewQueue(nil, 10, 1)

	now := time.Now().UTC().Truncate(time.Second)
	q.Enqueue(CaptureJob{ImageData: []byte("img1"), DeviceID: "d1", CapturedAt: now, AppName: "App1"})
	q.Enqueue(CaptureJob{ImageData: []byte("img2"), DeviceID: "d2", CapturedAt: now.Add(time.Minute), AppName: "App2"})

	persisted := q.Persist(pendingStore)
	if persisted != 2 {
		t.Fatalf("persisted = %d, want 2", persisted)
	}
	if q.Pending() != 0 {
		t.Errorf("Pending after persist = %d, want 0", q.Pending())
	}

	// Restore into a fresh queue.
	q2 := NewQueue(nil, 10, 1)
	restored := q2.Restore(pendingStore)
	if restored != 2 {
		t.Fatalf("restored = %d, want 2", restored)
	}
	if q2.Pending() != 2 {
		t.Errorf("Pending after restore = %d, want 2", q2.Pending())
	}

	// Verify job data survived the round-trip.
	jobs := q2.Drain()
	if string(jobs[0].ImageData) != "img1" {
		t.Errorf("job[0].ImageData = %q, want img1", jobs[0].ImageData)
	}
	if jobs[0].DeviceID != "d1" {
		t.Errorf("job[0].DeviceID = %q, want d1", jobs[0].DeviceID)
	}
	if jobs[0].AppName != "App1" {
		t.Errorf("job[0].AppName = %q, want App1", jobs[0].AppName)
	}
	if jobs[1].DeviceID != "d2" {
		t.Errorf("job[1].DeviceID = %q, want d2", jobs[1].DeviceID)
	}
}

func TestPersist_EmptyQueue(t *testing.T) {
	db := testDB(t)
	pendingStore := store.NewPendingStore(db)
	q := NewQueue(nil, 10, 1)

	persisted := q.Persist(pendingStore)
	if persisted != 0 {
		t.Errorf("persisted = %d, want 0", persisted)
	}
}

func TestRestore_DropsWhenFull(t *testing.T) {
	db := testDB(t)
	pendingStore := store.NewPendingStore(db)

	// Persist 3 jobs.
	q := NewQueue(nil, 10, 1)
	for i := range 3 {
		q.Enqueue(CaptureJob{ImageData: []byte{byte(i)}, DeviceID: "d1", CapturedAt: time.Now()})
	}
	q.Persist(pendingStore)

	// Restore into a queue with buffer size 2 — one job should be dropped.
	q2 := NewQueue(nil, 2, 1)
	restored := q2.Restore(pendingStore)
	if restored != 2 {
		t.Errorf("restored = %d, want 2 (buffer full, third dropped)", restored)
	}
}

func TestRun_Shutdown(t *testing.T) {
	q := NewQueue(nil, 10, 2)
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		q.Run(ctx)
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// Clean shutdown.
	case <-time.After(2 * time.Second):
		t.Fatal("Run did not shut down within 2 seconds")
	}
}

// --- helpers ---

func minimalPNG() []byte {
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
}

func newTestInferenceClient(t *testing.T, url string) *inference.OpenAIClient {
	t.Helper()
	return inference.NewOpenAIClient(inference.ClientConfig{Host: url, MaxRetries: 1})
}
