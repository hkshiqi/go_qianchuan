package authstate

import (
	"testing"
	"time"
)

func TestStateIsOpaqueTenantBoundAndSingleUse(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	store := newStoreWithClock(10*time.Minute, func() time.Time { return now })

	state, err := store.Issue("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	if state == "" || state == "tenant-a" {
		t.Fatalf("state must be opaque, got %q", state)
	}
	tenantID, err := store.Consume(state)
	if err != nil {
		t.Fatal(err)
	}
	if tenantID != "tenant-a" {
		t.Fatalf("tenant = %q, want tenant-a", tenantID)
	}
	if _, err := store.Consume(state); err == nil {
		t.Fatal("state was reusable")
	}
}

func TestStateCannotBeReplayedAfterExpiry(t *testing.T) {
	now := time.Date(2026, 11, 20, 12, 0, 0, 0, time.UTC)
	store := newStoreWithClock(time.Minute, func() time.Time { return now })
	state, err := store.Issue("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if _, err := store.Consume(state); err == nil {
		t.Fatal("expired state was accepted")
	}
}

func TestDiscardInvalidatesPendingState(t *testing.T) {
	store := New(0)
	state, err := store.Issue("tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	store.Discard(state)
	if _, err := store.Peek(state); err == nil {
		t.Fatal("discarded state was accepted")
	}
}
