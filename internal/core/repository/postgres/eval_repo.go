package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/graydraft/core/service/agent"
)

// EvalRepo persists agent evaluation runs.
type EvalRepo struct {
	db *Client
}

func NewEvalRepo(db *Client) *EvalRepo { return &EvalRepo{db: db} }

func (r *EvalRepo) Save(ctx context.Context, rec agent.EvalRunRecord) (agent.EvalRunRecord, error) {
	if r == nil || r.db == nil {
		return rec, fmt.Errorf("eval repo unavailable")
	}
	if rec.EvalID == "" {
		rec.EvalID = uuid.NewString()
	}
	if rec.Status == "" {
		rec.Status = "completed"
	}
	raw, err := json.Marshal(rec.Report)
	if err != nil {
		return rec, fmt.Errorf("marshal eval report: %w", err)
	}
	var createdAt time.Time
	row := r.db.pool.QueryRow(ctx, `INSERT INTO agent_eval_runs
		(eval_id, tenant_id, agent_id, model, policy_version, dataset_path, total, passed, pass_rate, report, status)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::jsonb,$11)
		RETURNING created_at`,
		rec.EvalID, rec.TenantID, rec.AgentID, rec.Model, rec.PolicyVersion, rec.DatasetPath,
		rec.Total, rec.Passed, rec.PassRate, raw, rec.Status)
	if err := row.Scan(&createdAt); err != nil {
		return rec, fmt.Errorf("insert eval run: %w", err)
	}
	rec.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return rec, nil
}

func (r *EvalRepo) List(ctx context.Context, tenantID string, limit int) ([]agent.EvalRunRecord, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("eval repo unavailable")
	}
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT eval_id, tenant_id, agent_id, model, policy_version, dataset_path,
		total, passed, pass_rate, report, status, created_at
		FROM agent_eval_runs`
	args := []any{limit}
	if tenantID != "" {
		query += ` WHERE tenant_id = $2`
		args = []any{limit, tenantID}
	}
	query += ` ORDER BY created_at DESC LIMIT $1`

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list eval runs: %w", err)
	}
	defer rows.Close()

	var out []agent.EvalRunRecord
	for rows.Next() {
		var rec agent.EvalRunRecord
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&rec.EvalID, &rec.TenantID, &rec.AgentID, &rec.Model, &rec.PolicyVersion,
			&rec.DatasetPath, &rec.Total, &rec.Passed, &rec.PassRate, &raw, &rec.Status, &createdAt); err != nil {
			return nil, fmt.Errorf("list eval runs scan: %w", err)
		}
		_ = json.Unmarshal(raw, &rec.Report)
		rec.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out = append(out, rec)
	}
	return out, rows.Err()
}

// EvalQualitySummary aggregates recent evaluation runs for admin dashboards.
type EvalQualitySummary struct {
	EvalRuns       int                        `json:"eval_runs"`
	AvgPassRate    float64                    `json:"avg_pass_rate"`
	LatestEvalID   string                     `json:"latest_eval_id,omitempty"`
	LatestPassRate float64                    `json:"latest_pass_rate"`
	Categories     []agent.CategoryScore      `json:"categories,omitempty"`
	Metrics        agent.EvalMetrics          `json:"metrics,omitempty"`
	Recent         []agentEvalQualitySnapshot `json:"recent,omitempty"`
}

type agentEvalQualitySnapshot struct {
	EvalID    string  `json:"eval_id"`
	TenantID  string  `json:"tenant_id"`
	PassRate  float64 `json:"pass_rate"`
	Total     int     `json:"total"`
	Passed    int     `json:"passed"`
	CreatedAt string  `json:"created_at"`
}

func (r *EvalRepo) QualitySummary(ctx context.Context, tenantID string, since time.Time, limit int) (EvalQualitySummary, error) {
	if r == nil || r.db == nil {
		return EvalQualitySummary{}, fmt.Errorf("eval repo unavailable")
	}
	if limit <= 0 || limit > 50 {
		limit = 10
	}
	query := `SELECT eval_id, tenant_id, total, passed, pass_rate, report, created_at
		FROM agent_eval_runs WHERE created_at >= $1`
	args := []any{since}
	if tenantID != "" {
		query += ` AND tenant_id = $2`
		args = append(args, tenantID)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)

	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return EvalQualitySummary{}, fmt.Errorf("eval quality summary: %w", err)
	}
	defer rows.Close()

	var out EvalQualitySummary
	var passSum float64
	for rows.Next() {
		var rec agent.EvalRunRecord
		var raw []byte
		var createdAt time.Time
		if err := rows.Scan(&rec.EvalID, &rec.TenantID, &rec.Total, &rec.Passed, &rec.PassRate, &raw, &createdAt); err != nil {
			return EvalQualitySummary{}, fmt.Errorf("eval quality scan: %w", err)
		}
		_ = json.Unmarshal(raw, &rec.Report)
		rec.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		out.EvalRuns++
		passSum += rec.PassRate
		out.Recent = append(out.Recent, agentEvalQualitySnapshot{
			EvalID: rec.EvalID, TenantID: rec.TenantID, PassRate: rec.PassRate,
			Total: rec.Total, Passed: rec.Passed, CreatedAt: rec.CreatedAt,
		})
		if out.LatestEvalID == "" {
			out.LatestEvalID = rec.EvalID
			out.LatestPassRate = rec.PassRate
			out.Categories = rec.Report.Categories
			out.Metrics = rec.Report.Metrics
		}
	}
	if out.EvalRuns > 0 {
		out.AvgPassRate = passSum / float64(out.EvalRuns)
	}
	return out, rows.Err()
}
