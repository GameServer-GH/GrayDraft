package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/graydraft/graydraft/pkg/nexus"
	"github.com/graydraft/graydraft/pkg/workspace"
)

// maxVideoBytes is the maximum base64-decoded video size (100 MB).
const maxVideoBytes = 100 * 1024 * 1024

// allowedVideoMimes lists accepted video MIME types.
var allowedVideoMimes = map[string]bool{
	"video/mp4":  true,
	"video/webm": true,
	"video/quicktime": true, // .mov
}

func (a *App) handleKnowledgeChatStream(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	wsKey := strings.TrimSpace(r.Header.Get(workspace.HeaderName))
	sess := a.startAgentLog(r.Context(), wsKey, "chat-stream")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()

	if a.Nexus == nil {
		runErr = errNexusDisabled
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}
	wsRoot, err := a.workspaceRootFromHeader(r)
	if err != nil {
		runErr = err
		switch {
		case err == errNoWorkspaceKey:
			writeJSON(w, http.StatusUnauthorized, map[string]string{"error": err.Error()})
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		}
		return
	}
	raw, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	var in struct {
		Query           string `json:"query"`
		ChatMode        string `json:"chat_mode"`
		SessionID       string `json:"session_id"`
		TopK            int    `json:"top_k"`
		SkipCache       bool   `json:"skip_cache"`
		SkipAnswerCache bool   `json:"skip_answer_cache"`
		AgentModel      string `json:"agent_model"`
		ThinkingMode    *bool  `json:"thinking_mode"`
		// Image / Video base64 fields
		ImageBase64   string `json:"image_base64"`
		ImageMimeType string `json:"image_mime_type"`
		VideoBase64   string `json:"video_base64"`
		VideoMimeType string `json:"video_mime_type"`
		// URL to an image or video (publicly accessible)
		MediaURL string `json:"media_url"`
	}
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &in)
	}
	query := strings.TrimSpace(in.Query)

	// ── Unified media recognition (image/video base64 or URL) ──
	imageBase64 := strings.TrimSpace(in.ImageBase64)
	videoBase64 := strings.TrimSpace(in.VideoBase64)
	mediaURL := strings.TrimSpace(in.MediaURL)
	hasImage := imageBase64 != ""
	hasVideo := videoBase64 != ""
	hasURL := mediaURL != ""

	if hasImage || hasVideo || hasURL {
		llmOverride := a.resolveLLMForImage(r)
		coreURL := ""
		if a.Nexus != nil {
			coreURL = a.Nexus.CoreURL()
		}

		var mediaDataURL string
		var kind nexus.MediaKind
		var attachmentLabel string

		switch {
		case hasVideo:
			videoMime := strings.TrimSpace(in.VideoMimeType)
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
			imageMime := strings.TrimSpace(in.ImageMimeType)
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
			// Detect media kind from URL extension or content-type
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

		// Step 1: Describe the media via VLM
		var descPrompt string
		if kind == nexus.MediaKindVideo {
			descPrompt = "请仔细观看这个视频，描述视频中的场景、人物、动作、对话、文字等所有关键信息。如果是教学视频，提取核心知识；如果是演示视频，描述操作流程；请用中文详细描述。"
		} else {
			descPrompt = "请仔细描述这张图片的内容，包括其中的文字、数据、图表、结构等所有关键信息。如果是表格，列出表头和关键数据；如果是截图，描述界面布局和关键信息；如果是文档，提取核心内容。请用中文描述。"
		}

		// Detach from request context: VLM media description may take 20+ seconds.
		// Client disconnect should not cancel the media recognition.
		mediaDescription, descErr := a.chatWithMedia(context.Background(), kind, mediaDataURL, descPrompt, llmOverride, coreURL)
		if descErr != nil {
			a.Log.Error("knowledge chat: media describe failed", "err", descErr, "kind", kind)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description failed: " + descErr.Error()})
			return
		}
		mediaDescription = strings.TrimSpace(mediaDescription)
		if mediaDescription == "" {
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description returned empty"})
			return
		}

		// Step 2: Clean / extract question from description + user text
		// Detach from request context: question cleaning may take 5-10 seconds.
		cleanedQuestion, cleanErr := a.cleanMediaQuestion(context.Background(), kind, mediaDescription, query, llmOverride, coreURL)
		if cleanErr != nil {
			a.Log.Error("knowledge chat: media clean failed", "err", cleanErr)
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "question cleaning failed: " + cleanErr.Error()})
			return
		}
		cleanedQuestion = strings.TrimSpace(cleanedQuestion)
		if cleanedQuestion == "" {
			if kind == nexus.MediaKindVideo {
				cleanedQuestion = fmt.Sprintf("请根据以下视频内容回答问题：%s", mediaDescription)
			} else {
				cleanedQuestion = fmt.Sprintf("请根据以下图片内容回答问题：%s", mediaDescription)
			}
		}

		// Append media description as attachment content to the query
		if query != "" {
			query = fmt.Sprintf("%s\n\n---\n附件内容（%s）：\n%s", query, attachmentLabel, mediaDescription)
		} else {
			query = cleanedQuestion
		}

		if sess != nil {
			sess.Step("media_recognized", map[string]any{
				"kind":                     string(kind),
				"media_description_length": len(mediaDescription),
				"cleaned_question":         cleanedQuestion,
			})
		}
	}

	if query == "" {
		runErr = errRawRequired
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "query required"})
		return
	}

	scope, err := a.knowledgeScopeForWorkspace(wsRoot, wsKey)
	if err != nil {
		runErr = err
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	mode := strings.TrimSpace(in.ChatMode)
	if mode == "" {
		mode = "hybrid"
	}
	topK := in.TopK
	if topK <= 0 {
		topK = 5
	}

	override, err := workspace.ResolveLLMOverride(wsRoot, in.AgentModel)
	if err != nil {
		runErr = err
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if sess != nil {
		sess.Step("request_in", map[string]any{
			"query":             query,
			"chat_mode":         mode,
			"session_id":        in.SessionID,
			"top_k":             topK,
			"agent_model":       in.AgentModel,
			"thinking_mode":     in.ThinkingMode == nil || *in.ThinkingMode,
			"skip_cache":        in.SkipCache,
			"skip_answer_cache": in.SkipAnswerCache,
			"has_image":         hasImage,
			"has_video":         hasVideo,
			"has_media_url":     hasURL,
		})
	}

	profile, wcText, _ := workspace.WorkspaceContextBundle(wsRoot)
	var profilePtr *nexus.ContextProfile
	if profile.Description != "" || len(profile.Entities) > 0 {
		p := profile
		profilePtr = &p
	}
	req := nexus.ChatStreamRequest{
		Query:            query,
		TenantID:         scope.TenantID,
		KBID:             scope.KBID,
		KBType:           scope.StoragePartition,
		TopK:             topK,
		SessionID:        strings.TrimSpace(in.SessionID),
		ChatMode:         mode,
		SkipCache:        in.SkipCache,
		SkipAnswerCache:  in.SkipAnswerCache,
		IncludeCold:      true,
		LLMOverride:      override,
		ThinkingMode:     in.ThinkingMode,
		WorkspaceContext: wcText,
		ContextProfile:   profilePtr,
	}
	var sseLines int
	// Detach from request context: core streaming with LLM may take minutes.
	// User disconnecting their browser tab should not cancel the Core stream.
	if err := a.Nexus.ProxyChatStreamWithHook(context.Background(), req, w, func(line string) {
		if sess == nil {
			return
		}
		sseLines++
		if strings.HasPrefix(line, "data:") {
			sess.StepRaw("sse_data", line)
		}
	}); err != nil {
		runErr = err
		a.Log.Error("knowledge chat stream", "err", err)
		if fl, ok := w.(http.Flusher); ok {
			_, _ = w.Write([]byte(`data: {"type":"error","message":"` + strings.ReplaceAll(err.Error(), `"`, `\"`) + `"}` + "\n\n"))
			fl.Flush()
		}
		return
	}
	if sess != nil {
		sess.Step("stream_done", map[string]any{"sse_lines": sseLines})
	}
}

// decodeBase64 tries to decode a base64 string, stripping data URL prefix if present.
func decodeBase64(b64 string) ([]byte, error) {
	b64 = strings.TrimSpace(b64)
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		if idx := strings.Index(b64, ","); idx >= 0 {
			b64 = b64[idx+1:]
		}
		decoded, err = base64.StdEncoding.DecodeString(b64)
	}
	return decoded, err
}

