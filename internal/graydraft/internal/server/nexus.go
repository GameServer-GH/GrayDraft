package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/graydraft/graydraft/pkg/nexus"
	"github.com/graydraft/graydraft/pkg/workspace"
)

var (
	errNexusDisabled = errors.New("nexus not configured")
	errRawRequired   = errors.New("raw_text or attachments required")
)

func (a *App) nexusUnavailable() bool {
	return a.Nexus == nil
}

func (a *App) kbTypeForWorkspaceRoot(wsRoot, headerKey string) (string, error) {
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		return "", err
	}
	return scope.StoragePartition, nil
}

func (a *App) knowledgeScopeForWorkspace(wsRoot, headerKey string) (nexus.KnowledgeScope, error) {
	norm, err := workspace.NormalizeKey(headerKey)
	if err != nil {
		return nexus.KnowledgeScope{}, err
	}
	cfg := nexus.LoadConfigFromEnv()
	scope := nexus.ScopeFromWorkspace(norm, cfg.KBTypePrefix)
	if kb, err := workspace.ReadNexusKbType(wsRoot); err == nil && kb != "" {
		scope.StoragePartition = kb
	}
	if tenant, kb, err := workspace.ReadNexusScope(wsRoot); err == nil && tenant != "" && kb != "" {
		scope.TenantID = tenant
		scope.KBID = kb
	}
	if a.Nexus == nil {
		if scope.StoragePartition == "" {
			return nexus.KnowledgeScope{}, errors.New("nexus not configured and workspace has no nexusKbType")
		}
		return scope, nil
	}
	return scope, nil
}

func (a *App) handleNexusHealth(ctx context.Context, w http.ResponseWriter) {
	if a.Nexus == nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "nexus": "disabled"})
		return
	}
	if err := a.Nexus.Health(ctx); err != nil {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "nexus": "unreachable", "nexusError": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "nexus": "ok"})
}

