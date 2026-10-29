package server

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/graydraft/graydraft/pkg/workspace"
)

func (a *App) syncWorkspaceContextToNexus(ctx context.Context, wsRoot, headerKey string) {
	if a.Nexus == nil {
		return
	}
	kbType, err := a.kbTypeForWorkspace(wsRoot, headerKey)
	if err != nil || kbType == "" {
		return
	}
	local, err := workspace.ReadContextProfile(wsRoot)
	if err != nil {
		return
	}
	_ = a.Nexus.PutWorkspaceContext(ctx, kbType, local.ToNexus())
}

func (a *App) kbTypeForWorkspace(wsRoot, headerKey string) (string, error) {
	if strings.TrimSpace(headerKey) != "" {
		return a.kbTypeForWorkspaceRoot(wsRoot, headerKey)
	}
	if kb := workspace.NexusKbTypeFromRoot(wsRoot); kb != "" {
		return kb, nil
	}
	return "", errNoWorkspaceKey
}

func (a *App) handleGetWorkspaceContext(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string) {
	local, _ := workspace.ReadContextProfile(wsRoot)

	if a.nexusUnavailable() {
		writeJSON(w, http.StatusOK, local)
		return
	}
	kbType, err := a.kbTypeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	profile, err := a.Nexus.GetWorkspaceContext(ctx, kbType)
	if err != nil {
		// Core unreachable or profile empty: still return local settings copy.
		if local.Description != "" || len(local.Entities) > 0 || len(local.Rules) > 0 {
			writeJSON(w, http.StatusOK, local)
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	out := workspace.ContextProfileFromNexus(profile)
	if out.Description == "" && len(out.Entities) == 0 && len(out.Rules) == 0 {
		if local.Description != "" || len(local.Entities) > 0 {
			writeJSON(w, http.StatusOK, local)
			return
		}
	}
	writeJSON(w, http.StatusOK, out)
}

func (a *App) handleEnrichWorkspaceContext(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		Description string `json:"description"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	desc := strings.TrimSpace(in.Description)
	if desc == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "description required"})
		return
	}
	kbType, err := a.kbTypeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	profile, err := a.Nexus.EnrichWorkspaceContext(ctx, kbType, desc)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	local := workspace.ContextProfileFromNexus(profile)
	path := workspace.SettingsPath(wsRoot)
	settingsRaw, err := workspace.ReadSettingsRaw(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	merged := workspace.MergeContextProfile(settingsRaw, local)
	if err := workspace.WriteSettings(path, merged); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "profile": local})
}

func (a *App) handleSaveWorkspaceContext(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	var in workspace.ContextProfileSettings
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	path := workspace.SettingsPath(wsRoot)
	settingsRaw, err := workspace.ReadSettingsRaw(path)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	merged := workspace.MergeContextProfile(settingsRaw, in)
	if err := workspace.WriteSettings(path, merged); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	a.syncWorkspaceContextToNexus(ctx, wsRoot, headerKey)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}