// zeroBytes overwrites a byte slice with zeros (for secure cleanup).
func zeroBytes(b []byte) {
	for i := range b {
		b[i] = 0
	}
}

// detectMediaKindFromURL guesses whether a URL points to an image or video
// based on the file extension.
func detectMediaKindFromURL(rawURL string) nexus.MediaKind {
	lower := strings.ToLower(rawURL)
	// Strip query string and fragment for extension check
	if idx := strings.Index(lower, "?"); idx >= 0 {
		lower = lower[:idx]
	}
	if idx := strings.Index(lower, "#"); idx >= 0 {
		lower = lower[:idx]
	}
	for _, ext := range []string{".mp4", ".webm", ".mov", ".avi", ".mkv", ".flv", ".wmv"} {
		if strings.HasSuffix(lower, ext) {
			return nexus.MediaKindVideo
		}
	}
	return nexus.MediaKindImage
}

// allowedVideoURL checks if the URL extension is a permitted video format.
func allowedVideoURL(rawURL string) bool {
	lower := strings.ToLower(rawURL)
	if idx := strings.Index(lower, "?"); idx >= 0 {
		lower = lower[:idx]
	}
	if idx := strings.Index(lower, "#"); idx >= 0 {
		lower = lower[:idx]
	}
	for _, ext := range []string{".mp4", ".webm", ".mov"} {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}

// chatWithMedia sends a multimodal chat request (image or video + text) and returns the response.
func (a *App) chatWithMedia(ctx context.Context, kind nexus.MediaKind, mediaURL, prompt string, override *nexus.LLMOverride, coreURL string) (string, error) {
	msg := nexus.NewMediaMessage("user", kind, mediaURL, prompt)
	return a.Nexus.MultimodalChatCompletion(ctx, []nexus.MultimodalMessage{msg}, override, coreURL)
}

// chatWithImage sends a multimodal chat request (image + text) and returns the response.
func (a *App) chatWithImage(ctx context.Context, imageDataURL, prompt string, override *nexus.LLMOverride, coreURL string) (string, error) {
	messages := []nexus.MultimodalMessage{
		nexus.NewVisionMessage("user", []string{imageDataURL}, prompt),
	}
	return a.Nexus.MultimodalChatCompletion(ctx, messages, override, coreURL)
}

// cleanImageQuestion combines image description and user text into a cleaned question.
func (a *App) cleanImageQuestion(ctx context.Context, imageDescription, userText string, override *nexus.LLMOverride, coreURL string) (string, error) {
	cleanPrompt := fmt.Sprintf(`你是一个问题整理助手。用户上传了一张图片，图片的VLM识别描述如下：

%s

用户还输入了以下文字（可能为空）：
%s

请结合图片描述和用户文字，推断用户真正想询问的问题，整理成一句完整、清晰的问题文本。

注意：
- 如果用户文字已经是一个完整的问题，保留并可能用图片信息补充上下文使其更精确
- 如果用户文字很简短（如"这个"、"解释一下"、"为什么"），用图片描述内容填充为完整问题
- 如果用户没有输入文字，根据图片描述推断最可能想问的问题
- 最终输出只包含整理后的问题文本，不要加任何前缀、解释或markdown格式`, imageDescription, userText)

	messages := []nexus.MultimodalMessage{
		nexus.NewTextMessage("user", cleanPrompt),
	}
	return a.Nexus.MultimodalChatCompletion(ctx, messages, override, coreURL)
}

// cleanMediaQuestion is the generic version of cleanImageQuestion handling both image and video.
func (a *App) cleanMediaQuestion(ctx context.Context, kind nexus.MediaKind, mediaDescription, userText string, override *nexus.LLMOverride, coreURL string) (string, error) {
	mediaWord := "图片"
	if kind == nexus.MediaKindVideo {
		mediaWord = "视频"
	}
	cleanPrompt := fmt.Sprintf(`你是一个问题整理助手。用户上传了一个%s，VLM识别描述如下：

%s

用户还输入了以下文字（可能为空）：
%s

请结合%s描述和用户文字，推断用户真正想询问的问题，整理成一句完整、清晰的问题文本。

注意：
- 如果用户文字已经是一个完整的问题，保留并可能用%s信息补充上下文使其更精确
- 如果用户文字很简短（如"这个"、"解释一下"、"为什么"），用%s描述内容填充为完整问题
- 如果用户没有输入文字，根据%s描述推断最可能想问的问题
- 最终输出只包含整理后的问题文本，不要加任何前缀、解释或markdown格式`,
		mediaWord, mediaDescription, userText, mediaWord, mediaWord, mediaWord, mediaWord)

	messages := []nexus.MultimodalMessage{
		nexus.NewTextMessage("user", cleanPrompt),
	}
	return a.Nexus.MultimodalChatCompletion(ctx, messages, override, coreURL)
}
