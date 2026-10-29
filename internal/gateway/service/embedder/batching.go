package embedder

import (
	"context"
	"sync"
	"time"
)

// BatchingEmbedder wraps the base Embed method with a short-window batching
// pattern (V1-31). Single requests are held for up to batchWindow and then
// dispatched together.
type BatchingEmbedder struct {
	svc         *Service
	batchWindow time.Duration
	maxBatch    int

	mu       sync.Mutex
	pending  []batchItem
	timer    *time.Timer
}

type batchItem struct {
	text    string
	model   string
	result  chan batchResult
}

type batchResult struct {
	vec   []float32
	model string
	err   error
}

// NewBatchingEmbedder wraps the given Service.
func NewBatchingEmbedder(svc *Service, window time.Duration, maxBatch int) *BatchingEmbedder {
	return &BatchingEmbedder{
		svc:         svc,
		batchWindow: window,
		maxBatch:    maxBatch,
	}
}

// EmbedOne enqueues a single text and returns the embedding.
// It blocks until the batch window expires or the batch is full.
func (be *BatchingEmbedder) EmbedOne(ctx context.Context, text string, model string) ([]float32, string, error) {
	ch := make(chan batchResult, 1)
	be.mu.Lock()
	be.pending = append(be.pending, batchItem{text: text, model: model, result: ch})
	shouldFlush := len(be.pending) >= be.maxBatch
	if !shouldFlush && be.timer == nil {
		be.timer = time.AfterFunc(be.batchWindow, be.flush)
	}
	be.mu.Unlock()

	select {
	case r := <-ch:
		return r.vec, r.model, r.err
	case <-ctx.Done():
		return nil, model, ctx.Err()
	}
}

func (be *BatchingEmbedder) flush() {
	be.mu.Lock()
	if len(be.pending) == 0 {
		be.timer = nil
		be.mu.Unlock()
		return
	}
	items := be.pending
	be.pending = nil
	be.timer = nil
	be.mu.Unlock()

	// Group by model
	byModel := map[string][]int{}
	texts := make([]string, len(items))
	for i, item := range items {
		texts[i] = item.text
		byModel[item.model] = append(byModel[item.model], i)
	}

	for model, indices := range byModel {
		batchTexts := make([]string, len(indices))
		for j, idx := range indices {
			batchTexts[j] = texts[idx]
		}
		vecs, usedModel, err := be.svc.Embed(context.Background(), batchTexts, model)
		for j, idx := range indices {
			var r batchResult
			if err != nil {
				r = batchResult{model: model, err: err}
			} else if j < len(vecs) {
				r = batchResult{vec: vecs[j], model: usedModel}
			} else {
				r = batchResult{model: model, err: err}
			}
			select {
			case items[idx].result <- r:
			default:
			}
		}
	}
}
