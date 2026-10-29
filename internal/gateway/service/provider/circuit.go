// Package provider implements a simple circuit breaker for V1-30.
package provider

import (
	"sync"
	"sync/atomic"
	"time"
)

// State represents a circuit breaker state.
type State int32

const (
	Closed   State = 0
	Open     State = 1
	HalfOpen State = 2
)

// Breaker tracks error and timeout counts to trip.
type Breaker struct {
	mu           sync.Mutex
	state        State
	failureCount int
	lastFailure  time.Time

	// Config.
	maxFailures  int
	resetAfter   time.Duration
	halfOpenMax  int
}

// NewBreaker creates a circuit breaker.
func NewBreaker(maxFailures int, resetAfter time.Duration) *Breaker {
	return &Breaker{
		state:       Closed,
		maxFailures: maxFailures,
		resetAfter:  resetAfter,
		halfOpenMax: 3,
	}
}

// Allow returns true if the request may proceed.
func (b *Breaker) Allow() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	switch b.state {
	case Closed:
		return true
	case Open:
		if time.Since(b.lastFailure) > b.resetAfter {
			b.state = HalfOpen
			b.failureCount = 0
			return true
		}
		return false
	case HalfOpen:
		return b.failureCount < b.halfOpenMax
	}
	return false
}

// RecordResult reports success or failure.
func (b *Breaker) RecordResult(success bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if success {
		if b.state == HalfOpen {
			b.state = Closed
			b.failureCount = 0
		} else {
			b.failureCount = 0
		}
		return
	}
	b.failureCount++
	b.lastFailure = time.Now()
	if b.failureCount >= b.maxFailures {
		b.state = Open
	}
}

// State returns current state label.
func (b *Breaker) State() string {
	switch atomic.LoadInt32((*int32)(&b.state)) {
	case 0:
		return "closed"
	case 1:
		return "open"
	default:
		return "half_open"
	}
}

// ProviderBreakers holds per-provider breakers.
var ProviderBreakers sync.Map

// GetBreaker returns or creates a breaker for the given provider.
func GetBreaker(provider string) *Breaker {
	v, _ := ProviderBreakers.LoadOrStore(provider, NewBreaker(5, 30*time.Second))
	return v.(*Breaker)
}
