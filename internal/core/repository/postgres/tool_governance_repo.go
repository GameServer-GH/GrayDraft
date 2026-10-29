package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/graydraft/core/service/agent"
)

// ToolGovernanceRepo persists approval decisions and invocation audit records.
// It also implements the service-layer ports used by GovernedExecutor.
type ToolGovernanceRepo struct {
	db       *Client
	TenantID string
}

func NewToolGovernanceRepo(db *Client, tenantID string) *ToolGovernanceRepo {
	return &ToolGovernanceRepo{db: db, TenantID: tenantID}
}

func (r *ToolGovernanceRepo) IsApproved(ctx context.Context, runID, callID, toolName, argumentsHash string) bool {
	var ok bool
	err := r.db.pool.QueryRow(ctx, `SELECT status='approved' AND expires_at>now() FROM tool_approvals WHERE tenant_id=$1 AND run_id=$2::uuid AND call_id=$3 AND tool_name=$4 AND arguments_hash=$5`, r.TenantID, runID, callID, toolName, argumentsHash).Scan(&ok)
	return err == nil && ok
}

func (r *ToolGovernanceRepo) Record(ctx context.Context, runID, callID, toolName string, risk agent.RiskLevel, argumentsHash, outputHash, result string, latency time.Duration, execErr error) {
	var run any
	if runID != "" {
		run = runID
	}
	var code any
	if execErr != nil {
		code = execErr.Error()
	}
	_, _ = r.db.pool.Exec(ctx, `INSERT INTO tool_invocations(tenant_id,run_id,call_id,tool_name,risk_level,status,arguments_hash,output_hash,latency_ms,error_code) VALUES($1,$2::uuid,$3,$4,$5,$6,$7,$8,$9,$10)`, r.TenantID, run, callID, toolName, risk, result, argumentsHash, outputHash, latency.Milliseconds(), code)
}

// CreateApproval creates or refreshes a pending approval request.
func (r *ToolGovernanceRepo) CreateApproval(ctx context.Context, actorID, runID, callID, toolName, argumentsHash string, expiresAt time.Time) error {
	_, err := r.db.pool.Exec(ctx, `INSERT INTO tool_approvals(tenant_id,actor_id,run_id,call_id,tool_name,arguments_hash,expires_at) VALUES($1,$2,$3::uuid,$4,$5,$6,$7) ON CONFLICT(run_id,call_id) DO UPDATE SET status='pending',arguments_hash=EXCLUDED.arguments_hash,expires_at=EXCLUDED.expires_at,decided_at=NULL,decided_by=NULL`, r.TenantID, actorID, runID, callID, toolName, argumentsHash, expiresAt)
	return err
}

func (r *ToolGovernanceRepo) Decide(ctx context.Context, runID, callID, actorID, status string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE tool_approvals SET status=$1,decided_at=now(),decided_by=$2 WHERE tenant_id=$3 AND run_id=$4::uuid AND call_id=$5 AND status='pending'`, status, actorID, r.TenantID, runID, callID)
	return err
}

func (r *ToolGovernanceRepo) ExpireApprovals(ctx context.Context) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE tool_approvals SET status='expired',decided_at=now() WHERE status='pending' AND expires_at<=now()`)
	return err
}

type ApprovalRecord struct {
	RunID         string    `json:"run_id"`
	CallID        string    `json:"call_id"`
	ToolName      string    `json:"tool_name"`
	ActorID       string    `json:"actor_id"`
	Status        string    `json:"status"`
	ArgumentsHash string    `json:"arguments_hash"`
	ExpiresAt     time.Time `json:"expires_at"`
	DecidedAt     *time.Time `json:"decided_at,omitempty"`
	DecidedBy     string    `json:"decided_by,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

func (r *ToolGovernanceRepo) ListApprovals(ctx context.Context, status string, limit int) ([]ApprovalRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	query := `SELECT run_id, call_id, tool_name, actor_id, status, arguments_hash, expires_at, decided_at, decided_by, created_at
		FROM tool_approvals WHERE tenant_id=$1`
	args := []any{r.TenantID}
	if status != "" {
		query += ` AND status=$2`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC LIMIT $` + fmt.Sprint(len(args)+1)
	args = append(args, limit)
	rows, err := r.db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ApprovalRecord
	for rows.Next() {
		var rec ApprovalRecord
		if err := rows.Scan(&rec.RunID, &rec.CallID, &rec.ToolName, &rec.ActorID, &rec.Status, &rec.ArgumentsHash, &rec.ExpiresAt, &rec.DecidedAt, &rec.DecidedBy, &rec.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, rec)
	}
	return out, rows.Err()
}

var _ agent.ApprovalStore = (*ToolGovernanceRepo)(nil)
var _ agent.AuditRecorder = (*ToolGovernanceRepo)(nil)
