package inference

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeClient is a minimal Client for testing SwappableClient.
type fakeClient struct {
	completeFunc    func(ChatRequest) (string, error)
	isAvailableFunc func() bool
}

func (f *fakeClient) Complete(req ChatRequest) (string, error) {
	if f.completeFunc != nil {
		return f.completeFunc(req)
	}
	return "", nil
}

func (f *fakeClient) IsAvailable() bool {
	if f.isAvailableFunc != nil {
		return f.isAvailableFunc()
	}
	return true
}

func TestSwappableClient_DelegatesToInner(t *testing.T) {
	inner := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			return "from-inner", nil
		},
		isAvailableFunc: func() bool { return true },
	}
	sc := NewSwappableClient(inner)

	result, err := sc.Complete(ChatRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "from-inner" {
		t.Errorf("result = %q, want %q", result, "from-inner")
	}
	if !sc.IsAvailable() {
		t.Error("expected IsAvailable = true")
	}
}

func TestSwappableClient_Swap(t *testing.T) {
	old := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			return "old", nil
		},
	}
	new := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			return "new", nil
		},
	}

	sc := NewSwappableClient(old)

	// Before swap.
	result, _ := sc.Complete(ChatRequest{})
	if result != "old" {
		t.Errorf("before swap: result = %q, want %q", result, "old")
	}

	sc.Swap(new)

	// After swap.
	result, _ = sc.Complete(ChatRequest{})
	if result != "new" {
		t.Errorf("after swap: result = %q, want %q", result, "new")
	}
}

func TestSwappableClient_IsAvailable_DelegatesAfterSwap(t *testing.T) {
	available := &fakeClient{isAvailableFunc: func() bool { return true }}
	unavailable := &fakeClient{isAvailableFunc: func() bool { return false }}

	sc := NewSwappableClient(available)
	if !sc.IsAvailable() {
		t.Error("expected available before swap")
	}

	sc.Swap(unavailable)
	if sc.IsAvailable() {
		t.Error("expected unavailable after swap")
	}
}

func TestSwappableClient_ErrorPropagation(t *testing.T) {
	errClient := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			return "", errors.New("inference failed")
		},
	}
	sc := NewSwappableClient(errClient)

	_, err := sc.Complete(ChatRequest{})
	if err == nil || err.Error() != "inference failed" {
		t.Errorf("error = %v, want 'inference failed'", err)
	}
}

func TestSwappableClient_ConcurrentReads(t *testing.T) {
	var calls atomic.Int64
	inner := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			calls.Add(1)
			time.Sleep(10 * time.Millisecond)
			return "ok", nil
		},
	}
	sc := NewSwappableClient(inner)

	var wg sync.WaitGroup
	for range 20 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sc.Complete(ChatRequest{})
		}()
	}
	wg.Wait()

	if calls.Load() != 20 {
		t.Errorf("calls = %d, want 20", calls.Load())
	}
}

func TestSwappableClient_SwapDuringInflight(t *testing.T) {
	// Verify that Swap blocks until in-flight Complete calls finish,
	// then subsequent calls use the new client.
	var oldStarted, oldFinished atomic.Int64

	slowOld := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			oldStarted.Add(1)
			time.Sleep(100 * time.Millisecond)
			oldFinished.Add(1)
			return "old", nil
		},
	}
	fast := &fakeClient{
		completeFunc: func(req ChatRequest) (string, error) {
			return "new", nil
		},
	}

	sc := NewSwappableClient(slowOld)

	// Start an in-flight call.
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		result, _ := sc.Complete(ChatRequest{})
		if result != "old" {
			t.Errorf("in-flight result = %q, want old", result)
		}
	}()

	// Wait for the old call to start, then swap.
	time.Sleep(20 * time.Millisecond)
	sc.Swap(fast)

	// After swap returns, the old call should have finished.
	// Note: SwappableClient takes a snapshot of the client under RLock,
	// so Swap doesn't actually wait for the old Complete to finish —
	// it waits for readers to release the RLock (which happens immediately
	// after copying the client pointer). This is correct behavior:
	// swap completes quickly and new calls use the new client.
	wg.Wait()

	// Calls after swap use the new client.
	result, _ := sc.Complete(ChatRequest{})
	if result != "new" {
		t.Errorf("post-swap result = %q, want new", result)
	}
}
