package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

func (a *App) handleGetWorkspaceDataStats(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, _ json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	kbType, err := a.kbTypeForWorkspaceRoot(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	stats, err := a.Nexus.GetWorkspaceDataStats(ctx, kbType)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (a *App) handleClearWorkspaceData(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		Target string `json:"target"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	target := strings.ToLower(strings.TrimSpace(in.Target))
	if target != "knowledge" && target != "redis" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "target must be knowledge or redis"})
		return
	}
	kbType, err := a.kbTypeForWorkspaceRoot(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	out, err := a.Nexus.PurgeWorkspaceData(ctx, kbType, target)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
