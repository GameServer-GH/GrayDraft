package postgres

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/graydraft/core/service/agent"
	"strings"
)

type MemoryRepo struct {
	db       *Client
	tenantID string
	actorID  string
}

func NewMemoryRepo(db *Client, tenantID, actorID string) *MemoryRepo {
	return &MemoryRepo{db: db, tenantID: tenantID, actorID: actorID}
}
func (r *MemoryRepo) Append(ctx context.Context, conversationID string, msg agent.MemoryMessage) error {
	if msg.Role == "" || msg.Content == "" {
		return fmt.Errorf("memory: role and content are required")
	}
	if r.tenantID == "" || r.actorID == "" {
		return fmt.Errorf("memory: tenant and actor scope are required")
	}
	_, err := r.db.pool.Exec(ctx, `INSERT INTO conversation_messages(message_id,tenant_id,actor_id,conversation_id,role,content) VALUES($1,$2,$3,$4,$5,$6)`, uuid.New(), r.tenantID, r.actorID, conversationID, msg.Role, msg.Content)
	if err != nil {
		return fmt.Errorf("append memory: %w", err)
	}
	return nil
}
func (r *MemoryRepo) Recent(ctx context.Context, conversationID string, limit int) ([]agent.MemoryMessage, error) {
	if r.tenantID == "" {
		return nil, fmt.Errorf("memory: tenant scope is required")
	}
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.pool.Query(ctx, `SELECT role,content FROM conversation_messages WHERE tenant_id=$1 AND conversation_id=$2 ORDER BY created_at DESC LIMIT $3`, r.tenantID, conversationID, limit)
	if err != nil {
		return nil, fmt.Errorf("recent memory: %w", err)
	}
	defer rows.Close()
	var rev []agent.MemoryMessage
	for rows.Next() {
		var m agent.MemoryMessage
		if err := rows.Scan(&m.Role, &m.Content); err != nil {
			return nil, err
		}
		rev = append(rev, m)
	}
	for i, j := 0, len(rev)-1; i < j; i, j = i+1, j-1 {
		rev[i], rev[j] = rev[j], rev[i]
	}
	return rev, rows.Err()
}

func (r *MemoryRepo) DeleteConversation(ctx context.Context, conversationID string) error {
	if r.tenantID == "" {
		return fmt.Errorf("memory: tenant scope is required")
	}
	_, err := r.db.pool.Exec(ctx, `DELETE FROM conversation_messages WHERE tenant_id=$1 AND conversation_id=$2`, r.tenantID, conversationID)
	if err != nil {
		return fmt.Errorf("delete memory: %w", err)
	}
	_, err = r.db.pool.Exec(ctx, `DELETE FROM conversation_summaries WHERE tenant_id=$1 AND conversation_id=$2`, r.tenantID, conversationID)
	return err
}

func (r *MemoryRepo) ExportConversation(ctx context.Context, conversationID string) ([]agent.MemoryMessage, error) {
	return r.Recent(ctx, conversationID, 1000)
}

func (r *MemoryRepo) LatestSummary(ctx context.Context, conversationID string) (string, error) {
	var summary string
	err := r.db.pool.QueryRow(ctx, `SELECT summary FROM conversation_summaries WHERE tenant_id=$1 AND conversation_id=$2 ORDER BY version DESC LIMIT 1`, r.tenantID, conversationID).Scan(&summary)
	if err != nil {
		return "", nil
	}
	return summary, nil
}

// Compact stores a bounded deterministic summary while retaining all raw messages.
func (r *MemoryRepo) Compact(ctx context.Context, conversationID string, keepRecent int) error {
	msgs, err := r.Recent(ctx, conversationID, 100)
	if err != nil || len(msgs) <= keepRecent {
		return err
	}
	older := msgs[:len(msgs)-keepRecent]
	var b strings.Builder
	for _, m := range older {
		text := m.Content
		if len(text) > 500 {
			text = text[:500]
		}
		b.WriteString(m.Role)
		b.WriteString(": ")
		b.WriteString(text)
		b.WriteString(" | ")
	}
	summary := b.String()
	if len(summary) > 8000 {
		summary = summary[len(summary)-8000:]
	}
	_, err = r.db.pool.Exec(ctx, `INSERT INTO conversation_summaries(tenant_id,conversation_id,version,summary,message_count)
		VALUES($1,$2,COALESCE((SELECT max(version)+1 FROM conversation_summaries WHERE tenant_id=$1 AND conversation_id=$2),1),$3,$4)`, r.tenantID, conversationID, summary, len(older))
	return err
}
