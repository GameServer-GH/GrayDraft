// Package agentlog writes one log file per agent/Core invocation (correlate via trace_id).
package agentlog

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/graydraft/core/observability/trace"
)

const maxFieldBytes = 96 << 10

// Session is one agent API handling on Core.
type Session struct {
	path string
	f    *os.File
	mu   sync.Mutex
}

// StartFromContext opens a log under logRoot/logs/agent/{day}/ (or AGENT_LOG_DIR).
func StartFromContext(ctx context.Context, logRoot, operation string) (*Session, string) {
	if logRoot == "" {
		logRoot = "."
	}
	if dir := strings.TrimSpace(os.Getenv("AGENT_LOG_DIR")); dir != "" {
		logRoot = dir
	}
	s, path, err := start(logRoot, operation, trace.FromContext(ctx))
	if err != nil {
		return nil, ""
	}
	return s, path
}

func start(logRoot, operation, traceID string) (*Session, string, error) {
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
	day := time.Now().UTC().Format("2006-01-02")
	stamp := time.Now().UTC().Format("20060102T150405Z")
	name := fmt.Sprintf("%s_%s_%s_core.log", stamp, tid, op)
	dir := filepath.Join(logRoot, "logs", "agent", day)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, "", err
	}
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, "", err
	}
	s := &Session{path: path, f: f}
	s.writeHeader(operation, traceID)
	return s, path, nil
}

func (s *Session) writeHeader(operation, traceID string) {
	if s == nil || s.f == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	_, _ = fmt.Fprintf(s.f, "=== GrayDraft Core Agent Session ===\n")
	_, _ = fmt.Fprintf(s.f, "operation: %s\n", operation)
	_, _ = fmt.Fprintf(s.f, "trace_id: %s\n", traceID)
	_, _ = fmt.Fprintf(s.f, "started_utc: %s\n\n", time.Now().UTC().Format(time.RFC3339Nano))
}

// Step logs a named step with JSON fields.
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
		b = append(b[:maxFieldBytes], []byte(fmt.Sprintf("\n... truncated (%d bytes)", len(b)))...)
	}
	_, _ = s.f.Write(b)
	_, _ = fmt.Fprintln(s.f)
	_, _ = fmt.Fprintln(s.f)
}

// StepData logs arbitrary JSON-serializable value under "data".
func (s *Session) StepData(name string, v any) {
	if s == nil {
		return
	}
	b, err := json.Marshal(v)
	if err != nil {
		s.Step(name, map[string]any{"marshal_error": err.Error()})
		return
	}
	var payload any
	_ = json.Unmarshal(b, &payload)
	s.Step(name, map[string]any{"data": payload})
}

// Finish closes the session file.
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
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}
