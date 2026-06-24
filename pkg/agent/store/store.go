package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	pkgdb "github.com/mobazha/mobazha3.0/pkg/database"
	"github.com/mobazha/mobazha3.0/pkg/redact"
	"gorm.io/gorm"
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
	ListThreads(ctx context.Context, tenantID string, limit, offset int) ([]*Thread, error)
	LoadMessages(ctx context.Context, tenantID, threadID string) ([]*Message, error)
	DeleteThread(ctx context.Context, tenantID, threadID string) error
}

// GormPersistence stores agent runtime state in the tenant-scoped node DB.
type GormPersistence struct {
	db pkgdb.Database
}

// NewGormPersistence creates a durable agent runtime persistence adapter.
func NewGormPersistence(db pkgdb.Database) *GormPersistence {
	return &GormPersistence{db: db}
}

// MigrateModels creates or updates the agent runtime tables.
func MigrateModels(db pkgdb.Database) error {
	return db.Update(func(tx pkgdb.Tx) error {
		for _, model := range []interface{}{&Thread{}, &Turn{}, &Message{}} {
			if err := tx.Migrate(model); err != nil {
				return err
			}
		}
		return nil
	})
}

// SaveThread persists an agent thread.
func (p *GormPersistence) SaveThread(_ context.Context, t *Thread) error {
	if p == nil || p.db == nil {
		return nil
	}
	if t == nil {
		return fmt.Errorf("agent store: thread is nil")
	}
	now := time.Now()
	if t.CreatedAt.IsZero() {
		t.CreatedAt = now
	}
	if t.LastActive.IsZero() {
		t.LastActive = now
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(t)
	})
}

// SaveTurn persists an agent turn.
func (p *GormPersistence) SaveTurn(_ context.Context, t *Turn) error {
	if p == nil || p.db == nil {
		return nil
	}
	if t == nil {
		return fmt.Errorf("agent store: turn is nil")
	}
	if t.StartedAt.IsZero() {
		t.StartedAt = time.Now()
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(t)
	})
}

// SaveMessage persists a redacted agent message.
func (p *GormPersistence) SaveMessage(_ context.Context, m *Message) error {
	if p == nil || p.db == nil {
		return nil
	}
	if m == nil {
		return fmt.Errorf("agent store: message is nil")
	}
	cp := sanitizeMessage(*m)
	if cp.CreatedAt.IsZero() {
		cp.CreatedAt = time.Now()
	}
	if cp.Bytes == 0 {
		cp.Bytes = len(cp.Content)
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		return tx.Save(&cp)
	})
}

// LoadThread loads a tenant-scoped agent thread.
func (p *GormPersistence) LoadThread(_ context.Context, tenantID, threadID string) (*Thread, error) {
	if p == nil || p.db == nil {
		return nil, ErrThreadNotFound
	}
	var thread Thread
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().Where("id = ?", threadID).First(&thread).Error
	})
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrThreadNotFound
		}
		return nil, err
	}
	return &thread, nil
}

// ListThreads returns recent tenant-scoped agent threads.
func (p *GormPersistence) ListThreads(_ context.Context, tenantID string, limit, offset int) ([]*Thread, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	if limit <= 0 {
		limit = 20
	}
	var records []Thread
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().
			Order("last_active DESC").
			Limit(limit).
			Offset(offset).
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Thread, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// LoadMessages loads a tenant-scoped thread history, oldest first.
func (p *GormPersistence) LoadMessages(_ context.Context, tenantID, threadID string) ([]*Message, error) {
	if p == nil || p.db == nil {
		return nil, nil
	}
	var records []Message
	err := p.db.View(func(tx pkgdb.Tx) error {
		return tx.Read().
			Where("thread_id = ?", threadID).
			Order("created_at ASC, id ASC").
			Find(&records).Error
	})
	if err != nil {
		return nil, err
	}
	out := make([]*Message, len(records))
	for i := range records {
		out[i] = &records[i]
	}
	return out, nil
}

// DeleteThread removes a thread and all its turn/message rows.
func (p *GormPersistence) DeleteThread(_ context.Context, tenantID, threadID string) error {
	if p == nil || p.db == nil {
		return nil
	}
	return p.db.Update(func(tx pkgdb.Tx) error {
		if err := tx.Delete("thread_id", threadID, nil, &Message{}); err != nil {
			return err
		}
		if err := tx.Delete("thread_id", threadID, nil, &Turn{}); err != nil {
			return err
		}
		return tx.Delete("id", threadID, nil, &Thread{})
	})
}

func sanitizeMessage(m Message) Message {
	m.Content = sanitizeJSONText(m.Content)
	m.ToolCalls = sanitizeToolCalls(m.ToolCalls)
	return m
}

func sanitizeJSONText(content string) string {
	var obj map[string]any
	if err := json.Unmarshal([]byte(content), &obj); err == nil {
		return redact.RedactMapJSON(obj)
	}

	var arr []map[string]any
	if err := json.Unmarshal([]byte(content), &arr); err == nil {
		for i := range arr {
			arr[i] = redact.RedactMap(arr[i])
		}
		if data, err := json.Marshal(arr); err == nil {
			return string(data)
		}
	}
	return content
}

func sanitizeToolCalls(raw string) string {
	if raw == "" {
		return ""
	}
	var calls []map[string]any
	if err := json.Unmarshal([]byte(raw), &calls); err != nil {
		return raw
	}
	for _, call := range calls {
		if args, ok := call["arguments"].(string); ok {
			call["arguments"] = sanitizeJSONText(args)
		}
	}
	data, err := json.Marshal(calls)
	if err != nil {
		return raw
	}
	return string(data)
}

// RuntimeStore is an in-memory cache for active thread state and messages.
// Provides fast reads during a turn without hitting the database.
// Keys are composite (tenantID, threadID) to prevent cross-tenant leakage.
type RuntimeStore struct {
	mu       sync.RWMutex
	threads  map[string]*Thread    // key: threadKey(tenantID, threadID)
	messages map[string][]*Message // key: threadKey(tenantID, threadID)
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
