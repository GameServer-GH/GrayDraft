package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/graydraft/core/service/agent"
	"github.com/pgvector/pgvector-go"
)

type LongMemoryRepo struct{ db *Client }

func NewLongMemoryRepo(db *Client) *LongMemoryRepo { return &LongMemoryRepo{db: db} }
func (r *LongMemoryRepo) Write(ctx context.Context, item agent.MemoryItem, consent bool) error {
	if err := agent.ValidateMemoryWrite(item, consent); err != nil {
		return err
	}
	if item.Status == "" {
		item.Status = "active"
	}
	value, err := json.Marshal(item.Value)
	if err != nil {
		return err
	}
	var existing []byte
	var existingConfidence float32
	_ = r.db.pool.QueryRow(ctx, `SELECT value,confidence FROM memory_items WHERE tenant_id=$1 AND actor_id=$2 AND kind=$3 AND subject=$4 AND status='active' ORDER BY updated_at DESC LIMIT 1`, item.TenantID, item.ActorID, item.Kind, item.Subject).Scan(&existing, &existingConfidence)
	if len(existing) > 0 && string(existing) != string(value) {
		if item.Confidence >= existingConfidence {
			// Newer, more confident facts supersede the active fact; the old row
			// remains auditable as deleted rather than being overwritten.
			_, _ = r.db.pool.Exec(ctx, `UPDATE memory_items SET status='deleted',updated_at=now() WHERE tenant_id=$1 AND actor_id=$2 AND kind=$3 AND subject=$4 AND status='active'`, item.TenantID, item.ActorID, item.Kind, item.Subject)
		} else {
			item.Status = "conflict"
			_, _ = r.db.pool.Exec(ctx, `UPDATE memory_items SET status='conflict',updated_at=now() WHERE tenant_id=$1 AND actor_id=$2 AND kind=$3 AND subject=$4 AND status='active'`, item.TenantID, item.ActorID, item.Kind, item.Subject)
		}
	}
	_, err = r.db.pool.Exec(ctx, `INSERT INTO memory_items(memory_id,tenant_id,actor_id,kind,subject,value,confidence,sensitivity,status) VALUES($1,$2,$3,$4,$5,$6::jsonb,$7,$8,$9)`, uuid.New(), item.TenantID, item.ActorID, item.Kind, item.Subject, value, item.Confidence, item.Sensitivity, item.Status)
	if err != nil {
		return fmt.Errorf("write long memory: %w", err)
	}
	return nil
}

func (r *LongMemoryRepo) CleanupExpired(ctx context.Context, tenantID string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET status='deleted',updated_at=now() WHERE tenant_id=$1 AND expires_at IS NOT NULL AND expires_at<now() AND status='active'`, tenantID)
	return err
}
func (r *LongMemoryRepo) CleanupAllExpired(ctx context.Context) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET status='deleted',updated_at=now() WHERE expires_at IS NOT NULL AND expires_at<now() AND status='active'`)
	return err
}
func (r *LongMemoryRepo) Search(ctx context.Context, tenantID, actorID, subject string, limit int) ([]agent.MemoryItem, error) {
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	rows, err := r.db.pool.Query(ctx, `SELECT memory_id,tenant_id,actor_id,kind,subject,value,confidence,sensitivity,status FROM memory_items WHERE tenant_id=$1 AND actor_id=$2 AND status='active' AND ($3='' OR subject ILIKE '%'||$3||'%') ORDER BY confidence DESC,updated_at DESC LIMIT $4`, tenantID, actorID, subject, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []agent.MemoryItem
	for rows.Next() {
		var m agent.MemoryItem
		var id uuid.UUID
		var value []byte
		if err := rows.Scan(&id, &m.TenantID, &m.ActorID, &m.Kind, &m.Subject, &value, &m.Confidence, &m.Sensitivity, &m.Status); err != nil {
			return nil, err
		}
		m.MemoryID = id.String()
		_ = json.Unmarshal(value, &m.Value)
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *LongMemoryRepo) Delete(ctx context.Context, tenantID, actorID string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET status='deleted',updated_at=now() WHERE tenant_id=$1 AND actor_id=$2`, tenantID, actorID)
	return err
}

func (r *LongMemoryRepo) SetEmbedding(ctx context.Context, memoryID string, embedding []float32) error {
	if len(embedding) == 0 {
		return fmt.Errorf("memory: empty embedding")
	}
	_, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET embedding=$2,updated_at=now() WHERE memory_id=$1`, memoryID, pgvector.NewVector(embedding))
	return err
}
func (r *LongMemoryRepo) SearchSimilar(ctx context.Context, tenantID, actorID string, embedding []float32, limit int) ([]agent.MemoryItem, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	if limit <= 0 || limit > 50 {
		limit = 5
	}
	rows, err := r.db.pool.Query(ctx, `SELECT memory_id,tenant_id,actor_id,kind,subject,value,confidence,sensitivity,status FROM memory_items WHERE tenant_id=$1 AND actor_id=$2 AND status='active' AND embedding IS NOT NULL ORDER BY embedding <=> $3 LIMIT $4`, tenantID, actorID, pgvector.NewVector(embedding), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []agent.MemoryItem
	for rows.Next() {
		var m agent.MemoryItem
		var id uuid.UUID
		var value []byte
		if err := rows.Scan(&id, &m.TenantID, &m.ActorID, &m.Kind, &m.Subject, &value, &m.Confidence, &m.Sensitivity, &m.Status); err != nil {
			return nil, err
		}
		m.MemoryID = id.String()
		_ = json.Unmarshal(value, &m.Value)
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *LongMemoryRepo) List(ctx context.Context, tenantID, actorID, status string, limit int) ([]agent.MemoryItem, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	rows, err := r.db.pool.Query(ctx, `SELECT memory_id,tenant_id,actor_id,kind,subject,value,confidence,sensitivity,status FROM memory_items WHERE tenant_id=$1 AND actor_id=$2 AND ($3='' OR status=$3) ORDER BY updated_at DESC LIMIT $4`, tenantID, actorID, status, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []agent.MemoryItem
	for rows.Next() {
		var m agent.MemoryItem
		var id uuid.UUID
		var value []byte
		if err := rows.Scan(&id, &m.TenantID, &m.ActorID, &m.Kind, &m.Subject, &value, &m.Confidence, &m.Sensitivity, &m.Status); err != nil {
			return nil, err
		}
		m.MemoryID = id.String()
		_ = json.Unmarshal(value, &m.Value)
		out = append(out, m)
	}
	return out, rows.Err()
}
func (r *LongMemoryRepo) ResolveConflict(ctx context.Context, tenantID, actorID, memoryID string) error {
	_, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET status='active',updated_at=now() WHERE memory_id=$1 AND tenant_id=$2 AND actor_id=$3 AND status='conflict'`, memoryID, tenantID, actorID)
	return err
}

func (r *LongMemoryRepo) Revoke(ctx context.Context, tenantID, actorID, memoryID string) error {
	if _, err := uuid.Parse(memoryID); err != nil {
		return fmt.Errorf("memory: invalid id")
	}
	tag, err := r.db.pool.Exec(ctx, `UPDATE memory_items SET status='deleted',updated_at=now() WHERE memory_id=$1 AND tenant_id=$2 AND actor_id=$3 AND status<>'deleted'`, memoryID, tenantID, actorID)
	if err != nil {
		return fmt.Errorf("revoke memory: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("memory: not found")
	}
	return nil
}
