package state

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *SQLiteStore {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")

	store, err := NewSQLiteStore(path)
	if err != nil {
		t.Fatalf("create store: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestSQLiteStore_PutAndGet(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	rs := &ResourceState{
		Kind:       "Agent",
		Key:        "my-bot",
		ExternalID: "uuid-123",
		SpecHash:   "abc123",
		Synced:     true,
		LastSync:   now,
	}

	if err := store.Put(rs); err != nil {
		t.Fatalf("put: %v", err)
	}

	got, err := store.Get("Agent", "my-bot")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got == nil {
		t.Fatal("expected non-nil state")
	}
	if got.ExternalID != "uuid-123" {
		t.Errorf("expected uuid-123, got %s", got.ExternalID)
	}
	if got.SpecHash != "abc123" {
		t.Errorf("expected abc123, got %s", got.SpecHash)
	}
	if !got.Synced {
		t.Error("expected synced=true")
	}
}

func TestSQLiteStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)

	got, err := store.Get("Agent", "nonexistent")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got != nil {
		t.Error("expected nil for nonexistent resource")
	}
}

func TestSQLiteStore_List(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	for _, key := range []string{"a", "b", "c"} {
		store.Put(&ResourceState{Kind: "Agent", Key: key, SpecHash: "h", LastSync: now})
	}

	states, err := store.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(states) != 3 {
		t.Errorf("expected 3 states, got %d", len(states))
	}
}

func TestSQLiteStore_Delete(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	store.Put(&ResourceState{Kind: "Agent", Key: "x", SpecHash: "h", LastSync: now})

	if err := store.Delete("Agent", "x"); err != nil {
		t.Fatalf("delete: %v", err)
	}

	got, _ := store.Get("Agent", "x")
	if got != nil {
		t.Error("expected nil after delete")
	}
}

func TestSQLiteStore_Upsert(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	store.Put(&ResourceState{Kind: "Agent", Key: "bot", SpecHash: "v1", LastSync: now})
	store.Put(&ResourceState{Kind: "Agent", Key: "bot", SpecHash: "v2", LastSync: now})

	got, _ := store.Get("Agent", "bot")
	if got.SpecHash != "v2" {
		t.Errorf("expected v2, got %s", got.SpecHash)
	}

	states, _ := store.List()
	if len(states) != 1 {
		t.Errorf("expected 1 state after upsert, got %d", len(states))
	}
}

func TestSQLiteStore_ErrorField(t *testing.T) {
	store := newTestStore(t)

	now := time.Now().Truncate(time.Second)
	store.Put(&ResourceState{
		Kind: "Agent", Key: "err-bot", SpecHash: "h",
		LastSync: now, Error: "connection timeout",
	})

	got, _ := store.Get("Agent", "err-bot")
	if got.Error != "connection timeout" {
		t.Errorf("expected error field, got %q", got.Error)
	}
}

func TestSQLiteStore_InvalidPath(t *testing.T) {
	_, err := NewSQLiteStore(filepath.Join(os.TempDir(), "nonexistent-dir-gcplane", "sub", "test.db"))
	if err == nil {
		t.Error("expected error for invalid path")
	}
}
