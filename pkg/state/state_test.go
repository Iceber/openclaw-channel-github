package state

import (
	"testing"
	"time"
)

func TestStoreIsDuplicate(t *testing.T) {
	store := NewStore(time.Hour)

	// First time should not be duplicate
	if store.IsDuplicate("delivery-1") {
		t.Error("expected first delivery to not be duplicate")
	}

	// Mark as processed
	store.MarkProcessed("delivery-1")

	// Second time should be duplicate
	if !store.IsDuplicate("delivery-1") {
		t.Error("expected second delivery to be duplicate")
	}

	// Different delivery should not be duplicate
	if store.IsDuplicate("delivery-2") {
		t.Error("expected different delivery to not be duplicate")
	}
}

func TestStoreEmptyDeliveryID(t *testing.T) {
	store := NewStore(time.Hour)

	// Empty delivery ID should never be duplicate
	if store.IsDuplicate("") {
		t.Error("expected empty delivery ID to not be duplicate")
	}

	// Marking empty should be a no-op
	store.MarkProcessed("")
	if store.Size() != 0 {
		t.Errorf("expected size 0 after marking empty ID, got %d", store.Size())
	}
}

func TestStoreCleanup(t *testing.T) {
	store := NewStore(100 * time.Millisecond)

	store.MarkProcessed("delivery-1")
	if store.Size() != 1 {
		t.Errorf("expected size 1, got %d", store.Size())
	}

	// Wait for TTL to expire
	time.Sleep(200 * time.Millisecond)
	store.Cleanup()

	if store.Size() != 0 {
		t.Errorf("expected size 0 after cleanup, got %d", store.Size())
	}
}

func TestStoreSize(t *testing.T) {
	store := NewStore(time.Hour)

	store.MarkProcessed("d1")
	store.MarkProcessed("d2")
	store.MarkProcessed("d3")

	if store.Size() != 3 {
		t.Errorf("expected size 3, got %d", store.Size())
	}
}

func TestStoreDefaultTTL(t *testing.T) {
	store := NewStore(0) // should default to 1 hour
	store.MarkProcessed("d1")

	// Should still be present (not expired)
	if !store.IsDuplicate("d1") {
		t.Error("expected delivery to be present with default TTL")
	}
}
