// Package answercache implements session-level result prefetching (V1-19).
// When a user asks a question, the system asynchronously prefetches likely
// follow-up questions and their retrieval results, warming the cache.
package answercache

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// PrefetchConfig controls per-session prefetch behavior.
type PrefetchConfig struct {
	Enabled        bool          `json:"enabled"`
	MaxPerSession  int           `json:"max_per_session"`   // max prefetch slots per session
	MaxConcurrency int           `json:"max_concurrency"`    // concurrent prefetch goroutines
	TTL            time.Duration `json:"ttl"`                // TTL for prefetched results
	LowPriorityTTL time.Duration `json:"low_priority_ttl"`   // shorter TTL for speculative hits
}

// PrefetchSession tracks inflight and completed prefetches for a session.
type PrefetchSession struct {
	SessionID string
	Max       int
	Inflight  int
	Completed int
	CreatedAt time.Time
	mu        sync.Mutex
	expired   bool
}

// PrefetchManager manages per-session speculating prefetch.
type PrefetchManager struct {
	cfg      PrefetchConfig
	mu       sync.RWMutex
	sessions map[string]*PrefetchSession
	logger   *slog.Logger
}

// DefaultPrefetchConfig returns reasonable prefetch defaults.
func DefaultPrefetchConfig() PrefetchConfig {
	return PrefetchConfig{
		Enabled:        false, // disabled by default, enabled after shadow calibration
		MaxPerSession:  5,
		MaxConcurrency: 3,
		TTL:            5 * time.Minute,
		LowPriorityTTL: 2 * time.Minute,
	}
}

// NewPrefetchManager creates a session prefetch manager.
func NewPrefetchManager(cfg PrefetchConfig) *PrefetchManager {
	if cfg.MaxPerSession <= 0 {
		cfg.MaxPerSession = 5
	}
	if cfg.MaxConcurrency <= 0 {
		cfg.MaxConcurrency = 3
	}
	return &PrefetchManager{
		cfg:      cfg,
		sessions: make(map[string]*PrefetchSession),
		logger:   slog.With("component", "prefetch"),
	}
}

// Enabled reports whether prefetching is active.
func (pm *PrefetchManager) Enabled() bool {
	return pm != nil && pm.cfg.Enabled
}

// ReserveSlot returns true if the session has room for another prefetch.
func (pm *PrefetchManager) ReserveSlot(sessionID string) bool {
	if pm == nil || !pm.cfg.Enabled {
		return false
	}
	pm.mu.RLock()
	s, ok := pm.sessions[sessionID]
	pm.mu.RUnlock()
	if ok {
		s.mu.Lock()
		defer s.mu.Unlock()
		if s.expired || s.Inflight >= s.Max {
			return false
		}
		s.Inflight++
		return true
	}
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok2 := pm.sessions[sessionID]; ok2 {
		// double-check after write lock
		return pm.ReserveSlot(sessionID)
	}
	pm.sessions[sessionID] = &PrefetchSession{
		SessionID: sessionID,
		Max:       pm.cfg.MaxPerSession,
		CreatedAt: time.Now(),
		Inflight:  1,
	}
	return true
}

// ReleaseSlot decrements the inflight count for a session.
func (pm *PrefetchManager) ReleaseSlot(sessionID string, completed bool) {
	if pm == nil {
		return
	}
	pm.mu.RLock()
	s, ok := pm.sessions[sessionID]
	pm.mu.RUnlock()
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.Inflight > 0 {
		s.Inflight--
	}
	if completed {
		s.Completed++
	}
}

// GetStats returns aggregate prefetch statistics.
func (pm *PrefetchManager) GetStats() map[string]any {
	if pm == nil {
		return map[string]any{"enabled": false}
	}
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	activeSessions := 0
	totalInflight := 0
	totalCompleted := 0
	for _, s := range pm.sessions {
		s.mu.Lock()
		if s.Inflight > 0 {
			activeSessions++
		}
		totalInflight += s.Inflight
		totalCompleted += s.Completed
		s.mu.Unlock()
	}
	// Garbage collect expired sessions.
	for id := range pm.sessions {
		s := pm.sessions[id]
		_ = s
		// TODO: periodic GC goroutine
		// if time.Since(s.CreatedAt) > 1*time.Hour { delete(pm.sessions, id) }
	}
	return map[string]any{
		"enabled":           pm.cfg.Enabled,
		"sessions":          len(pm.sessions),
		"active_sessions":   activeSessions,
		"total_inflight":    totalInflight,
		"total_completed":   totalCompleted,
		"max_per_session":   pm.cfg.MaxPerSession,
		"max_concurrency":   pm.cfg.MaxConcurrency,
	}
}

// TriggerPrefetch would be called after a chat query to prefetch related results.
// The actual implementation depends on the chat service being able to spawn background
// goroutines to run retrieval queries with a session context.
type PrefetchFunc func(ctx context.Context, sessionID string, query string, kbType string) error

// RegisterPrefetchFunc would register the callback in the chat service.
var prefetchFunc PrefetchFunc

// SetPrefetchFunc sets the async prefetch trigger.
func SetPrefetchFunc(f PrefetchFunc) {
	prefetchFunc = f
}

// PrefetchAsync triggers a background prefetch for a session.
func (pm *PrefetchManager) PrefetchAsync(ctx context.Context, sessionID string, query string, kbType string) {
	if pm == nil || !pm.cfg.Enabled || prefetchFunc == nil {
		return
	}
	if !pm.ReserveSlot(sessionID) {
		return
	}
	go func() {
		err := prefetchFunc(ctx, sessionID, query, kbType)
		pm.ReleaseSlot(sessionID, err == nil)
		if err != nil {
			pm.logger.Debug("prefetch failed",
				"session_id", sessionID,
				"query", query,
				"error", err,
			)
		}
	}()
}
