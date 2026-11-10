// Package server：HTTP 路由，对接前端 bridge POST /api/{Method} 与 Agent SSE。
package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/graydraft/graydraft/pkg/export"
	"github.com/graydraft/graydraft/pkg/index"
	"github.com/graydraft/graydraft/pkg/nexus"
	"github.com/graydraft/graydraft/pkg/notebookdoc"
	"github.com/graydraft/graydraft/pkg/trace"
	"github.com/graydraft/graydraft/pkg/workspace"
)

// App 聚合各域服务。
type App struct {
	Log                *slog.Logger
	DataDir            string
	Nexus              *nexus.Client
	smartImportTimeout time.Duration
	sseBroker          *sseBroker
}

// NewApp 使用同一 data 根目录。
func NewApp(log *slog.Logger, dataDir string) *App {
	if log == nil {
		log = slog.Default()
	}
	nexusCfg := nexus.LoadConfigFromEnv()
	nc := nexus.NewClient(nexusCfg)
	if nc != nil {
		log.Info("nexus client enabled", "core_url", nexusCfg.CoreURL)
	} else {
		log.Warn("nexus client disabled; set GRAYDRAFT_CORE_URL for knowledge base panel")
	}
	t := nexusCfg.SmartImportTimeout
	if t <= 0 {
		t = 15 * time.Minute
	}
	broker := newSSEBroker()
	return &App{
		Log:                log,
		DataDir:            dataDir,
		Nexus:              nc,
		smartImportTimeout: t,
		sseBroker:          broker,
	}
}

// Handler 返回根 mux（含 /health、/api/...）。
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", a.handleHealth)
	mux.HandleFunc("POST /api/knowledge/chat/stream", a.handleKnowledgeChatStream)
	mux.HandleFunc("GET /api/knowledge/library/events", a.handleKnowledgeLibraryEvents)
	mux.HandleFunc("POST /api/", a.handleAPIPost)
	mux.HandleFunc("POST /api/image/recognize", a.handleImageRecognize)
	mux.HandleFunc("POST /api/speech/transcribe", a.handleSpeechTranscribe)
	return a.withLogging(withCORS(mux))
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Access-Control-Allow-Origin", "*")
		h.Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, HEAD")
		h.Set("Access-Control-Allow-Headers", "Content-Type, Accept, "+workspace.HeaderName+", "+trace.HeaderTraceID+", "+trace.HeaderRequestID)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (a *App) handleHealth(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	resp := map[string]string{"status": "ok"}
	if a.Nexus == nil {
		resp["nexus"] = "disabled"
	} else if err := a.Nexus.Health(ctx); err != nil {
		resp["nexus"] = "unreachable"
		resp["nexusError"] = err.Error()
	} else {
		resp["nexus"] = "ok"
	}
	writeJSON(w, http.StatusOK, resp)
}

var (
	errNoWorkspaceKey    = errors.New("missing X-Graydraft-Workspace-Key header")
	errWorkspaceNotFound = errors.New("workspace not found; call OpenWorkspace first")
)

func (a *App) workspaceRootFromHeader(r *http.Request) (string, error) {
	raw := strings.TrimSpace(r.Header.Get(workspace.HeaderName))
	if raw == "" {
		return "", errNoWorkspaceKey
	}
	k, err := workspace.NormalizeKey(raw)
	if err != nil {
		return "", err
	}
	root := workspace.Root(a.DataDir, k)
	st, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errWorkspaceNotFound
		}
		return "", err
	}
	if !st.IsDir() {
		return "", errWorkspaceNotFound
	}
	return root, nil
}

func (a *App) apiHandlerTimeout(method string) time.Duration {
	switch method {
	case "AgentSmartImport":
		if a != nil && a.smartImportTimeout > 0 {
			return a.smartImportTimeout
		}
		return 15 * time.Minute
	default:
		return 90 * time.Second
	}
}

