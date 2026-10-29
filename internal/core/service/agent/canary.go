package agent

import (
	"hash/fnv"
	"sync"
	"time"
)

// CanaryController provides deterministic sampling and a failure-rate circuit
// breaker for candidate Runtime policies.
type CanaryController struct {
	SampleRate      float64
	MinSamples      int
	MaxFailureRate  float64
	Cooldown        time.Duration
	mu              sync.Mutex
	total, failures int
	openUntil       time.Time
}

func (c *CanaryController) ShouldSample(key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if time.Now().Before(c.openUntil) || c.SampleRate <= 0 {
		return false
	}
	if c.SampleRate >= 1 {
		RecordCanaryMetric("sampled")
		return true
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(key))
	sampled := float64(h.Sum32()%10000)/10000 < c.SampleRate
	if sampled {
		RecordCanaryMetric("sampled")
	}
	return sampled
}

func (c *CanaryController) Observe(success bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.total++
	if !success {
		c.failures++
		RecordCanaryMetric("failure")
	} else {
		RecordCanaryMetric("success")
	}
	min := c.MinSamples
	if min <= 0 {
		min = 20
	}
	max := c.MaxFailureRate
	if max <= 0 {
		max = .2
	}
	if c.total >= min && float64(c.failures)/float64(c.total) > max {
		cooldown := c.Cooldown
		if cooldown <= 0 {
			cooldown = 10 * time.Minute
		}
		c.openUntil = time.Now().Add(cooldown)
		c.total, c.failures = 0, 0
		RecordCanaryMetric("circuit_open")
	}
}

func (c *CanaryController) Open() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return time.Now().Before(c.openUntil)
}