func (a *App) handleListKnowledgeEntries(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var in struct {
		Limit  int `json:"limit"`
		Offset int `json:"offset"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if in.Limit <= 0 {
		in.Limit = 50
	}
	list, err := a.Nexus.ListDocuments(ctx, scope, in.Limit, in.Offset)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (a *App) handleGetKnowledgeEntry(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
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
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	doc, err := a.Nexus.GetDocument(ctx, scope, in.ID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(doc)
}

func (a *App) handleCreateKnowledgeEntry(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		Title      string         `json:"title"`
		Content    string         `json:"content"`
		SourceURL  string         `json:"source_url"`
		Extensions map[string]any `json:"extensions"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	title := strings.TrimSpace(in.Title)
	content := strings.TrimSpace(in.Content)
	if title == "" || content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "title and content required"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp, err := a.Nexus.UGCNotify(ctx, scope, nexus.UGCNotifyInput{
		Title:      title,
		Content:    content,
		SourceURL:  strings.TrimSpace(in.SourceURL),
		Extensions: in.Extensions,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
	a.broadcastLibraryEvent("library-changed", map[string]any{"action": "create", "id": resp.DocID})
}

func (a *App) handleUpdateKnowledgeEntry(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		ID         string         `json:"id"`
		Title      string         `json:"title"`
		Content    string         `json:"content"`
		SourceURL  string         `json:"source_url"`
		Extensions map[string]any `json:"extensions"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	docID := strings.TrimSpace(in.ID)
	title := strings.TrimSpace(in.Title)
	content := strings.TrimSpace(in.Content)
	if docID == "" || title == "" || content == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "id, title and content required"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	resp, err := a.Nexus.UGCNotify(ctx, scope, nexus.UGCNotifyInput{
		Title:      title,
		Content:    content,
		SourceURL:  strings.TrimSpace(in.SourceURL),
		DocID:      docID,
		Extensions: in.Extensions,
	})
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusAccepted, resp)
	a.broadcastLibraryEvent("library-changed", map[string]any{"action": "update", "id": in.ID})
}

func (a *App) handleAgentSmartImport(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	sess := a.startAgentLog(ctx, headerKey, "smart-import")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()

	if a.nexusUnavailable() {
		runErr = errNexusDisabled
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	if sess != nil {
		logJSON(sess, "request_in", raw)
	}

	var in struct {
		nexus.SmartImportBody
		AgentModel string `json:"agent_model"`
		// Media fields for image/video recognition before smart-import
		MediaImageBase64   string `json:"media_image_base64"`
		MediaImageMimeType string `json:"media_image_mime_type"`
		MediaVideoBase64   string `json:"media_video_base64"`
		MediaVideoMimeType string `json:"media_video_mime_type"`
		MediaURL           string `json:"media_url"`
	}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &in); err != nil {
			runErr = err
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid body"})
			return
		}
	}
	hasSmartImportMedia := strings.TrimSpace(in.MediaImageBase64) != "" || strings.TrimSpace(in.MediaVideoBase64) != "" || strings.TrimSpace(in.MediaURL) != ""
	if strings.TrimSpace(in.RawText) == "" && len(in.Attachments) == 0 && !hasSmartImportMedia {
		runErr = errRawRequired
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "raw_text, attachments, or media required"})
		return
	}

	// ── Unified media recognition for SmartImport (image/video base64 or URL) ──
	imageBase64 := strings.TrimSpace(in.MediaImageBase64)
	videoBase64 := strings.TrimSpace(in.MediaVideoBase64)
	mediaURL := strings.TrimSpace(in.MediaURL)
	hasImage := imageBase64 != ""
	hasVideo := videoBase64 != ""
	hasURL := mediaURL != ""

	if hasImage || hasVideo || hasURL {
		llmOverride, _ := workspace.ResolveLLMOverride(wsRoot, "auto")
		if llmOverride == nil {
			llmOverride = nexus.SystemLLMOverride()
		}
		coreURL := ""
		if a.Nexus != nil {
			coreURL = a.Nexus.CoreURL()
		}

		var mediaDataURL string
		var kind nexus.MediaKind
		var attachmentLabel string

		switch {
		case hasVideo:
			videoMime := strings.TrimSpace(in.MediaVideoMimeType)
			if videoMime == "" {
				videoMime = "video/mp4"
			}
			if !allowedVideoMimes[videoMime] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported video type: " + videoMime})
				return
			}
			decoded, err := decodeBase64(videoBase64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 video: " + err.Error()})
				return
			}
			if len(decoded) > maxVideoBytes {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("video too large (%d bytes, max %d)", len(decoded), maxVideoBytes)})
				return
			}
			defer zeroBytes(decoded)
			mediaDataURL = fmt.Sprintf("data:%s;base64,%s", videoMime, base64.StdEncoding.EncodeToString(decoded))
			kind = nexus.MediaKindVideo
			attachmentLabel = "视频识别"

		case hasImage:
			imageMime := strings.TrimSpace(in.MediaImageMimeType)
			if imageMime == "" {
				imageMime = "image/png"
			}
			if !allowedImageMimes[imageMime] {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported image type: " + imageMime})
				return
			}
			decoded, err := decodeBase64(imageBase64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 image: " + err.Error()})
				return
			}
			if len(decoded) > maxImageBytes {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("image too large (%d bytes, max %d)", len(decoded), maxImageBytes)})
				return
			}
			defer zeroBytes(decoded)
			mediaDataURL = fmt.Sprintf("data:%s;base64,%s", imageMime, base64.StdEncoding.EncodeToString(decoded))
			kind = nexus.MediaKindImage
			attachmentLabel = "图片识别"

		case hasURL:
			kind = detectMediaKindFromURL(mediaURL)
			if kind == nexus.MediaKindVideo {
				if !allowedVideoURL(mediaURL) {
					writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported video URL; must end with .mp4, .webm, or .mov"})
					return
				}
			}
			mediaDataURL = mediaURL
			if kind == nexus.MediaKindVideo {
				attachmentLabel = "视频链接识别"
			} else {
				attachmentLabel = "图片链接识别"
			}
		}

		// Describe the media via VLM
		var descPrompt string
		if kind == nexus.MediaKindVideo {
			descPrompt = "你是一个知识库内容提取助手。请从这段视频中提取有价值的知识内容，用于存入知识库。\n\n要求：\n1. 提取视频中的关键信息（文字、对话、操作步骤等），忽略片头片尾、水印、装饰元素\n2. 如果是教学视频，只提取核心知识点和操作流程，不要描述讲解者的表情、语气\n3. 如果是演示视频，按时间顺序提取操作步骤\n4. 最终输出应该是结构化的知识文档内容\n5. 用中文输出，禁止使用 Emoji"
		} else {
			descPrompt = "你是一个知识库内容提取助手。请从这张图片中提取有价值的知识内容，用于存入知识库。\n\n要求：\n1. 提取图片中的文字信息和数据，但严格忽略以下无关元素：\n   - 水印、版权声明、免责声明（如「素材仅作展示」「具体以实际效果为准」「出品方：XX」）\n   - 页面装饰性标注（如「底部标注」「顶部主视觉」「水印」「备注：内容仅供展示」）\n   - UI 布局描述词（如「顶部」「底部」「左侧」「右侧」等纯位置词）\n2. 保留所有实质性的知识信息：活动名称、时间、模型、参数、课程、优化内容、版本号等\n3. 如果是表格或列表形式的内容，转换为清晰的文本结构\n4. 最终输出应该是结构化的知识文档内容，适合直接存入知识库\n5. 用中文输出，禁止使用 Emoji"
		}

		// Detach from request context: media recognition may take 60+ seconds.
		// If the client disconnects, we do not want to cancel the downstream call.
		bgCtx := context.Background()
		mediaDescription, descErr := a.chatWithMedia(bgCtx, kind, mediaDataURL, descPrompt, llmOverride, coreURL)
		if descErr != nil {
			runErr = descErr
			a.Log.Error("smart-import: media describe failed", "err", descErr, "kind", kind)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description failed: " + descErr.Error()})
			return
		}
		mediaDescription = strings.TrimSpace(mediaDescription)
		if mediaDescription == "" {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description returned empty"})
			return
		}

		// Clean the media description: strip UI chrome and noise patterns
		// before appending to raw_text for smart-import
		mediaDescription = cleanSmartImportMediaDescription(mediaDescription)
		// Append media description as attachment content to raw_text
		if in.RawText != "" {
			in.RawText = fmt.Sprintf("%s\n\n---\n附件内容（%s）：\n%s", in.RawText, attachmentLabel, mediaDescription)
		} else {
			in.RawText = fmt.Sprintf("附件内容（%s）：\n%s", attachmentLabel, mediaDescription)
		}

		if sess != nil {
			preview := mediaDescription
			if len(preview) > 200 {
				preview = preview[:200] + "..."
			}
			sess.Step("smart_import_media_recognized", map[string]any{
				"kind":                      string(kind),
				"media_description_length":  len(mediaDescription),
				"media_description_preview": preview,
			})
		}
	}

	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		runErr = err
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	norm, err := workspace.NormalizeKey(headerKey)
	if err != nil {
		runErr = err
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	override, err := workspace.ResolveLLMOverride(wsRoot, in.AgentModel)
	if err != nil {
		runErr = err
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if override == nil {
		override = nexus.DefaultTextLLMOverride()
	}
	body := in.SmartImportBody
	body.LLMOverride = override
	if wsPrompt, err := workspace.ReadAgentUnstructuredPrompt(wsRoot); err == nil && strings.TrimSpace(body.WorkspacePromptOverride) == "" {
		body.WorkspacePromptOverride = strings.TrimSpace(wsPrompt)
	}
	if profile, wcText, err := workspace.WorkspaceContextBundle(wsRoot); err == nil {
		if strings.TrimSpace(body.WorkspaceContext) == "" {
			body.WorkspaceContext = wcText
		}
		if body.ContextProfile == nil && (profile.Description != "" || len(profile.Entities) > 0) {
			p := profile
			body.ContextProfile = &p
		}
	}
	attDetails := make([]map[string]any, 0, len(body.Attachments))
	for _, att := range body.Attachments {
		d := map[string]any{
			"filename":  att.Filename,
			"mime_type": att.MimeType,
		}
		if att.DataURL != "" {
			d["data_url"] = att.DataURL
		}
		if att.DataBase64 != "" {
			d["data_base64_bytes"] = len(att.DataBase64)
		}
		if att.DataURL == "" && att.DataBase64 == "" {
			d["warning"] = "no data_url or data_base64 provided"
		}
		attDetails = append(attDetails, d)
	}
	logJSON(sess, "bff_prepared", map[string]any{
		"scope":                scope,
		"workspace_key":        norm,
		"agent_model":          in.AgentModel,
		"workspace_prompt_len": len(body.WorkspacePromptOverride),
		"attachments":          len(body.Attachments),
		"attachment_details":   attDetails,
		"split":                body.Split,
		"submit":               body.Submit,
		"raw_text_len":         len(body.RawText),
	})
	// Detach from request context: smart-import with multimodal processing may take minutes.
	// Client disconnect should not cancel the Nexus processing.
	bgCtx := context.Background()
	out, err := a.Nexus.SmartImport(bgCtx, scope, norm, body)
	if err != nil {
		runErr = err
		if sess != nil {
			sess.Step("nexus_error", map[string]any{"error": err.Error()})
		}
		if len(out) > 0 {
			// Nexus core returned a structured response body with
			// confidence/preview details. Inject a user-friendly error
			// field before forwarding to the frontend.
			logJSON(sess, "nexus_error_body", out)
			friendlyBody := injectUserFriendlyError(out, "导入暂未成功")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnprocessableEntity)
			_, _ = w.Write(friendlyBody)
		} else {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		}
		return
	}
	logJSON(sess, "nexus_response", out)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (a *App) handleGetMultimodalPromptDefaults(ctx context.Context, w http.ResponseWriter, wsRoot string) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	def, err := a.Nexus.GetMultimodalPromptDefaults(ctx)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	wsPrompt, _ := workspace.ReadAgentUnstructuredPrompt(wsRoot)
	writeJSON(w, http.StatusOK, map[string]any{
		"base_prompt":      def.BasePrompt,
		"enabled":          def.Enabled,
		"workspace_prompt": wsPrompt,
	})
}

func (a *App) handleSaveMultimodalWorkspacePrompt(ctx context.Context, w http.ResponseWriter, wsRoot string, raw json.RawMessage) {
	var in struct {
		WorkspacePrompt string `json:"workspace_prompt"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if err := workspace.WriteMultimodalWorkspacePrompt(wsRoot, strings.TrimSpace(in.WorkspacePrompt)); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleGetIngestTask(ctx context.Context, w http.ResponseWriter, _ string, _ string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	var in struct {
		TaskID string `json:"task_id"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	if strings.TrimSpace(in.TaskID) == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "missing task_id"})
		return
	}
	st, err := a.Nexus.GetIngestTask(ctx, in.TaskID)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, st)
}

func (a *App) handleDeleteKnowledgeEntry(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
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
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := a.Nexus.DeleteDocument(ctx, scope, in.ID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *App) handleRetrievalSearch(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	sess := a.startAgentLog(ctx, headerKey, "retrieval-search")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()

	if a.nexusUnavailable() {
		runErr = errNexusDisabled
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	if sess != nil {
		logJSON(sess, "request_in", raw)
	}
	var in struct {
		Query     string `json:"query"`
		TopK      int    `json:"top_k"`
		SkipCache bool   `json:"skip_cache"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	query := strings.TrimSpace(in.Query)
	if query == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	topK := in.TopK
	if topK <= 0 {
		topK = 5
	}
	if topK > 50 {
		topK = 50
	}
	wsPrompt, _ := workspace.ReadAgentUnstructuredPrompt(wsRoot)
	profile, wcText, _ := workspace.WorkspaceContextBundle(wsRoot)
	wc := wcText
	if wc == "" {
		wc = strings.TrimSpace(wsPrompt)
	}
	var profilePtr *nexus.ContextProfile
	if profile.Description != "" || len(profile.Entities) > 0 {
		p := profile
		profilePtr = &p
	}
	out, err := a.Nexus.RetrievalSearch(ctx, nexus.RetrievalSearchRequest{
		Query:            query,
		TenantID:         scope.TenantID,
		KBID:             scope.KBID,
		TopK:             topK,
		SkipCache:        in.SkipCache,
		WorkspaceContext: wc,
		ContextProfile:   profilePtr,
	})
	if err != nil {
		runErr = err
		if sess != nil {
			sess.Step("nexus_error", map[string]any{"error": err.Error()})
		}
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	logJSON(sess, "nexus_response", out)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(out)
}

func (a *App) handleGetKnowledgeGraph(ctx context.Context, w http.ResponseWriter, wsRoot, headerKey string, raw json.RawMessage) {
	if a.nexusUnavailable() {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	scope, err := a.knowledgeScopeForWorkspace(wsRoot, headerKey)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var in struct {
		Limit int `json:"limit"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	graph, err := a.Nexus.GetDocumentGraph(ctx, scope, in.Limit)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, graph)
}

// injectUserFriendlyError parses a Nexus error response body, appends a
// human-readable "error" field derived from the Nexus "message" field,
// and returns the modified JSON. If parsing fails, returns a minimal
// fallback JSON with just the canned error string.
func injectUserFriendlyError(raw json.RawMessage, fallbackPrefix string) json.RawMessage {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		if b, _ := json.Marshal(map[string]string{"error": fallbackPrefix + "，请检查图片是否清晰且包含可识别的知识内容。"}); len(b) > 0 {
			return b
		}
		return nil
	}
	msg, _ := body["message"].(string)
	importAllowed, _ := body["import_allowed"].(bool)
	conf, _ := body["confidence"].(float64)
	minConf, _ := body["min_confidence_required"].(float64)

	var userMsg string
	if msg != "" {
		userMsg = fallbackPrefix + "：" + msg
	} else if !importAllowed && conf >= minConf {
		userMsg = fallbackPrefix + "：图片内容质量不符合知识库入库标准，请确保图片清晰且包含有意义的文字内容。"
	} else {
		userMsg = fallbackPrefix + "，请检查图片是否清晰且包含可识别的知识内容，然后重新尝试。"
	}
	body["error"] = userMsg
	if b, err := json.Marshal(body); err == nil && len(b) > 0 {
		return b
	}
	return nil
}

// cleanSmartImportMediaDescription strips common UI chrome and noise patterns
// from VLM-generated image/video descriptions before they are passed to
// smart-import. This prevents irrelevant labels like "底部标注", "水印",
// "素材仅作展示" from polluting knowledge base content.
func cleanSmartImportMediaDescription(desc string) string {
	// Patterns to remove: lines that consist solely of or start with
	// known UI chrome / disclaimer patterns.
	noisePrefixes := []string{
		"底部标注",
		"顶部主视觉",
		"水印",
		"水印",
		"备注：内容仅供展示",
		"素材仅作展示",
		"出品方：",
		"提示素材仅作展示",
		"注：素材仅作展示",
		"*注：素材仅作展示",
		"备注：素材仅作展示",
		"*注：内容仅供展示",
		"免责声明",
		"版权声明",
		"©",
		"copyright",
		"水印：",
		"底部水印",
		"具体以文档为准",
		"具体以课件为准",
		"内容仅供展示",
	}
	// Substrings that indicate a full line is a UI layout description
	uiLayoutMarkers := []string{
		"顶部：",
		"底部：",
		"左侧：",
		"右侧：",
		"中间：",
		"页面顶部",
		"页面底部",
		"左上角",
		"右上角",
		"左下角",
		"右下角",
	}

	var kept []string
	for _, line := range strings.Split(desc, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			kept = append(kept, line)
			continue
		}

		// Check if this line is pure noise
		isNoise := false
		for _, prefix := range noisePrefixes {
			if strings.HasPrefix(trimmed, prefix) {
				isNoise = true
				break
			}
		}
		if isNoise {
			continue
		}

		// Check if this line is purely a UI layout description
		runes := []rune(trimmed)
		if len(runes) <= 15 {
			for _, marker := range uiLayoutMarkers {
				if strings.HasPrefix(trimmed, marker) {
					isNoise = true
					break
				}
			}
		}
		if isNoise {
			continue
		}

		kept = append(kept, line)
	}

	result := strings.TrimSpace(strings.Join(kept, "\n"))
	result = collapseBlankLines(result)
	return result
}

// collapseBlankLines reduces 3+ consecutive blank lines to at most 2.
func collapseBlankLines(s string) string {
	lines := strings.Split(s, "\n")
	var out []string
	blankCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			blankCount++
			if blankCount <= 2 {
				out = append(out, line)
			}
		} else {
			blankCount = 0
			out = append(out, line)
		}
	}
	return strings.TrimSpace(strings.Join(out, "\n"))
}
