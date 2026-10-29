package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/graydraft/graydraft/pkg/workspace"
)

const (
	maxAudioBytes = 25 * 1024 * 1024 // 25 MB
	maxAudioSecs  = 60               // max recording duration
)

type speechTranscribeRequest struct {
	AudioBase64 string `json:"audio_base64"`
	Format      string `json:"format"`
	Language    string `json:"language"`
}

type speechTranscribeResponse struct {
	Text       string `json:"text"`
	Language   string `json:"language,omitempty"`
	DurationMs int    `json:"duration_ms,omitempty"`
}

func (a *App) handleSpeechTranscribe(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	raw, err := io.ReadAll(io.LimitReader(r.Body, maxAudioBytes+64*1024))
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "read body: " + err.Error()})
		return
	}

	var in speechTranscribeRequest
	if err := json.Unmarshal(raw, &in); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json body"})
		return
	}

	b64 := strings.TrimSpace(in.AudioBase64)
	if b64 == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "audio_base64 required"})
		return
	}

	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid base64 audio"})
		return
	}
	defer func() {
		for i := range decoded {
			decoded[i] = 0
		}
	}()

	if len(decoded) > maxAudioBytes {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": fmt.Sprintf("audio too large (%d bytes, max %d)", len(decoded), maxAudioBytes)})
		return
	}

	format := strings.TrimSpace(in.Format)
	if format == "" {
		format = "webm"
	}

	lang := strings.TrimSpace(in.Language)
	if lang == "" || lang == "auto" {
		lang = "zh"
	}

	text, err := a.transcribeViaOpenAI(r, decoded, format, lang)
	if err != nil {
		a.Log.Error("speech transcribe failed", "err", err)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": "speech transcribe failed: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, speechTranscribeResponse{
		Text:     strings.TrimSpace(text),
		Language: lang,
	})
}

// transcribeViaOpenAI calls OpenAI Whisper API to transcribe audio.
func (a *App) transcribeViaOpenAI(r *http.Request, audioData []byte, format, language string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OPENAI_API_KEY"))
	baseURL := strings.TrimSpace(os.Getenv("OPENAI_BASE_URL"))
	if apiKey == "" {
		// Try workspace settings.
		wsRoot := a.workspaceRootFromHeaderOrNil(r)
		if wsRoot != "" {
			if key, _ := resolveFirstAPIKey(wsRoot); key != "" {
				apiKey = key
			}
		}
	}
	if apiKey == "" {
		return "", fmt.Errorf("no API key configured (set OPENAI_API_KEY or configure workspace vendor)")
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com"
	}
	baseURL = strings.TrimRight(baseURL, "/")

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Map format to file extension.
	ext := format
	switch format {
	case "webm":
		ext = "webm"
	case "wav":
		ext = "wav"
	case "mp4":
		ext = "mp4"
	default:
		ext = "webm"
	}

	part, err := writer.CreateFormFile("file", fmt.Sprintf("audio.%s", ext))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audioData); err != nil {
		return "", err
	}
	_ = writer.WriteField("model", "whisper-1")
	_ = writer.WriteField("language", language)
	_ = writer.WriteField("response_format", "json")
	writer.Close()

	ctx := r.Context()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/v1/audio/transcriptions", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 30 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return "", fmt.Errorf("whisper API %d: %s", res.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &out); err != nil {
		return "", fmt.Errorf("parse whisper response: %w", err)
	}
	return out.Text, nil
}

// resolveFirstAPIKey returns the first enabled vendor API key from workspace settings.
func resolveFirstAPIKey(wsRoot string) (string, error) {
	raw, err := os.ReadFile(workspace.SettingsPath(wsRoot))
	if err != nil {
		return "", err
	}
	var doc struct {
		Vendors []struct {
			Enabled bool   `json:"enabled"`
			APIKey  string `json:"apiKey"`
		} `json:"vendors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", err
	}
	for _, v := range doc.Vendors {
		if v.Enabled && strings.TrimSpace(v.APIKey) != "" {
			return strings.TrimSpace(v.APIKey), nil
		}
	}
	return "", nil
}
