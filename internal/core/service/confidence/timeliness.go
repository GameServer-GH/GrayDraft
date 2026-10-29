// Package confidence: timeliness detection for answer cache (V1-40).
// Prevents stale answer cache hits for queries containing time-sensitive keywords.
package confidence

import (
	"strings"
	"sync"
)

// Default timeliness keywords — configurable per workspace via dynamic config.
var defaultTimelinessKeywords = []string{
	"最新", "今天", "现在", "当前", "本月", "本周", "今日",
	"版本", "更新", "价格", "实时", "最新消息", "最新版本",
	"当前学期", "本周课件", "今日作业", "本次实验",
}

var workspaceTimelinessMu sync.RWMutex
var workspaceTimeliness = make(map[string][]string)

// RegisterWorkspaceTimeliness registers workspace-specific timeliness keywords.
func RegisterWorkspaceTimeliness(workspace string, keywords []string) {
	workspaceTimelinessMu.Lock()
	defer workspaceTimelinessMu.Unlock()
	if len(keywords) == 0 {
		delete(workspaceTimeliness, workspace)
		return
	}
	cp := make([]string, len(keywords))
	copy(cp, keywords)
	workspaceTimeliness[workspace] = cp
}

// IsTimelinessQuery returns true if the query contains time-sensitive keywords.
func IsTimelinessQuery(query, workspace string) bool {
	lowerQ := strings.ToLower(strings.TrimSpace(query))
	if lowerQ == "" {
		return false
	}

	// Check workspace-specific first.
	workspaceTimelinessMu.RLock()
	wsKeywords := workspaceTimeliness[workspace]
	workspaceTimelinessMu.RUnlock()
	for _, kw := range wsKeywords {
		if strings.Contains(lowerQ, strings.ToLower(kw)) {
			return true
		}
	}

	// Check defaults.
	for _, kw := range defaultTimelinessKeywords {
		if strings.Contains(lowerQ, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}
