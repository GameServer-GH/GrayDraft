package nexus

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

// KbTypeFromWorkspaceKey derives a stable GrayDraft kb_type from a normalized workspace key.
// One workspace maps to exactly one partition: {prefix}{sha256(key)[:32]}.
func KbTypeFromWorkspaceKey(normalizedKey, prefix string) string {
	if strings.TrimSpace(prefix) == "" {
		prefix = "gd_"
	}
	sum := sha256.Sum256([]byte(normalizedKey))
	return prefix + hex.EncodeToString(sum[:16])
}

// IsWorkspacePartition reports GrayDraft per-workspace storage partitions (gd_*).
func IsWorkspacePartition(kbType string) bool {
	kbType = strings.TrimSpace(kbType)
	return strings.HasPrefix(kbType, "gd_") && len(kbType) > 3
}

// WorkspaceStoragePartition returns the ingest/list partition for a workspace.
// Legacy tutorial/paper/note values in settings must not override the derived gd_* partition.
func WorkspaceStoragePartition(scope KnowledgeScope, normalizedWorkspaceKey, kbTypePrefix string) string {
	if IsWorkspacePartition(scope.StoragePartition) {
		return scope.StoragePartition
	}
	return KbTypeFromWorkspaceKey(normalizedWorkspaceKey, kbTypePrefix)
}
