package api

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/mobazha/mobazha/pkg/contracts"
	"github.com/mobazha/mobazha/pkg/models"
	responsePkg "github.com/mobazha/mobazha/pkg/response"
)

func getAnalyticsProvider(r *http.Request) (contracts.AnalyticsProvider, bool) {
	ap, ok := getNodeService(r).(contracts.AnalyticsProvider)
	return ap, ok
}

// analyticsRateLimiter tracks per-IP request counts using a sliding window.
// Stale entries are evicted during allow() to prevent unbounded memory growth.
type analyticsRateLimiter struct {
	mu       sync.Mutex
	entries  map[string]*rateLimitEntry
	limit    int
	window   time.Duration
	lastSweep time.Time
}

type rateLimitEntry struct {
	count   int
	resetAt time.Time
}

var analyticsRL = &analyticsRateLimiter{
	entries:   make(map[string]*rateLimitEntry),
	limit:     60,
	window:    time.Minute,
	lastSweep: time.Now(),
}

const sweepInterval = 5 * time.Minute

func (rl *analyticsRateLimiter) allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	if now.Sub(rl.lastSweep) > sweepInterval {
		for k, e := range rl.entries {
			if now.After(e.resetAt) {
				delete(rl.entries, k)
			}
		}
		rl.lastSweep = now
	}

	entry, ok := rl.entries[ip]
	if !ok || now.After(entry.resetAt) {
		rl.entries[ip] = &rateLimitEntry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	entry.count++
	return entry.count <= rl.limit
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		// First IP in the chain
		for i := 0; i < len(xff); i++ {
			if xff[i] == ',' {
				return xff[:i]
			}
		}
		return xff
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// handlePOSTAnalyticsEvent records visitor analytics events (public, rate-limited).
func (g *Gateway) handlePOSTAnalyticsEvent(w http.ResponseWriter, r *http.Request) {
	if !analyticsRL.allow(clientIP(r)) {
		responsePkg.Error(w, http.StatusTooManyRequests, "RATE_LIMITED", "Too many requests")
		return
	}

	ap, ok := getAnalyticsProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Analytics not available")
		return
	}
	svc := ap.Analytics()
	if svc == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Analytics not available")
		return
	}

	var req struct {
		Events []struct {
			EventType   string `json:"eventType"`
			SessionID   string `json:"sessionId"`
			VisitorID   string `json:"visitorId"`
			PagePath    string `json:"pagePath"`
			ProductSlug string `json:"productSlug,omitempty"`
			Referrer    string `json:"referrer,omitempty"`
		} `json:"events"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32*1024)).Decode(&req); err != nil {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeBadRequest, "Invalid request body")
		return
	}
	if len(req.Events) == 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "At least one event is required")
		return
	}

	events := make([]models.AnalyticsEvent, 0, len(req.Events))
	for _, e := range req.Events {
		if !models.ValidEventTypes[e.EventType] {
			continue
		}
		if e.SessionID == "" || e.VisitorID == "" {
			continue
		}
		events = append(events, models.AnalyticsEvent{
			EventType:   e.EventType,
			SessionID:   e.SessionID,
			VisitorID:   e.VisitorID,
			PagePath:    e.PagePath,
			ProductSlug: e.ProductSlug,
			Referrer:    e.Referrer,
		})
	}

	if len(events) == 0 {
		responsePkg.Error(w, http.StatusBadRequest, responsePkg.CodeValidation, "No valid events provided")
		return
	}

	if err := svc.TrackEvents(events); err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to track events")
		return
	}

	responsePkg.NoContent(w)
}

// handleGETAnalyticsStats returns aggregated visitor statistics (authenticated).
func (g *Gateway) handleGETAnalyticsStats(w http.ResponseWriter, r *http.Request) {
	ap, ok := getAnalyticsProvider(r)
	if !ok {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Analytics not available")
		return
	}
	svc := ap.Analytics()
	if svc == nil {
		responsePkg.Error(w, http.StatusNotImplemented, responsePkg.CodeNotImplemented, "Analytics not available")
		return
	}

	days := 30
	if d := r.URL.Query().Get("days"); d != "" {
		if parsed, err := strconv.Atoi(d); err == nil && parsed > 0 && parsed <= 90 {
			days = parsed
		}
	}

	stats, err := svc.GetVisitorStats(days)
	if err != nil {
		responsePkg.Error(w, http.StatusInternalServerError, responsePkg.CodeInternalError, "Failed to retrieve analytics")
		return
	}

	responsePkg.Success(w, stats)
}
