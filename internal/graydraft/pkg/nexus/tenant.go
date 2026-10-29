package nexus

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// DefaultKBID is the primary knowledge base for 1:1 workspace mapping.
const DefaultKBID = "default"

// TenantIDFromWorkspaceKey derives a stable tenant id from a normalized workspace key.
func TenantIDFromWorkspaceKey(normalizedKey string) string {
	sum := sha256.Sum256([]byte(normalizedKey))
	return "tn_" + hex.EncodeToString(sum[:16])
}

// KnowledgeScope identifies tenant + kb + storage partition for GrayDraft PaaS APIs.
type KnowledgeScope struct {
	TenantID         string
	KBID             string
	StoragePartition string
}

// ScopeFromWorkspace builds default tenant/kb for a GrayDraft workspace.
func ScopeFromWorkspace(normalizedKey, kbTypePrefix string) KnowledgeScope {
	return KnowledgeScope{
		TenantID:         TenantIDFromWorkspaceKey(normalizedKey),
		KBID:             DefaultKBID,
		StoragePartition: KbTypeFromWorkspaceKey(normalizedKey, kbTypePrefix),
	}
}

// ScopeHeaders returns optional HTTP headers for tenant-scoped Core calls.
func (s KnowledgeScope) ScopeHeaders() map[string]string {
	if strings.TrimSpace(s.TenantID) == "" || strings.TrimSpace(s.KBID) == "" {
		return nil
	}
	return map[string]string{
		"X-GrayDraft-Tenant-Id": s.TenantID,
		"X-GrayDraft-KB-Id":     s.KBID,
	}
}
