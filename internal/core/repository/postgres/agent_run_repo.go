package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/graydraft/core/service/agent"
)

// AgentRunRepo persists the append-only Run/Event stream. The application
// owns state transitions; this adapter only enforces ordering and idempotency.
type AgentRunRepo struct {
	db       *Client
	notifier RunEventNotifier
}

// RunEventNotifier wakes SSE subscribers when a new event is appended.
type RunEventNotifier interface {
	NotifyRunEvent(ctx context.Context, tenantID, runID string, seq int64) error
}

func NewAgentRunRepo(db *Client) *AgentRunRepo { return &AgentRunRepo{db: db} }

func (r *AgentRunRepo) WithEventNotifier(n RunEventNotifier) *AgentRunRepo {
	r.notifier = n
	return r
}

func (r *AgentRunRepo) CreateRun(ctx context.Context, run agent.RunRecord) (agent.RunRecord, error) {
	if run.RunID == "" {
		run.RunID = uuid.NewString()
	}
	if run.Status == "" {
		run.Status = agent.StateQueued
	}
	if run.Deadline.IsZero() {
		run.Deadline = time.Now().Add(time.Minute)
	}
	run.ConversationID = agent.NormalizeConversationID(run.ConversationID)
	row := r.db.pool.QueryRow(ctx, `INSERT INTO agent_runs
		(run_id, tenant_id, actor_id, agent_id, conversation_id, status, policy_version, deadline_at, idempotency_key, prompt, authorized_kbs)
		VALUES ($1,$2,$3,$4,NULLIF($5,'')::uuid,$6,$7,$8,$9,$10,$11)
		ON CONFLICT (tenant_id,idempotency_key) DO UPDATE SET updated_at=agent_runs.updated_at
		RETURNING run_id, status, deadline_at`,
		run.RunID, run.TenantID, run.ActorID, run.AgentID, run.ConversationID,
		run.Status, run.PolicyVersion, run.Deadline, run.IdempotencyKey, run.Prompt, run.AuthorizedKBs)
	var status string
	if err := row.Scan(&run.RunID, &status, &run.Deadline); err != nil {
		return run, fmt.Errorf("create agent run: %w", err)
	}
	run.Status = agent.State(status)
	return run, nil
}

func (r *AgentRunRepo) AppendEvent(ctx context.Context, ev agent.RunEvent) error {
	if ev.EventID == "" {
		ev.EventID = uuid.NewString()
	}
	if ev.OccurredAt.IsZero() {
		ev.OccurredAt = time.Now()
	}
	if len(ev.Payload) == 0 {
		ev.Payload = []byte(`{}`)
	}
	if !json.Valid(ev.Payload) {
		return fmt.Errorf("append agent event: payload is not JSON")
	}
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("append agent event begin: %w", err)
	}
	defer tx.Rollback(ctx)
	result, err := tx.Exec(ctx, `INSERT INTO agent_run_events
        (run_id, tenant_id, seq, event_id, type, payload, occurred_at)
        VALUES ($1,$2,$3,$4,$5,$6::jsonb,$7) ON CONFLICT DO NOTHING`,
		ev.RunID, ev.TenantID, ev.Seq, ev.EventID, ev.Type, ev.Payload, ev.OccurredAt)
	if err != nil {
		return fmt.Errorf("append agent event: %w", err)
	}
	if result.RowsAffected() == 1 {
		if err := projectEvent(ctx, tx, ev); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `INSERT INTO agent_run_outbox
			(tenant_id, run_id, event_seq, topic, payload)
			VALUES ($1,$2,$3,$4,$5::jsonb)
			ON CONFLICT (run_id, event_seq, topic) DO NOTHING`,
			ev.TenantID, ev.RunID, ev.Seq, "agent.run.event", ev.Payload); err != nil {
			return fmt.Errorf("append agent outbox: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("append agent event commit: %w", err)
	}
	if result.RowsAffected() == 1 && r.notifier != nil {
		_ = r.notifier.NotifyRunEvent(ctx, ev.TenantID, ev.RunID, ev.Seq)
	}
	return nil
}

