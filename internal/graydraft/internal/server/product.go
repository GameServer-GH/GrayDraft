package server

import (
	"context"
	"encoding/json"
	"net/http"
	"regexp"
	"strings"
)

var mediaMD5Pattern = regexp.MustCompile(`(?i)^[a-f0-9]{32}$`)

func (a *App) handleListMemories(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, _ json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	raw, err := a.Nexus.ListMemories(ctx, scope.TenantID)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, http.StatusOK, raw)
}

func (a *App) handleRevokeMemory(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		MemoryID string `json:"memory_id"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if strings.TrimSpace(in.MemoryID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing memory_id"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.Nexus.RevokeMemory(ctx, scope.TenantID, in.MemoryID); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleGetAgentThread(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		ThreadKey string `json:"thread_key"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if strings.TrimSpace(in.ThreadKey) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing thread_key"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	body, err := a.Nexus.GetProductThread(ctx, scope.TenantID, in.ThreadKey)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, http.StatusOK, body)
}

func (a *App) handleSaveAgentThread(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		ThreadKey string          `json:"thread_key"`
		Payload   json.RawMessage `json:"payload"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if strings.TrimSpace(in.ThreadKey) == "" || len(in.Payload) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "thread_key and payload are required"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.Nexus.PutProductThread(ctx, scope.TenantID, in.ThreadKey, in.Payload); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleDeleteAgentThreads(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		ThreadKey string `json:"thread_key"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.Nexus.DeleteProductThreads(ctx, scope.TenantID, in.ThreadKey); err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleGetMultimodalMedia(ctx context.Context, w http.ResponseWriter, _ string, _ string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		MD5 string `json:"md5"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	md5 := strings.TrimSpace(in.MD5)
	if !mediaMD5Pattern.MatchString(md5) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid md5"})
		return
	}
	body, err := a.Nexus.GetMediaMeta(ctx, md5)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeRawJSON(w, http.StatusOK, body)
}

func writeRawJSON(w http.ResponseWriter, status int, raw []byte) {
	if len(raw) == 0 {
		raw = []byte("{}")
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_, _ = w.Write(raw)
}
