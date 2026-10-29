package kbscope

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/repository/postgres"
)

// Resolver maps PaaS (tenant_id, kb_id) to storage partitions and ensures registration.
type Resolver struct {
	scope *postgres.ScopeRepo
}

// NewResolver creates KB scope resolver.
func NewResolver(scope *postgres.ScopeRepo) *Resolver {
	return &Resolver{scope: scope}
}

// EnsureTenant registers a tenant.
func (r *Resolver) EnsureTenant(ctx context.Context, req model.EnsureTenantRequest) error {
	return r.scope.EnsureTenant(ctx, strings.TrimSpace(req.TenantID), strings.TrimSpace(req.DisplayName))
}

// EnsureKnowledgeBase registers a KB under a tenant.
func (r *Resolver) EnsureKnowledgeBase(ctx context.Context, tenantID string, req model.EnsureKnowledgeBaseRequest) (*postgres.KnowledgeBaseRow, error) {
	return r.scope.EnsureKnowledgeBase(
		ctx,
		strings.TrimSpace(tenantID),
		strings.TrimSpace(req.KBID),
		strings.TrimSpace(req.DisplayName),
		strings.TrimSpace(req.Description),
		strings.TrimSpace(req.StoragePartition),
	)
}

// ResolveKBType picks storage partition from tenant/kb or legacy kb_type.
func (r *Resolver) ResolveKBType(ctx context.Context, tenantID, kbID, legacyKBType string) (string, error) {
	tenantID = strings.TrimSpace(tenantID)
	kbID = strings.TrimSpace(kbID)
	legacyKBType = strings.TrimSpace(legacyKBType)

	if legacyKBType != "" && isWorkspaceStoragePartition(legacyKBType) {
		return legacyKBType, nil
	}

	if tenantID != "" && kbID != "" {
		return r.scope.ResolveStoragePartition(ctx, tenantID, kbID)
	}
	if legacyKBType != "" {
		return legacyKBType, nil
	}
	if tenantID != "" || kbID != "" {
		return "", fmt.Errorf("both tenant_id and kb_id are required when using PaaS scope")
	}
	return "", fmt.Errorf("kb_type or (tenant_id, kb_id) required")
}

// ListKnowledgeBases returns KBs under a tenant (O-13b).
func (r *Resolver) ListKnowledgeBases(ctx context.Context, tenantID string) ([]postgres.KnowledgeBaseRow, error) {
	return r.scope.ListKnowledgeBases(ctx, strings.TrimSpace(tenantID))
}

// ScopeFromRequest builds KBScope when tenant/kb present.
func (r *Resolver) ScopeFromRequest(ctx context.Context, tenantID, kbID, legacyKBType string) (*model.KBScope, error) {
	partition, err := r.ResolveKBType(ctx, tenantID, kbID, legacyKBType)
	if err != nil {
		return nil, err
	}
	tenantID = strings.TrimSpace(tenantID)
	kbID = strings.TrimSpace(kbID)
	if tenantID == "" {
		tenantID = "_legacy"
	}
	if kbID == "" {
		kbID = legacyKBType
	}
	return &model.KBScope{
		TenantID:         tenantID,
		KBID:             kbID,
		StoragePartition: partition,
	}, nil
}

func isWorkspaceStoragePartition(t string) bool {
	t = strings.TrimSpace(t)
	return strings.HasPrefix(t, "gd_") && len(t) > 3
}
