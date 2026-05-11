package store

import (
	"context"
	"errors"
	"sync"
	"time"
)

// Common errors.
var (
	ErrThreadNotFound = errors.New("agent: thread not found")
)

// Persistence provides durable storage for agent threads, turns, and messages.
type Persistence interface {
	SaveThread(ctx context.Context, t *Thread) error
	SaveTurn(ctx context.Context, t *Turn) error
	SaveMessage(ctx context.Context, m *Message) error
	LoadThread(ctx context.Context, tenantID, threadID string) (*Thread, error)
	ListThreads(ctx context.Context, tenantID string, limit int) ([]*Thread, error)
	LoadMessages(ctx context.Context, tenantID, threadID string) ([]*Message, error)
}

// RuntimeStore is an in-memory cache for active thread state and messages.
// Provides fast reads during a turn without hitting the database.
// Keys are composite (tenantID, threadID) to prevent cross-tenant leakage.
type RuntimeStore struct {
	mu       sync.RWMutex
	threads  map[string]*Thread     // key: threadKey(tenantID, threadID)
	messages map[string][]*Message  // key: threadKey(tenantID, threadID)
}

// threadKey builds a composite map key that prevents cross-tenant collisions.
func threadKey(tenantID, threadID string) string {
	return tenantID + "\x00" + threadID
}

// NewRuntimeStore creates an empty in-memory store.
func NewRuntimeStore() *RuntimeStore {
	return &RuntimeStore{
		threads:  make(map[string]*Thread),
		messages: make(map[string][]*Message),
	}
}

// GetThread returns a defensive copy of the thread to prevent callers
// from mutating internal state without holding the lock.
func (r *RuntimeStore) GetThread(tenantID, threadID string) (*Thread, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.threads[threadKey(tenantID, threadID)]
	if !ok {
		return nil, false
	}
	cp := *t
	return &cp, true
}

func (r *RuntimeStore) UpdateThread(t *Thread) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.threads[threadKey(t.TenantID, t.ID)] = t
}

// TouchThread atomically updates the thread's LastActive timestamp.
func (r *RuntimeStore) TouchThread(tenantID, threadID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if t, ok := r.threads[threadKey(tenantID, threadID)]; ok {
		t.LastActive = time.Now()
	}
}

func (r *RuntimeStore) RemoveThread(tenantID, threadID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	delete(r.threads, k)
	delete(r.messages, k)
}

// CleanupIdle removes threads that have been inactive for longer than maxIdle.
func (r *RuntimeStore) CleanupIdle(maxIdle time.Duration) int {
	r.mu.Lock()
	defer r.mu.Unlock()
	cutoff := time.Now().Add(-maxIdle)
	removed := 0
	for id, t := range r.threads {
		if t.LastActive.Before(cutoff) {
			delete(r.threads, id)
			delete(r.messages, id)
			removed++
		}
	}
	return removed
}

func (r *RuntimeStore) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.threads)
}

// AppendMessage adds a message to the thread's in-memory history.
func (r *RuntimeStore) AppendMessage(tenantID, threadID string, m *Message) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	r.messages[k] = append(r.messages[k], m)
}

// GetMessages returns all in-memory messages for a thread, oldest first.
// Returns deep copies to prevent callers from mutating internal state.
func (r *RuntimeStore) GetMessages(tenantID, threadID string) []*Message {
	r.mu.RLock()
	defer r.mu.RUnlock()
	msgs := r.messages[threadKey(tenantID, threadID)]
	out := make([]*Message, len(msgs))
	for i, m := range msgs {
		cp := *m
		out[i] = &cp
	}
	return out
}

// TruncateMessages keeps only the last n messages for a thread (budget shaping).
func (r *RuntimeStore) TruncateMessages(tenantID, threadID string, keepLast int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	k := threadKey(tenantID, threadID)
	msgs := r.messages[k]
	if len(msgs) > keepLast {
		r.messages[k] = msgs[len(msgs)-keepLast:]
	}
}