func projectEvent(ctx context.Context, tx pgx.Tx, ev agent.RunEvent) error {
	status := ""
	switch ev.Type {
	case "run.started":
		status = "queued"
	case "step.started":
		status = "deciding"
	case "tool.started":
		status = "executing_tool"
	case "tool.completed":
		status = "observing"
	case "run.completed":
		status = "completed"
	case "run.failed", "run.invalid_decision", "run.no_progress":
		status = "failed"
	case "run.cancelled":
		status = "cancelled"
	case "run.expired", "run.budget_exceeded", "run.tool_budget_exceeded":
		status = "expired"
	}
	if status == "" {
		return nil
	}
	_, err := tx.Exec(ctx, `UPDATE agent_runs SET status=$2, step_count=step_count+CASE WHEN $2='deciding' THEN 1 ELSE 0 END, tool_call_count=tool_call_count+CASE WHEN $2='executing_tool' THEN 1 ELSE 0 END, updated_at=now() WHERE run_id=$1 AND tenant_id=$3`, ev.RunID, status, ev.TenantID)
	return err
}

// RebuildProjection reconstructs the Run projection from its append-only events.
func (r *AgentRunRepo) RebuildProjection(ctx context.Context, tenantID, runID string) error {
	events, err := r.ListEvents(ctx, tenantID, runID, 0, 5000)
	if err != nil {
		return err
	}
	if _, err := r.db.pool.Exec(ctx, `UPDATE agent_runs SET status='queued',step_count=0,tool_call_count=0,updated_at=now() WHERE tenant_id=$1 AND run_id=$2`, tenantID, runID); err != nil {
		return err
	}
	for _, ev := range events {
		if err := r.projectOne(ctx, ev); err != nil {
			return err
		}
	}
	return nil
}
func (r *AgentRunRepo) projectOne(ctx context.Context, ev agent.RunEvent) error {
	tx, err := r.db.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := projectEvent(ctx, tx, ev); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (r *AgentRunRepo) ListEvents(ctx context.Context, tenantID, runID string, afterSeq int64, limit int64) ([]agent.RunEvent, error) {
	if limit <= 0 || limit > 5000 {
		limit = 200
	}
	rows, err := r.db.pool.Query(ctx, `SELECT run_id, tenant_id, seq, event_id, type, payload, occurred_at
        FROM agent_run_events WHERE tenant_id=$1 AND run_id=$2 AND seq>$3 ORDER BY seq LIMIT $4`, tenantID, runID, afterSeq, limit)
	if err != nil {
		return nil, fmt.Errorf("list agent events: %w", err)
	}
	defer rows.Close()
	var out []agent.RunEvent
	for rows.Next() {
		var ev agent.RunEvent
		if err := rows.Scan(&ev.RunID, &ev.TenantID, &ev.Seq, &ev.EventID, &ev.Type, &ev.Payload, &ev.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan agent event: %w", err)
		}
		out = append(out, ev)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agent events: %w", err)
	}
	return out, nil
}

func (r *AgentRunRepo) CancelRun(ctx context.Context, tenantID, runID string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE agent_runs SET status='cancelled', updated_at=now()
        WHERE tenant_id=$1 AND run_id=$2 AND status IN ('queued','deciding','validating_tool','executing_tool','observing')`, tenantID, runID)
	if err != nil {
		return fmt.Errorf("cancel agent run: %w", err)
	}
	return nil
}

func (r *AgentRunRepo) UpdateStatus(ctx context.Context, runID string, status agent.State, steps, toolCalls int) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE agent_runs SET status=$2, step_count=$3, tool_call_count=$4, updated_at=now() WHERE run_id=$1`, runID, status, steps, toolCalls)
	if err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	return nil
}
