package server

import (
	"context"
	"encoding/json"

	"github.com/graydraft/graydraft/pkg/agentlog"
	"github.com/graydraft/graydraft/pkg/trace"
)

func (a *App) startAgentLog(ctx context.Context, workspaceKey, operation string) *agentlog.Session {
	sess, path, err := agentlog.Start(a.DataDir, workspaceKey, operation, trace.FromContext(ctx))
	if err != nil {
		a.Log.Warn("agent session log unavailable", "operation", operation, "err", err)
		return nil
	}
	a.Log.Info("agent session log", "operation", operation, "path", path, "trace_id", trace.FromContext(ctx))
	return sess
}

func logJSON(sess *agentlog.Session, step string, v any) {
	if sess == nil {
		return
	}
	var payload any
	switch t := v.(type) {
	case json.RawMessage:
		_ = json.Unmarshal(agentlog.SanitizeJSON(t), &payload)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			sess.Step(step, map[string]any{"marshal_error": err.Error()})
			return
		}
		_ = json.Unmarshal(agentlog.SanitizeJSON(b), &payload)
	}
	sess.Step(step, map[string]any{"data": payload})
}
