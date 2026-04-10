package ai

import (
	"fmt"
	"sync"
	"time"
)

// DailyRateLimiter tracks per-tenant daily AI call counts in memory.
// Counters reset at midnight UTC. Not persisted across restarts (acceptable for V1).
type DailyRateLimiter struct {
	mu       sync.Mutex
	counters map[string]*dayCounter
}

type dayCounter struct {
	date  string // "2006-01-02"
	count int
}

func NewDailyRateLimiter() *DailyRateLimiter {
	return &DailyRateLimiter{
		counters: make(map[string]*dayCounter),
	}
}

// Allow checks whether tenantID can make another call within the daily limit.
// Returns (allowed, currentCount, error). When limit <= 0, always allows.
func (r *DailyRateLimiter) Allow(tenantID string, limit int) (bool, int) {
	if limit <= 0 {
		return true, 0
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	dc := r.counters[tenantID]
	if dc == nil || dc.date != today {
		dc = &dayCounter{date: today, count: 0}
		r.counters[tenantID] = dc
	}

	if dc.count >= limit {
		return false, dc.count
	}
	return true, dc.count
}

// Increment records one AI call for the tenant.
func (r *DailyRateLimiter) Increment(tenantID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	dc := r.counters[tenantID]
	if dc == nil || dc.date != today {
		dc = &dayCounter{date: today, count: 0}
		r.counters[tenantID] = dc
	}
	dc.count++
}

// Usage returns the current daily count for a tenant.
func (r *DailyRateLimiter) Usage(tenantID string) int {
	r.mu.Lock()
	defer r.mu.Unlock()

	today := time.Now().UTC().Format("2006-01-02")
	dc := r.counters[tenantID]
	if dc == nil || dc.date != today {
		return 0
	}
	return dc.count
}

// ErrRateLimited is returned when a tenant exceeds the daily platform AI limit.
var ErrRateLimited = fmt.Errorf("daily platform AI limit exceeded")
