// Package agentlog writes one log file per agent invocation for debugging.
package agentlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxFieldBytes = 96 << 10 // 96 KiB per JSON blob in log

// Session is a single agent use (one HTTP invocation).
type Session struct {
	path string
	f    *os.File
	mu   sync.Mutex
}

// Start creates logs under baseDir/logs/agent/{workspaceKey}/{yyyy-mm-dd}/.
// baseDir is typically GrayDraft data root; workspaceKey may be empty.
func Start(baseDir, workspaceKey, operation, traceID string) (*Session, string, error) {
	baseDir = strings.TrimSpace(baseDir)
	if baseDir == "" {
		baseDir = "."
	}
	if dir := strings.TrimSpace(os.Getenv("AGENT_LOG_DIR")); dir != "" {
		baseDir = dir
	}
	op := sanitizeFilePart(operation)
	if op == "" {
		op = "agent"
	}
	tid := sanitizeFilePart(traceID)
	if tid == "" {
		tid = "notrace"
	}
	if len(tid) > 12 {
		tid = tid[:12]
	}
	ws := sanitizeFilePart(workspaceKey)
	if ws == "" {
		ws = "_"
	}
	day := time.Now().UTC().Format("2006-01-02")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s_%s_%s.log", stamp, tid, op)
	dir := filepath.Join(baseDir, "logs", "agent", ws, day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", err
	}
	s := &Session{path: path, f: f}
	s.writeHeader(operation, traceID, workspaceKey)
	return s, path, nil
}

func (s *Session) writeHeader(operation, traceID, workspace string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.f, "=== GrayDraft Agent Session ===\n")
	_, _ = fmt.Fprintf(s.f, "operation: %s\n", operation)
	_, _ = fmt.Fprintf(s.f, "trace_id: %s\n", traceID)
	_, _ = fmt.Fprintf(s.f, "workspace: %s\n", workspace)
	_, _ = fmt.Fprintf(s.f, "started_utc: %s\n\n", time.Now().UTC().Format(time.RFC3339Nano))
}

// Path returns the log file path.
func (s *Session) Path() string {
	if s == nil {
		return ""
	}
	return s.path
}

// Step appends a named step with optional JSON payload.
func (s *Session) Step(name string, fields map[string]any) {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	_, _ = fmt.Fprintf(s.f, "--- [%s] %s ---\n", ts, name)
	if len(fields) == 0 {
		_, _ = fmt.Fprintln(s.f)
		return
	}
	b, err := json.MarshalIndent(fields, "", "  ")
	if err != nil {
		_, _ = fmt.Fprintf(s.f, "(marshal error: %v)\n\n", err)
		return
	}
	if len(b) > maxFieldBytes {
		b = append(b[:maxFieldBytes], []byte(fmt.Sprintf("\n... truncated (%d bytes total)", len(b)))...)
	}
	_, _ = s.f.Write(b)
	_, _ = fmt.Fprintln(s.f)
	_, _ = fmt.Fprintln(s.f)
}

// StepRaw logs a string field (e.g. SSE line).
func (s *Session) StepRaw(name, body string) {
	if s == nil {
		return
	}
	if len(body) > maxFieldBytes {
		body = body[:maxFieldBytes] + fmt.Sprintf("\n... truncated (%d bytes)", len(body))
	}
	s.Step(name, map[string]any{"body": body})
}

// Finish records completion; err is logged when non-nil.
func (s *Session) Finish(err error) {
	if s == nil {
		return
	}
	fields := map[string]any{"status": "ok"}
	if err != nil {
		fields["status"] = "error"
		fields["error"] = err.Error()
	}
	s.Step("session_end", fields)
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.f != nil {
		_ = s.f.Close()
		s.f = nil
	}
}

func sanitizeFilePart(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
