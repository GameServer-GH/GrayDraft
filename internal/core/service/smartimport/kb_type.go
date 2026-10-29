package smartimport

import (
	"fmt"
	"strings"
)

// ErrWorkspaceKBTypeRequired is returned when import lacks a workspace partition (gd_*).
var ErrWorkspaceKBTypeRequired = fmt.Errorf("hint_kb_type required: workspace partition (gd_*)")

// IsWorkspacePartition reports GrayDraft per-workspace storage partitions.
func IsWorkspacePartition(t string) bool {
	t = strings.TrimSpace(t)
	return strings.HasPrefix(t, "gd_") && len(t) > 3
}

// ValidKBType reports whether kb_type is an allowed workspace partition (gd_*).
func ValidKBType(t string) bool {
	return IsWorkspacePartition(t)
}

// ResolveKBType returns the kb partition for ingest. Only workspace gd_* partitions are accepted.
func ResolveKBType(hint string) (kbType string, confidence float64, reason string) {
	hint = strings.TrimSpace(hint)
	if IsWorkspacePartition(hint) {
		return hint, 0.95, "workspace partition (hint_kb_type)"
	}
	return "", 0, "workspace partition required"
}

