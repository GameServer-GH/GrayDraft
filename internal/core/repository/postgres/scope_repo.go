package postgres

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/jackc/pgx/v5"
)

var reScopeID = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)

// ScopeRepo resolves tenant/kb_id to storage partitions (kb_types.name).
type ScopeRepo struct {
	client *Client
	docs   *DocumentRepo
}

// NewScopeRepo creates scope repository.
func NewScopeRepo(client *Client, docs *DocumentRepo) *ScopeRepo {
	return &ScopeRepo{client: client, docs: docs}
}

// ValidateScopeID checks tenant_id / kb_id format.
func ValidateScopeID(id, field string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%s required", field)
	}
	if len(id) > 64 {
		return fmt.Errorf("%s too long (max 64)", field)
	}
	if !reScopeID.MatchString(id) {
		return fmt.Errorf("%s invalid: use letters, digits, underscore, hyphen", field)
	}
	return nil
}

// EnsureTenant inserts tenant if missing.
func (r *ScopeRepo) EnsureTenant(ctx context.Context, tenantID, displayName string) error {
	if err := ValidateScopeID(tenantID, "tenant_id"); err != nil {
		return err
	}
	_, err := r.client.pool.Exec(ctx, `
		INSERT INTO tenants (id, display_name)
		VALUES ($1, $2)
		ON CONFLICT (id) DO UPDATE SET
			display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), tenants.display_name)
	`, tenantID, displayName)
	return err
}

// KnowledgeBaseRow is a registered knowledge base.
type KnowledgeBaseRow struct {
	TenantID          string `json:"tenant_id"`
	KBID              string `json:"kb_id"`
	DisplayName       string `json:"display_name,omitempty"`
	StoragePartition  string `json:"storage_partition"`
	Description       string `json:"description,omitempty"`
}

// EnsureKnowledgeBase registers KB and underlying kb_type partition.
func (r *ScopeRepo) EnsureKnowledgeBase(ctx context.Context, tenantID, kbID, displayName, description, storagePartition string) (*KnowledgeBaseRow, error) {
	if err := ValidateScopeID(tenantID, "tenant_id"); err != nil {
		return nil, err
	}
	if err := ValidateScopeID(kbID, "kb_id"); err != nil {
		return nil, err
	}
	if err := r.EnsureTenant(ctx, tenantID, ""); err != nil {
		return nil, err
	}

	partition := strings.TrimSpace(storagePartition)
	if partition == "" {
		partition = DefaultStoragePartition(tenantID, kbID)
	}
	if len(partition) > 128 {
		return nil, fmt.Errorf("storage_partition too long")
	}

	desc := description
	if desc == "" {
		desc = fmt.Sprintf("tenant=%s kb=%s", tenantID, kbID)
	}
	if err := r.docs.EnsureKBType(ctx, partition, desc); err != nil {
		return nil, err
	}

	_, err := r.client.pool.Exec(ctx, `
		INSERT INTO knowledge_bases (id, tenant_id, display_name, description, storage_partition)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (tenant_id, id) DO UPDATE SET
			display_name = COALESCE(NULLIF(EXCLUDED.display_name, ''), knowledge_bases.display_name),
			description = COALESCE(NULLIF(EXCLUDED.description, ''), knowledge_bases.description)
	`, kbID, tenantID, displayName, description, partition)
	if err != nil {
		return nil, err
	}

	row, err := r.GetKnowledgeBase(ctx, tenantID, kbID)
	if err != nil {
		return nil, err
	}
	return row, nil
}

// GetKnowledgeBase loads KB metadata.
func (r *ScopeRepo) GetKnowledgeBase(ctx context.Context, tenantID, kbID string) (*KnowledgeBaseRow, error) {
	var row KnowledgeBaseRow
	err := r.client.pool.QueryRow(ctx, `
		SELECT tenant_id, id, COALESCE(display_name, ''), storage_partition, COALESCE(description, '')
		FROM knowledge_bases
		WHERE tenant_id = $1 AND id = $2
	`, tenantID, kbID).Scan(&row.TenantID, &row.KBID, &row.DisplayName, &row.StoragePartition, &row.Description)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("knowledge base not found: %s/%s", tenantID, kbID)
	}
	if err != nil {
		return nil, err
	}
	return &row, nil
}

// ListKnowledgeBases returns all KBs for a tenant (O-13b Phase B).
func (r *ScopeRepo) ListKnowledgeBases(ctx context.Context, tenantID string) ([]KnowledgeBaseRow, error) {
	if err := ValidateScopeID(tenantID, "tenant_id"); err != nil {
		return nil, err
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT tenant_id, id, COALESCE(display_name, ''), storage_partition, COALESCE(description, '')
		FROM knowledge_bases
		WHERE tenant_id = $1
		ORDER BY id
	`, tenantID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []KnowledgeBaseRow
	for rows.Next() {
		var row KnowledgeBaseRow
		if err := rows.Scan(&row.TenantID, &row.KBID, &row.DisplayName, &row.StoragePartition, &row.Description); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ResolveStoragePartition returns kb_types.name for (tenant_id, kb_id).
func (r *ScopeRepo) ResolveStoragePartition(ctx context.Context, tenantID, kbID string) (string, error) {
	row, err := r.GetKnowledgeBase(ctx, tenantID, kbID)
	if err != nil {
		return "", fmt.Errorf("knowledge base not found: %s/%s", tenantID, kbID)
	}
	return row.StoragePartition, nil
}

// DefaultStoragePartition generates a new partition name for greenfield tenants.
func DefaultStoragePartition(tenantID, kbID string) string {
	t := sanitizePartitionPart(tenantID)
	k := sanitizePartitionPart(kbID)
	return fmt.Sprintf("t_%s_%s", t, k)
}

func sanitizePartitionPart(s string) string {
	s = strings.TrimSpace(s)
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-' || r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" {
		out = "x"
	}
	if len(out) > 48 {
		out = out[:48]
	}
	return out
}
