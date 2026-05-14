package adapters

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// TestMemoryActionStore_LookupNotFound enforces the not-found
// contract: empty + unknown ActionIDs both surface
// ErrActionRecordNotFound. The two cases share a return path so a
// caller cannot tell them apart, which matches the relay_tasks
// production semantics (an empty primary-key query is just a
// 0-row select, not a parameter validation error).
func TestMemoryActionStore_LookupNotFound(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	cases := []string{"", "non-existent", "00000000-0000-0000-0000-000000000000"}
	for _, id := range cases {
		_, err := store.Lookup(context.Background(), id)
		if !errors.Is(err, ErrActionRecordNotFound) {
			t.Errorf("Lookup(%q) err = %v, want ErrActionRecordNotFound", id, err)
		}
	}
}

// TestMemoryActionStore_PutThenLookup is the happy-path round trip:
// a record inserted via Put MUST be visible to Lookup with all
// fields preserved verbatim. Verifies the defensive-copy semantics
// (mutating the returned record does not leak back into the store).
func TestMemoryActionStore_PutThenLookup(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	original := ActionRecord{
		ActionID:      "01HW00000000000000000000A1",
		OrderID:       "order-d16-01",
		Action:        "confirm",
		ChainID:       1,
		State:         "submitted",
		TxHash:        "0xabc",
		Confirmations: 3,
		LastError:     "",
		CreatedAt:     time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Put(original); err != nil {
		t.Fatalf("Put err = %v", err)
	}
	if got, want := store.Len(), 1; got != want {
		t.Errorf("Len = %d, want %d", got, want)
	}

	got, err := store.Lookup(context.Background(), original.ActionID)
	if err != nil {
		t.Fatalf("Lookup err = %v", err)
	}
	if got.OrderID != original.OrderID {
		t.Errorf("OrderID = %q, want %q", got.OrderID, original.OrderID)
	}
	if got.Action != original.Action {
		t.Errorf("Action = %q, want %q", got.Action, original.Action)
	}
	if got.ChainID != original.ChainID {
		t.Errorf("ChainID = %d, want %d", got.ChainID, original.ChainID)
	}
	if got.State != original.State {
		t.Errorf("State = %q, want %q", got.State, original.State)
	}
	if got.TxHash != original.TxHash {
		t.Errorf("TxHash = %q, want %q", got.TxHash, original.TxHash)
	}
	if got.Confirmations != original.Confirmations {
		t.Errorf("Confirmations = %d, want %d", got.Confirmations, original.Confirmations)
	}
	if !got.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", got.CreatedAt, original.CreatedAt)
	}
	// UpdatedAt MUST be set by Put (UTC, non-zero).
	if got.UpdatedAt.IsZero() {
		t.Errorf("UpdatedAt is zero, want non-zero (Put should bump it)")
	}

	// Mutating the returned copy MUST NOT affect the stored record.
	got.State = "confirmed"
	got.Confirmations = 99
	again, err := store.Lookup(context.Background(), original.ActionID)
	if err != nil {
		t.Fatalf("second Lookup err = %v", err)
	}
	if again.State != "submitted" || again.Confirmations != 3 {
		t.Errorf("returned record was not a defensive copy: stored State=%q Confirmations=%d, want submitted/3", again.State, again.Confirmations)
	}
}

