package postgres

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/graydraft/core/service/cost"
	"github.com/graydraft/core/observability"
)

// CostRepo persists model_cost_events and serves admin aggregates.
type CostRepo struct {
	db              *Client
	dailyTokenLimit int64
}

func NewCostRepo(db *Client, dailyTokenLimit int64) *CostRepo {
	return &CostRepo{db: db, dailyTokenLimit: dailyTokenLimit}
}

func (r *CostRepo) Record(ctx context.Context, ev cost.Event) error {
	if r == nil || r.db == nil {
		return nil
	}
	var runID *uuid.UUID
	if ev.RunID != "" {
		if id, err := uuid.Parse(ev.RunID); err == nil {
			runID = &id
		}
	}
	workspace := ev.Workspace
	if workspace == "" {
		workspace = ev.TenantID
	}
	_, err := r.db.pool.Exec(ctx, `INSERT INTO model_cost_events
		(trace_id, workspace, kb_type, provider, model, operation, input_tokens, output_tokens, item_count, latency_ms, status, tenant_id, agent_id, run_id)
		VALUES ($1,$2,NULLIF($3,''),$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)`,
		ev.TraceID, workspace, ev.KBType, ev.Provider, ev.Model, ev.Operation,
		ev.InputTokens, ev.OutputTokens, ev.ItemCount, ev.LatencyMS, ev.Status,
		ev.TenantID, ev.AgentID, runID)
	if err != nil {
		return fmt.Errorf("insert cost event: %w", err)
	}
	tokens := ev.InputTokens + ev.OutputTokens
	if tokens > 0 {
		total := cost.AddDailyTokens(tokens)
		if r.dailyTokenLimit > 0 && total > r.dailyTokenLimit {
			observability.RecordCostBudgetExceeded(ev.TenantID, ev.Operation)
			slog.Warn("daily token budget exceeded",
				"tenant_id", ev.TenantID,
				"operation", ev.Operation,
				"daily_tokens", total,
				"limit", r.dailyTokenLimit,
			)
		}
	}
	return nil
}

type CostSummaryRow struct {
	Operation    string  `json:"operation"`
	Provider     string  `json:"provider"`
	Model        string  `json:"model"`
	TenantID     string  `json:"tenant_id"`
	AgentID      string  `json:"agent_id"`
	InputTokens  int64   `json:"input_tokens"`
	OutputTokens int64   `json:"output_tokens"`
	Calls        int64   `json:"calls"`
	AvgLatencyMS float64 `json:"avg_latency_ms"`
}

// Summary aggregates model cost events since the given time.
func (r *CostRepo) Summary(ctx context.Context, tenantID string, since time.Time) ([]CostSummaryRow, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("cost repo unavailable")
	}
	query := `SELECT operation, provider, model, tenant_id, agent_id,
		COALESCE(SUM(input_tokens),0), COALESCE(SUM(output_tokens),0),
		COUNT(*), COALESCE(AVG(latency_ms),0)
		FROM model_cost_events
		WHERE created_at >= $1`
	args := []any{since}
	if tenantID != "" {
		query += ` AND tenant_id = $2`
		args = append(args, tenantID)
	}
	query += ` GROUP BY operation, provider, model, tenant_id, agent_id
		ORDER BY SUM(input_tokens)+SUM(output_tokens) DESC`

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("cost summary: %w", err)
	}
	defer rows.Close()

	var out []CostSummaryRow
	for rows.Next() {
		var row CostSummaryRow
		if err := rows.Scan(&row.Operation, &row.Provider, &row.Model, &row.TenantID, &row.AgentID,
			&row.InputTokens, &row.OutputTokens, &row.Calls, &row.AvgLatencyMS); err != nil {
			return nil, fmt.Errorf("cost summary scan: %w", err)
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DailyTotals returns total tokens per tenant for the current UTC day.
func (r *CostRepo) DailyTotals(ctx context.Context, tenantID string) (int64, error) {
	if r == nil || r.db == nil {
		return 0, fmt.Errorf("cost repo unavailable")
	}
	start := time.Now().UTC().Truncate(24 * time.Hour)
	query := `SELECT COALESCE(SUM(input_tokens)+SUM(output_tokens),0) FROM model_cost_events WHERE created_at >= $1`
	args := []any{start}
	if tenantID != "" {
		query += ` AND tenant_id = $2`
		args = append(args, tenantID)
	}
	var total int64
	if err := r.db.pool.QueryRow(ctx, query, args...).Scan(&total); err != nil {
		return 0, fmt.Errorf("cost daily totals: %w", err)
	}
	return total, nil
}
