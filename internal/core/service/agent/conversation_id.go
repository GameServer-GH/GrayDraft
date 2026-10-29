package agent

import (
	"strings"

	"github.com/google/uuid"
)

// conversationIDNamespace is a fixed UUID v5 namespace for opaque client session IDs
// that are not themselves UUIDs.
var conversationIDNamespace = uuid.MustParse("7c9e6679-7425-40de-944b-e07fc1f90ae7")

// NormalizeConversationID returns a value safe for agent_runs.conversation_id (UUID).
// Empty input stays empty (SQL NULL). Valid UUIDs pass through. Other opaque IDs are
// mapped deterministically via UUID v5 so the same session correlates across runs.
func NormalizeConversationID(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	if _, err := uuid.Parse(raw); err == nil {
		return raw
	}
	return uuid.NewSHA1(conversationIDNamespace, []byte(raw)).String()
}
