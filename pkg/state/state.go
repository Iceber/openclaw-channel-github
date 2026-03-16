// Package state provides idempotency and deduplication for webhook event processing.
package state

import (
	"sync"
	"time"
)

// Store manages idempotency state for processed events.
// It keeps track of delivery IDs to prevent duplicate processing.
type Store struct {
	mu      sync.Mutex
	entries map[string]time.Time
	ttl     time.Duration
}

// NewStore creates a new state Store with the given TTL for entries.
func NewStore(ttl time.Duration) *Store {
	if ttl <= 0 {
		ttl = 1 * time.Hour
	}
	return &Store{
		entries: make(map[string]time.Time),
		ttl:     ttl,
	}
}

// IsDuplicate checks whether the given delivery ID has already been processed.
// Returns true if the delivery was already seen (duplicate).
func (s *Store) IsDuplicate(deliveryID string) bool {
	if deliveryID == "" {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.entries[deliveryID]; exists {
		return true
	}
	return false
}

// MarkProcessed records a delivery ID as processed.
func (s *Store) MarkProcessed(deliveryID string) {
	if deliveryID == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.entries[deliveryID] = time.Now()
}

// Cleanup removes expired entries from the store.
func (s *Store) Cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoff := time.Now().Add(-s.ttl)
	for id, ts := range s.entries {
		if ts.Before(cutoff) {
			delete(s.entries, id)
		}
	}
}

// Size returns the current number of entries in the store.
func (s *Store) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.entries)
}

// StartCleanupLoop runs a background goroutine that periodically cleans up expired entries.
// It returns a stop function that can be called to terminate the loop.
func (s *Store) StartCleanupLoop(interval time.Duration) func() {
	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				s.Cleanup()
			case <-done:
				return
			}
		}
	}()
	return func() { close(done) }
}
