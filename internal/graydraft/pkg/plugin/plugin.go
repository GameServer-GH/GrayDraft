// Package plugin：插件存储与审计（占位）。
package plugin

import "sync"

// Audit 占位：后续写入 NDJSON 审计日志。
type Audit struct {
	mu sync.Mutex
}

func (a *Audit) Log(pluginID, action string, _ any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	_ = pluginID
	_ = action
}