func (a *App) handleAPIPost(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/api/")

	// 子路径分发：兼容旧 Go 版本或 GODEBUG=httpmuxgo121=1 场景，
	// 此时 Go 1.22+ ServeMux 精确模式可能未生效，请求会回退到 POST /api/ 子树模式。
	switch name {
	case "knowledge/chat/stream":
		a.handleKnowledgeChatStream(w, r)
		return
	case "image/recognize":
		a.handleImageRecognize(w, r)
		return
	case "speech/transcribe":
		a.handleSpeechTranscribe(w, r)
		return
	}

	if name == "" || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), a.apiHandlerTimeout(name))
	defer cancel()
	r = r.WithContext(ctx)

	b, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}
	var raw json.RawMessage
	if len(bytes.TrimSpace(b)) == 0 {
		raw = json.RawMessage(`{}`)
	} else if !json.Valid(b) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	} else {
		raw = json.RawMessage(b)
	}

	switch name {
	case "OpenWorkspace":
		var in struct {
			Key string `json:"key"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		root, created, err := workspace.Open(a.DataDir, in.Key)
		if err != nil {
			if errors.Is(err, workspace.ErrInvalidKey) {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		notebooks, settings, err := workspace.Snapshot(ctx, root)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		normKey, _ := workspace.NormalizeKey(in.Key)
		nexusCfg := nexus.LoadConfigFromEnv()
		scope := nexus.ScopeFromWorkspace(normKey, nexusCfg.KBTypePrefix)
		var ensureScope func(context.Context, nexus.KnowledgeScope) error
		var ensureKB func(context.Context, string, string) error
		if a.Nexus != nil {
			ensureScope = a.Nexus.EnsureKnowledgeScope
			ensureKB = a.Nexus.EnsureKBType
		}
		nexusKbType, settings, err := workspace.BindNexusKbType(ctx, root, settings, scope, ensureScope, ensureKB)
		if err != nil {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "nexus kb_type: " + err.Error()})
			return
		}
		if withIntern, changed, err := workspace.EnsureBuiltinInternVendor(settings); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		} else if changed {
			settings = withIntern
			if err := workspace.WriteSettings(workspace.SettingsPath(root), settings); err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"created":       created,
			"notebooks":     notebooks,
			"settings":      settings,
			"nexusKbType":   nexusKbType,
			"nexusTenantId": scope.TenantID,
			"nexusKbId":     scope.KBID,
		})
		return
	}

	wsRoot, err := a.workspaceRootFromHeader(r)
	if err != nil {
		switch {
		case errors.Is(err, errNoWorkspaceKey):
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		case errors.Is(err, errWorkspaceNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}

	idx := index.NewService(wsRoot)
	ex := export.NewService(wsRoot)
	wsKey := strings.TrimSpace(r.Header.Get(workspace.HeaderName))

	switch name {
	case "NexusHealth":
		a.handleNexusHealth(ctx, w)
	case "ListKnowledgeEntries":
		a.handleListKnowledgeEntries(ctx, w, wsRoot, wsKey, raw)
	case "GetKnowledgeEntry":
		a.handleGetKnowledgeEntry(ctx, w, wsRoot, wsKey, raw)
	case "CreateKnowledgeEntry":
		a.handleCreateKnowledgeEntry(ctx, w, wsRoot, wsKey, raw)
	case "UpdateKnowledgeEntry":
		a.handleUpdateKnowledgeEntry(ctx, w, wsRoot, wsKey, raw)
	case "AgentSmartImport":
		a.handleAgentSmartImport(ctx, w, wsRoot, wsKey, raw)
	case "GetMultimodalPromptDefaults":
		a.handleGetMultimodalPromptDefaults(ctx, w, wsRoot)
	case "SaveMultimodalWorkspacePrompt":
		a.handleSaveMultimodalWorkspacePrompt(ctx, w, wsRoot, raw)
	case "GetWorkspaceContext":
		a.handleGetWorkspaceContext(ctx, w, wsRoot, wsKey)
	case "SaveWorkspaceContext":
		a.handleSaveWorkspaceContext(ctx, w, wsRoot, wsKey, raw)
	case "EnrichWorkspaceContext":
		a.handleEnrichWorkspaceContext(ctx, w, wsRoot, wsKey, raw)
	case "GetIngestTask":
		a.handleGetIngestTask(ctx, w, wsRoot, wsKey, raw)
	case "KnowledgeLibraryEvents":
		a.handleKnowledgeLibraryEvents(w, r)
	case "DeleteKnowledgeEntry":
		a.handleDeleteKnowledgeEntry(ctx, w, wsRoot, wsKey, raw)
	case "RetrievalSearch":
		a.handleRetrievalSearch(ctx, w, wsRoot, wsKey, raw)
	case "GetKnowledgeGraph":
		a.handleGetKnowledgeGraph(ctx, w, wsRoot, wsKey, raw)
	case "GetWorkspaceDataStats":
		a.handleGetWorkspaceDataStats(ctx, w, wsRoot, wsKey, raw)
	case "ClearWorkspaceData":
		a.handleClearWorkspaceData(ctx, w, wsRoot, wsKey, raw)
	case "ListMemories":
		a.handleListMemories(ctx, w, wsRoot, wsKey, raw)
	case "RevokeMemory":
		a.handleRevokeMemory(ctx, w, wsRoot, wsKey, raw)
	case "GetAgentThread":
		a.handleGetAgentThread(ctx, w, wsRoot, wsKey, raw)
	case "SaveAgentThread":
		a.handleSaveAgentThread(ctx, w, wsRoot, wsKey, raw)
	case "DeleteAgentThreads":
		a.handleDeleteAgentThreads(ctx, w, wsRoot, wsKey, raw)
	case "GetMultimodalMedia":
		a.handleGetMultimodalMedia(ctx, w, wsRoot, wsKey, raw)
	case "ListNotebooks":
		list, err := idx.ListNotebooks(ctx)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, list)
	case "CreateNotebook":
		var in struct {
			Title string `json:"title,omitempty"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		meta, err := idx.CreateNotebook(ctx, in.Title)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, meta)
	case "RenameNotebook":
		var in struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		if strings.TrimSpace(in.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
			return
		}
		meta, err := idx.RenameNotebook(ctx, in.ID, in.Title)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, meta)
	case "DeleteNotebook":
		var in struct {
			ID string `json:"id"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		if strings.TrimSpace(in.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
			return
		}
		if err := idx.DeleteNotebook(ctx, in.ID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
				return
			}
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		_ = notebookdoc.Delete(wsRoot, in.ID)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "GetNotebookDocument":
		var in struct {
			ID string `json:"id"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		if strings.TrimSpace(in.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
			return
		}
		d, err := notebookdoc.Read(wsRoot, in.ID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		if d == nil {
			writeJSON(w, http.StatusOK, map[string]string{"html": "", "css": ""})
			return
		}
		writeJSON(w, http.StatusOK, d)
	case "SaveNotebookDocument":
		var in struct {
			ID   string `json:"id"`
			HTML string `json:"html"`
			CSS  string `json:"css"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		if strings.TrimSpace(in.ID) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing id"})
			return
		}
		if err := notebookdoc.Write(wsRoot, in.ID, &notebookdoc.Document{HTML: in.HTML, CSS: in.CSS}); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "SaveWorkspaceSettings":
		var in struct {
			Settings json.RawMessage `json:"settings"`
		}
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &in); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
				return
			}
		}
		if len(bytes.TrimSpace(in.Settings)) == 0 {
			in.Settings = json.RawMessage(`{}`)
		}
		if err := workspace.WriteSettings(workspace.SettingsPath(wsRoot), in.Settings); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid settings json"})
			return
		}
		a.syncWorkspaceContextToNexus(ctx, wsRoot, wsKey)
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case "ExportStatic":
		var in struct {
			ProjectID string `json:"projectId"`
		}
		if len(raw) > 0 {
			_ = json.Unmarshal(raw, &in)
		}
		jobID, err := ex.Start(in.ProjectID)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"jobId": jobID})
	default:
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "unknown method: " + name})
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func isClientGone(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, syscall.EPIPE) || errors.Is(err, syscall.ECONNRESET) {
		return true
	}
	if s := err.Error(); strings.Contains(s, "broken pipe") || strings.Contains(s, "connection reset") {
		return true
	}
	return false
}

func (a *App) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		incoming := r.Header.Get(trace.HeaderTraceID)
		if incoming == "" {
			incoming = r.Header.Get(trace.HeaderRequestID)
		}
		traceID := trace.Resolve(incoming)
		ctx := trace.WithContext(r.Context(), traceID)
		r = r.WithContext(ctx)
		w.Header().Set(trace.HeaderTraceID, traceID)
		w.Header().Set(trace.HeaderRequestID, traceID)
		next.ServeHTTP(w, r)
		a.Log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"trace_id", traceID,
			"dur", time.Since(start).String(),
		)
	})
}
