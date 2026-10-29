package agent

import "context"

type reasoningSinkKey struct{}

// WithReasoningSink attaches a callback that receives model reasoning_content deltas
// (e.g. DeepSeek reasoner). Used by planner Complete and final-answer streaming.
func WithReasoningSink(ctx context.Context, sink func(delta string) error) context.Context {
	if sink == nil {
		return ctx
	}
	return context.WithValue(ctx, reasoningSinkKey{}, sink)
}

// ReasoningSinkFrom returns the sink from ctx, or nil.
func ReasoningSinkFrom(ctx context.Context) func(string) error {
	if v, ok := ctx.Value(reasoningSinkKey{}).(func(string) error); ok {
		return v
	}
	return nil
}
