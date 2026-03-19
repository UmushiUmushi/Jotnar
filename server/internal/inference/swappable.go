// SwappableClient wraps any inference Client and allows hot-swapping the
// underlying implementation without restarting the server. All consumers
// (Interpreter, Consolidator, etc.) hold a reference to the SwappableClient,
// so swapping the inner client transparently reroutes all inference calls.
package inference

import "sync"

// SwappableClient delegates to an inner Client behind a RWMutex.
// Reads (Complete, IsAvailable) take a read lock; Swap takes a write lock.
// This means in-flight inference calls finish before the swap completes.
type SwappableClient struct {
	mu     sync.RWMutex
	client Client
}

// NewSwappableClient wraps an existing Client.
func NewSwappableClient(c Client) *SwappableClient {
	return &SwappableClient{client: c}
}

func (s *SwappableClient) Complete(req ChatRequest) (string, error) {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	return c.Complete(req)
}

func (s *SwappableClient) IsAvailable() bool {
	s.mu.RLock()
	c := s.client
	s.mu.RUnlock()
	return c.IsAvailable()
}

// Swap replaces the underlying Client. Blocks until all in-flight calls
// on the old client have returned (write lock waits for readers to drain).
func (s *SwappableClient) Swap(c Client) {
	s.mu.Lock()
	s.client = c
	s.mu.Unlock()
}