// TestMemoryActionStore_PutOverwrite documents the upsert behavior:
// a second Put with the same ActionID replaces the State / TxHash
// / Confirmations but preserves the original CreatedAt — mirroring
// the relay writer's UPDATE statement on relay_tasks.
func TestMemoryActionStore_PutOverwrite(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	original := ActionRecord{
		ActionID:  "id-overwrite",
		State:     "pending",
		CreatedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := store.Put(original); err != nil {
		t.Fatalf("first Put: %v", err)
	}
	first, _ := store.Lookup(context.Background(), original.ActionID)

	// Sleep just enough for UpdatedAt to advance (1ms is plenty on
	// any platform that gives time.Now() better-than-1ms resolution).
	time.Sleep(2 * time.Millisecond)

	// Second Put: caller passes CreatedAt zero; store should preserve
	// the original timestamp.
	overwrite := ActionRecord{
		ActionID:      "id-overwrite",
		State:         "submitted",
		TxHash:        "0xfeed",
		Confirmations: 1,
	}
	if err := store.Put(overwrite); err != nil {
		t.Fatalf("second Put: %v", err)
	}

	again, err := store.Lookup(context.Background(), overwrite.ActionID)
	if err != nil {
		t.Fatalf("Lookup after overwrite: %v", err)
	}
	if again.State != "submitted" || again.TxHash != "0xfeed" {
		t.Errorf("overwrite did not apply: State=%q TxHash=%q", again.State, again.TxHash)
	}
	if !again.CreatedAt.Equal(original.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v (overwrite should preserve original)", again.CreatedAt, original.CreatedAt)
	}
	if !again.UpdatedAt.After(first.UpdatedAt) {
		t.Errorf("UpdatedAt = %v, want strictly after %v (overwrite should bump it)", again.UpdatedAt, first.UpdatedAt)
	}
	if got, want := store.Len(), 1; got != want {
		t.Errorf("Len = %d, want %d (overwrite, not insert)", got, want)
	}
}

// TestMemoryActionStore_PutRejectsEmptyID — empty ActionID would
// silently shadow Lookup("") which already maps to not-found. Reject
// at insert time so the store never holds an unaddressable record.
func TestMemoryActionStore_PutRejectsEmptyID(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	err := store.Put(ActionRecord{ActionID: ""})
	if err == nil {
		t.Fatalf("Put(empty ActionID) err = nil, want non-nil")
	}
	if store.Len() != 0 {
		t.Errorf("Len = %d, want 0 (empty ID should not be stored)", store.Len())
	}
}

// TestMemoryActionStore_LookupHonorsContext verifies ctx cancellation
// before the map read. Relay status polling fan-outs cancel ctx
// when the parent deadline trips; the store must propagate it.
func TestMemoryActionStore_LookupHonorsContext(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	_ = store.Put(ActionRecord{ActionID: "id-1", State: "pending"})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := store.Lookup(ctx, "id-1")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("Lookup err = %v, want context.Canceled", err)
	}
}

// TestMemoryActionStore_Reset wipes the store between subtests.
// Relay-poller integration tests share an adapter; Reset is the
// hook they use to keep test data isolated.
func TestMemoryActionStore_Reset(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	_ = store.Put(ActionRecord{ActionID: "a", State: "pending"})
	_ = store.Put(ActionRecord{ActionID: "b", State: "pending"})
	store.Reset()
	if got, want := store.Len(), 0; got != want {
		t.Errorf("Len after Reset = %d, want %d", got, want)
	}
	_, err := store.Lookup(context.Background(), "a")
	if !errors.Is(err, ErrActionRecordNotFound) {
		t.Errorf("err = %v, want ErrActionRecordNotFound after Reset", err)
	}
}

// TestMemoryActionStore_ConcurrentLookup smokes out a regression in
// the RLock/Lock pairing: 100 goroutines reading and writing the
// same store MUST NOT race or deadlock. Race-detector picks up any
// regression; this test is the trigger.
func TestMemoryActionStore_ConcurrentLookup(t *testing.T) {
	t.Parallel()

	store := NewMemoryActionStore()
	const N = 100
	var wg sync.WaitGroup
	wg.Add(N * 2)

	for i := 0; i < N; i++ {
		i := i
		go func() {
			defer wg.Done()
			_ = store.Put(ActionRecord{
				ActionID: idFor(i),
				State:    "pending",
			})
		}()
		go func() {
			defer wg.Done()
			_, _ = store.Lookup(context.Background(), idFor(i))
		}()
	}
	wg.Wait()
	if got := store.Len(); got > N {
		t.Errorf("Len = %d, want <= %d", got, N)
	}
}

func idFor(i int) string {
	return "id-" + string(rune('a'+i%26)) + "-" + string(rune('a'+(i/26)%26))
}
