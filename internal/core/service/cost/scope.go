package cost

import "context"

type scopeKey struct{}

// Scope carries tenant/agent attribution for model cost events.
type Scope struct {
	TenantID  string
	AgentID   string
	RunID     string
	Workspace string
	KBType    string
}

// WithScope attaches cost attribution to ctx.
func WithScope(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, scopeKey{}, s)
}

// FromContext returns the cost scope when present.
func FromContext(ctx context.Context) Scope {
	if ctx == nil {
		return Scope{}
	}
	v, _ := ctx.Value(scopeKey{}).(Scope)
	return v
}
