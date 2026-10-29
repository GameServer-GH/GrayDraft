package cache

import "sync"

// PredictorL3 tracks recent lookup params per kb_type for simple next-query prefetch.
type PredictorL3 struct {
	mu   sync.RWMutex
	last map[string]LookupParams // kbType -> last stored search
}

// NewPredictorL3 creates L3 predictor.
func NewPredictorL3() *PredictorL3 {
	return &PredictorL3{last: make(map[string]LookupParams)}
}

// Record stores the latest lookup params for a kb partition.
func (p *PredictorL3) Record(params LookupParams) {
	if p == nil {
		return
	}
	p.mu.Lock()
	p.last[params.KBType] = params
	p.mu.Unlock()
}

// Previous returns the last lookup params before current (for prefetch lookup).
func (p *PredictorL3) Previous(kbType string) (LookupParams, bool) {
	if p == nil {
		return LookupParams{}, false
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	prev, ok := p.last[kbType]
	return prev, ok
}
