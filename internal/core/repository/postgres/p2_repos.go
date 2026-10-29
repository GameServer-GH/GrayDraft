package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/graydraft/core/service/agent"
)

type GlobalBudgetRepo struct{ db *Client }

func NewGlobalBudgetRepo(db *Client) *GlobalBudgetRepo { return &GlobalBudgetRepo{db: db} }

func (r *GlobalBudgetRepo) Policy(ctx context.Context, tenantID string) (agent.BudgetPolicy, error) {
	var p agent.BudgetPolicy
	err := r.db.pool.QueryRow(ctx, `SELECT daily_token_limit, daily_run_limit, collaborative_enabled
		FROM global_budget_policies WHERE tenant_id=$1`, tenantID).
		Scan(&p.DailyTokenLimit, &p.DailyRunLimit, &p.CollaborativeEnabled)
	if err != nil {
		return agent.BudgetPolicy{CollaborativeEnabled: true}, nil
	}
	return p, nil
}

func (r *GlobalBudgetRepo) UpsertPolicy(ctx context.Context, tenantID string, p agent.BudgetPolicy) error {
	_, err := r.db.pool.Exec(ctx, `INSERT INTO global_budget_policies (tenant_id, daily_token_limit, daily_run_limit, collaborative_enabled, updated_at)
		VALUES ($1,$2,$3,$4,now())
		ON CONFLICT (tenant_id) DO UPDATE SET
			daily_token_limit=EXCLUDED.daily_token_limit,
			daily_run_limit=EXCLUDED.daily_run_limit,
			collaborative_enabled=EXCLUDED.collaborative_enabled,
			updated_at=now()`, tenantID, p.DailyTokenLimit, p.DailyRunLimit, p.CollaborativeEnabled)
	return err
}

func (r *GlobalBudgetRepo) DailyRuns(ctx context.Context, tenantID string) (int, error) {
	start := time.Now().UTC().Truncate(24 * time.Hour)
	var n int
	err := r.db.pool.QueryRow(ctx, `SELECT COUNT(*) FROM agent_runs WHERE tenant_id=$1 AND created_at >= $2`, tenantID, start).Scan(&n)
	return n, err
}

type BlueprintRepo struct{ db *Client }

func NewBlueprintRepo(db *Client) *BlueprintRepo { return &BlueprintRepo{db: db} }

type BlueprintRecord struct {
	BlueprintID   string
	TenantID      string
	Name          string
	Version       string
	PolicyVersion string
	Description   string
	ToolAllowlist []string
	ChannelConfig map[string]any
	Limits        map[string]any
	Status        string
}

func (r *BlueprintRepo) Active(ctx context.Context, tenantID, name string) (BlueprintRecord, error) {
	var rec BlueprintRecord
	var tools, channels, limits []byte
	err := r.db.pool.QueryRow(ctx, `SELECT blueprint_id, tenant_id, name, version, policy_version, description, tool_allowlist, channel_config, limits, status
		FROM agent_blueprints
		WHERE tenant_id=$1 AND name=$2 AND status='active'
		ORDER BY updated_at DESC LIMIT 1`, tenantID, name).Scan(
		&rec.BlueprintID, &rec.TenantID, &rec.Name, &rec.Version, &rec.PolicyVersion, &rec.Description,
		&tools, &channels, &limits, &rec.Status)
	if err != nil {
		return rec, fmt.Errorf("active blueprint: %w", err)
	}
	_ = json.Unmarshal(tools, &rec.ToolAllowlist)
	_ = json.Unmarshal(channels, &rec.ChannelConfig)
	_ = json.Unmarshal(limits, &rec.Limits)
	return rec, nil
}

type ChannelBinding struct {
	BindingID       string
	TenantID        string
	Channel         string
	ExternalID      string
	BlueprintName   string
	BlueprintVersion string
	AgentID         string
	WebhookSecret   string
	Enabled         bool
}

func (r *BlueprintRepo) ResolveBinding(ctx context.Context, channel, externalID string) (ChannelBinding, error) {
	var b ChannelBinding
	err := r.db.pool.QueryRow(ctx, `SELECT binding_id, tenant_id, channel, external_id, blueprint_name, blueprint_version, agent_id, webhook_secret, enabled
		FROM channel_bindings
		WHERE channel=$1 AND external_id=$2 AND enabled=true`, channel, externalID).Scan(
		&b.BindingID, &b.TenantID, &b.Channel, &b.ExternalID, &b.BlueprintName, &b.BlueprintVersion, &b.AgentID, &b.WebhookSecret, &b.Enabled)
	if err != nil {
		return b, fmt.Errorf("channel binding: %w", err)
	}
	return b, nil
}

func (r *BlueprintRepo) UpsertBinding(ctx context.Context, b ChannelBinding) error {
	_, err := r.db.pool.Exec(ctx, `INSERT INTO channel_bindings
		(tenant_id, channel, external_id, blueprint_name, blueprint_version, agent_id, webhook_secret, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT (tenant_id, channel, external_id) DO UPDATE SET
			blueprint_name=EXCLUDED.blueprint_name,
			blueprint_version=EXCLUDED.blueprint_version,
			agent_id=EXCLUDED.agent_id,
			webhook_secret=EXCLUDED.webhook_secret,
			enabled=EXCLUDED.enabled,
			updated_at=now()`,
		b.TenantID, b.Channel, b.ExternalID, b.BlueprintName, b.BlueprintVersion, b.AgentID, b.WebhookSecret, b.Enabled)
	return err
}

func (r *BlueprintRepo) EvolveBlueprint(ctx context.Context, tenantID, name, version string, toolAllowlist []string) error {
	raw, _ := json.Marshal(toolAllowlist)
	_, err := r.db.pool.Exec(ctx, `INSERT INTO agent_blueprints
		(tenant_id, name, version, policy_version, description, tool_allowlist, status)
		VALUES ($1,$2,$3,'edu_workspace_v1','evolved blueprint',$4::jsonb,'active')
		ON CONFLICT (tenant_id, name, version) DO UPDATE SET
			tool_allowlist=EXCLUDED.tool_allowlist,
			status='active',
			updated_at=now()`, tenantID, name, version, raw)
	return err
}
