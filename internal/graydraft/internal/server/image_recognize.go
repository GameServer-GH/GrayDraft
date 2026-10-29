package server

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/graydraft/graydraft/pkg/nexus"
	"github.com/graydraft/graydraft/pkg/workspace"
)

// maxImageBytes is the maximum base64-decoded image size (10 MB).
const maxImageBytes = 10 * 1024 * 1024

// allowedImageMimes lists accepted image MIME types.
var allowedImageMimes = map[string]bool{
	"image/png":  true,
	"image/jpeg": true,
	"image/webp": true,
	"image/gif":  true,
}

type imageRecognizeRequest struct {
	ImageBase64 string `json:"image_base64"`
	MimeType    string `json:"mime_type"`
	UserText    string `json:"user_text"`
	// Video support
	VideoBase64 string `json:"video_base64"`
	VideoMimeType string `json:"video_mime_type"`
	// URL support (image or video)
	MediaURL string `json:"media_url"`
}

type imageRecognizeResponse struct {
	CleanedQuestion  string `json:"cleaned_question"`
	ImageDescription string `json:"image_description"`
}

func (a *App) handleImageRecognize(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if a.Nexus == nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "GRAYDRAFT_CORE_URL not configured"})
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxVideoBytes+64*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}

	var in imageRecognizeRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	ctx := r.Context()
	userText := strings.TrimSpace(in.UserText)

	llmOverride := a.resolveLLMForImage(r)
	coreURL := ""
	if a.Nexus != nil {
		coreURL = a.Nexus.CoreURL()
	}

	mediaURL := strings.TrimSpace(in.MediaURL)
	videoBase64 := strings.TrimSpace(in.VideoBase64)
	imageBase64 := strings.TrimSpace(in.ImageBase64)

	var kind nexus.MediaKind
	var dataURL string

	switch {
	case mediaURL != "":
		kind = detectMediaKindFromURL(mediaURL)
		if kind == nexus.MediaKindVideo && !allowedVideoURL(mediaURL) {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported video URL; must end with .mp4, .webm, or .mov"})
			return
		}
		dataURL = mediaURL

	case videoBase64 != "":
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
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 video"})
			return
		}
		if len(decoded) > maxVideoBytes {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("video too large (%d bytes, max %d)", len(decoded), maxVideoBytes)})
			return
		}
		defer zeroBytes(decoded)
		dataURL = fmt.Sprintf("data:%s;base64,%s", videoMime, base64.StdEncoding.EncodeToString(decoded))
		kind = nexus.MediaKindVideo

	default:
		// Original image path
		mime := strings.TrimSpace(in.MimeType)
		if mime == "" {
			mime = "image/png"
		}
		if !allowedImageMimes[mime] {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported image type: " + mime})
			return
		}

		if imageBase64 == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "image_base64 required"})
			return
		}
		decoded, err := decodeBase64(imageBase64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 image"})
			return
		}
		if len(decoded) > maxImageBytes {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("image too large (%d bytes, max %d)", len(decoded), maxImageBytes)})
			return
		}
		defer zeroBytes(decoded)
		dataURL = fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(decoded))
		kind = nexus.MediaKindImage
	}

	// ── Describe the media via VLM ──
	var descPrompt string
	if kind == nexus.MediaKindVideo {
		descPrompt = "请仔细观看这个视频，描述视频中的场景、人物、动作、对话、文字等所有关键信息。如果是教学视频，提取核心知识；如果是演示视频，描述操作流程；请用中文详细描述。"
	} else {
		descPrompt = "请仔细描述这张图片的内容，包括其中的文字、数据、图表、结构等所有关键信息。如果是表格，列出表头和关键数据；如果是截图，描述界面布局和关键信息；如果是文档，提取核心内容。请用中文描述。"
	}

	mediaDescription, descErr := a.chatWithMedia(ctx, kind, dataURL, descPrompt, llmOverride, coreURL)
	if descErr != nil {
		a.Log.Error("image recognize: describe failed", "err", descErr, "kind", kind)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description failed: " + descErr.Error()})
		return
	}
	mediaDescription = strings.TrimSpace(mediaDescription)
	if mediaDescription == "" {
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "media description returned empty"})
		return
	}

	// ── Clean / extract question from description + user text ──
	cleanedQuestion, cleanErr := a.cleanMediaQuestion(ctx, kind, mediaDescription, userText, llmOverride, coreURL)
	if cleanErr != nil {
		a.Log.Error("image recognize: clean failed", "err", cleanErr)
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

	writeJSON(w, http.StatusOK, imageRecognizeResponse{
		CleanedQuestion:  cleanedQuestion,
		ImageDescription: mediaDescription,
	})
}

// workspaceRootFromHeaderOrNil returns the workspace root, or empty string if unavailable.
func (a *App) workspaceRootFromHeaderOrNil(r *http.Request) string {
	wsRoot, err := a.workspaceRootFromHeader(r)
	if err != nil {
		return ""
	}
	return wsRoot
}

// resolveLLMForImage resolves the LLM override for image recognition.
// Priority: 1) workspace vendor, 2) system-level multimodal credentials (VLM-capable),
// 3) model gateway (only if it supports vision models).
func (a *App) resolveLLMForImage(r *http.Request) *nexus.LLMOverride {
	wsRoot := a.workspaceRootFromHeaderOrNil(r)
	if wsRoot != "" {
		if override, _ := workspace.ResolveLLMOverride(wsRoot, "auto"); override != nil {
			return override
		}
	}
	if override := nexus.SystemLLMOverride(); override != nil {
		return override
	}
	return nil
}
