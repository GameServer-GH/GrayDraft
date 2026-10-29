package cost

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/graydraft/core/observability/trace"
)

// TokenUsageHook records model token usage for metrics (wired to observability from main).
// Kept as a hook to avoid import cycles: observability → nats → agent → cost → observability.
var TokenUsageHook func(workspace, provider, model string, tokens float64)

// Event is one model gateway invocation cost record.
type Event struct {
	TraceID      string
	TenantID     string
	AgentID      string
	RunID        string
	Workspace    string
	KBType       string
	Provider     string
	Model        string
	Operation    string
	InputTokens  int64
	OutputTokens int64
	ItemCount    int
	LatencyMS    int
	Status       string
}

// Recorder persists or forwards cost events.
type Recorder interface {
	Record(context.Context, Event) error
}

type nopRecorder struct{}

func (nopRecorder) Record(context.Context, Event) error { return nil }

// Default is the process-wide cost recorder (wired in gateway main).
var Default Recorder = nopRecorder{}

// SetDefault replaces the global recorder.
func SetDefault(r Recorder) {
	if r != nil {
		Default = r
	}
}

var dailyTokenTotal atomic.Int64

// RecordAsync fires cost recording without blocking the caller.
func RecordAsync(ctx context.Context, ev Event) {
	if ev.TraceID == "" {
		ev.TraceID = trace.FromContext(ctx)
	}
	if scope := FromContext(ctx); ev.TenantID == "" {
		ev.TenantID = scope.TenantID
		ev.AgentID = scope.AgentID
		ev.RunID = scope.RunID
		ev.Workspace = scope.Workspace
		ev.KBType = scope.KBType
	}
	if ev.Workspace == "" {
		ev.Workspace = ev.TenantID
	}
	if ev.Status == "" {
		ev.Status = "ok"
	}
	tokens := ev.InputTokens + ev.OutputTokens
	if tokens > 0 && TokenUsageHook != nil {
		TokenUsageHook(ev.Workspace, ev.Provider, ev.Model, float64(tokens))
	}
	go func() {
		if err := Default.Record(context.Background(), ev); err != nil {
			slog.Warn("cost record failed", "operation", ev.Operation, "error", err)
		}
	}()
}

// AddDailyTokens tracks in-process token totals for budget alerts.
func AddDailyTokens(n int64) int64 {
	if n <= 0 {
		return dailyTokenTotal.Load()
	}
	return dailyTokenTotal.Add(n)
}

// ResetDailyTokens clears the in-process daily counter (tests / day rollover hook).
func ResetDailyTokens() {
	dailyTokenTotal.Store(0)
}

// DailyTokens returns the in-process daily token counter.
func DailyTokens() int64 {
	return dailyTokenTotal.Load()
}

// EstimateTokens approximates token count from text length.
func EstimateTokens(text string) int64 {
	n := len([]rune(text))
	if n == 0 {
		return 0
	}
	return int64((n + 3) / 4)
}
