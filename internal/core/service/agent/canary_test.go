package agent

import (
	"testing"
	"time"
)

func TestCanaryCircuitBreaker(t *testing.T) {
	c := &CanaryController{SampleRate: 1, MinSamples: 2, MaxFailureRate: .4, Cooldown: time.Minute}
	if !c.ShouldSample("a") {
		t.Fatal("expected initial sample")
	}
	c.Observe(false)
	c.Observe(false)
	if !c.Open() || c.ShouldSample("b") {
		t.Fatal("expected open circuit")
	}
}
