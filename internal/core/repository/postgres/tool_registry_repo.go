package postgres

import (
	"context"
	"encoding/json"

	"github.com/graydraft/core/service/agent"
)

type ToolRegistryRepo struct{ db *Client }

func NewToolRegistryRepo(db *Client) *ToolRegistryRepo { return &ToolRegistryRepo{db: db} }
func (r *ToolRegistryRepo) Upsert(ctx context.Context, s agent.ToolSpec) error {
	sc, _ := json.Marshal(s.Scopes)
	req, _ := json.Marshal(s.Required)
	spec, _ := json.Marshal(s)
	network, _ := json.Marshal(s.NetworkAllowlist)
	secrets, _ := json.Marshal(s.SecretRefs)
	_, err := r.db.pool.Exec(ctx, `INSERT INTO tool_specs(name,version,risk_level,scopes,required_args,schema_hash,status,timeout_ms,idempotent,spec,runner_type,runner_command,network_allowlist,secret_refs) VALUES($1,$2,$3,$4::jsonb,$5::jsonb,$6,COALESCE(NULLIF($7,''),'draft'),$8,$9,$10::jsonb,COALESCE(NULLIF($11,''),'in_process'),$12,$13::jsonb,$14::jsonb) ON CONFLICT(name,version) DO UPDATE SET risk_level=EXCLUDED.risk_level,scopes=EXCLUDED.scopes,required_args=EXCLUDED.required_args,schema_hash=EXCLUDED.schema_hash,timeout_ms=EXCLUDED.timeout_ms,idempotent=EXCLUDED.idempotent,spec=EXCLUDED.spec,runner_type=EXCLUDED.runner_type,runner_command=EXCLUDED.runner_command,network_allowlist=EXCLUDED.network_allowlist,secret_refs=EXCLUDED.secret_refs,updated_at=now()`, s.Name, s.Version, s.Risk, sc, req, s.SchemaHash, s.Status, s.TimeoutMS, s.Idempotent, spec, s.RunnerType, s.RunnerCommand, network, secrets)
	return err
}
func (r *ToolRegistryRepo) Publish(ctx context.Context, name, version string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE tool_specs SET status='deprecated',updated_at=now() WHERE name=$1 AND status='active'; UPDATE tool_specs SET status='active',updated_at=now() WHERE name=$1 AND version=$2`, name, version)
	return err
}
func (r *ToolRegistryRepo) BindTenant(ctx context.Context, tenant, name, version string, scopes []string) error {
	b, _ := json.Marshal(scopes)
	_, err := r.db.pool.Exec(ctx, `INSERT INTO tenant_tool_bindings(tenant_id,tool_name,tool_version,scopes) VALUES($1,$2,$3,$4::jsonb) ON CONFLICT(tenant_id,tool_name) DO UPDATE SET tool_version=EXCLUDED.tool_version,scopes=EXCLUDED.scopes,enabled=true`, tenant, name, version, b)
	return err
}
func (r *ToolRegistryRepo) DisableTenantTool(ctx context.Context, tenant, name string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE tenant_tool_bindings SET enabled=false WHERE tenant_id=$1 AND tool_name=$2`, tenant, name)
	return err
}
func (r *ToolRegistryRepo) ActiveForTenant(ctx context.Context, tenant string) ([]agent.ToolSpec, error) {
	rows, err := r.db.pool.Query(ctx, `SELECT s.name,s.version,s.risk_level,s.scopes,s.required_args,s.schema_hash,s.status,s.timeout_ms,s.idempotent,s.runner_type,s.runner_command,s.network_allowlist,s.secret_refs FROM tool_specs s JOIN tenant_tool_bindings b ON b.tool_name=s.name AND b.tool_version=s.version WHERE b.tenant_id=$1 AND b.enabled=true AND s.status='active'`, tenant)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []agent.ToolSpec
	for rows.Next() {
		var s agent.ToolSpec
		var scopes, req, network, secrets []byte
		if err := rows.Scan(&s.Name, &s.Version, &s.Risk, &scopes, &req, &s.SchemaHash, &s.Status, &s.TimeoutMS, &s.Idempotent, &s.RunnerType, &s.RunnerCommand, &network, &secrets); err != nil {
			return nil, err
		}
		_ = json.Unmarshal(scopes, &s.Scopes)
		_ = json.Unmarshal(req, &s.Required)
		_ = json.Unmarshal(network, &s.NetworkAllowlist)
		_ = json.Unmarshal(secrets, &s.SecretRefs)
		out = append(out, s)
	}
	return out, rows.Err()
}
