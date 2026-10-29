package nexus

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// MultimodalContentPart is one part of a multimodal message content array.
// Use Type="text" with Text set, Type="image_url" with ImageURL set,
// or Type="video_url" with VideoURL set.
type MultimodalContentPart struct {
	Type     string              `json:"type"`
	Text     string              `json:"text,omitempty"`
	ImageURL *MultimodalImgRef   `json:"image_url,omitempty"`
	VideoURL *MultimodalVideoRef `json:"video_url,omitempty"`
}

// MultimodalImgRef is the image_url object in an OpenAI-compatible content part.
type MultimodalImgRef struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"` // "auto", "low", "high"
}

// MultimodalVideoRef is the video_url object for video-capable multimodal models.
type MultimodalVideoRef struct {
	URL string `json:"url"`
}

// MultimodalMessage is a message whose Content may be a plain string or an
// array of MultimodalContentPart for multimodal (image/video + text) prompts.
type MultimodalMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []MultimodalContentPart
}

// MediaKind indicates whether a media URL points to an image or a video.
type MediaKind string

const (
	MediaKindImage MediaKind = "image"
	MediaKindVideo MediaKind = "video"
)

// NewTextMessage creates a simple text MultimodalMessage.
func NewTextMessage(role, text string) MultimodalMessage {
	return MultimodalMessage{Role: role, Content: text}
}

// NewVisionMessage creates a multimodal message with one or more image_url
// parts followed by an optional text part.
func NewVisionMessage(role string, imageDataURLs []string, text string) MultimodalMessage {
	parts := make([]MultimodalContentPart, 0, len(imageDataURLs)+1)
	for _, u := range imageDataURLs {
		parts = append(parts, MultimodalContentPart{
			Type:     "image_url",
			ImageURL: &MultimodalImgRef{URL: u, Detail: "auto"},
		})
	}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, MultimodalContentPart{Type: "text", Text: text})
	}
	return MultimodalMessage{Role: role, Content: parts}
}

// NewVideoMessage creates a multimodal message with a video_url part
// followed by an optional text part.
func NewVideoMessage(role string, videoURL string, text string) MultimodalMessage {
	parts := []MultimodalContentPart{
		{Type: "video_url", VideoURL: &MultimodalVideoRef{URL: videoURL}},
	}
	if strings.TrimSpace(text) != "" {
		parts = append(parts, MultimodalContentPart{Type: "text", Text: text})
	}
	return MultimodalMessage{Role: role, Content: parts}
}

// NewMediaMessage creates a multimodal message with either image_url or
// video_url depending on the media kind.
func NewMediaMessage(role string, kind MediaKind, mediaURL string, text string) MultimodalMessage {
	if kind == MediaKindVideo {
		return NewVideoMessage(role, mediaURL, text)
	}
	return NewVisionMessage(role, []string{mediaURL}, text)
}

// MultimodalChatRequest is the body sent to an OpenAI-compatible /v1/chat/completions.
type MultimodalChatRequest struct {
	Model    string              `json:"model"`
	Messages []MultimodalMessage `json:"messages"`
	Stream   bool                `json:"stream"`
}

// MultimodalChatCompletion sends messages to an OpenAI-compatible endpoint,
// collects the full stream, and returns the concatenated assistant text.
//
//   - override: workspace vendor credentials (nil = gateway at coreURL fallback)
//   - coreURL:  Core base URL for fallback model gateway
func (c *Client) MultimodalChatCompletion(
	ctx context.Context,
	messages []MultimodalMessage,
	override *LLMOverride,
	coreURL string,
) (string, error) {
	var streamErr error
	var fullText strings.Builder
	onDelta := func(delta string) error {
		fullText.WriteString(delta)
		return nil
	}

	if override != nil && override.Valid() {
		streamErr = streamMultimodalWithOverride(ctx, override, messages, onDelta)
	} else {
		gatewayURL := c.cfg.ModelGatewayURL
		if gatewayURL == "" {
			streamErr = fmt.Errorf("multimodal gateway not configured: set GRAYDRAFT_MODEL_GATEWAY_URL to the model gateway address, or configure workspace/system LLM credentials (GRAYDRAFT_SYSTEM_LLM_PROVIDER/_MODEL/_API_KEY)")
		} else {
			streamErr = streamMultimodalWithGateway(ctx, gatewayURL, messages, onDelta)
		}
	}

	if streamErr != nil {
		return fullText.String(), streamErr
	}
	return fullText.String(), nil
}

// Valid reports whether the override has all required fields.
func (o *LLMOverride) Valid() bool {
	return o != nil && strings.TrimSpace(o.Provider) != "" &&
		strings.TrimSpace(o.Model) != "" &&
		strings.TrimSpace(o.APIKey) != ""
}

// chatCompletionsEndpoint returns the full /chat/completions URL for a provider.
func chatCompletionsEndpoint(baseURL, provider string) string {
	base := strings.TrimRight(baseURL, "/")
	provider = strings.ToLower(strings.TrimSpace(provider))
	if provider == "intern" || provider == "volcengine" || provider == "ark" || provider == "volcark" || provider == "bytedance" {
		return base + "/chat/completions"
	}
	return base + "/v1/chat/completions"
}

// providerBaseURL maps provider id to OpenAI-compatible base URL.
func providerBaseURL(provider string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "deepseek":
		return "https://api.deepseek.com", nil
	case "openai":
		return "https://api.openai.com", nil
	case "zhipu":
		return "https://open.bigmodel.cn/api/paas/v4", nil
	case "google":
		return "https://generativelanguage.googleapis.com/v1beta/openai", nil
	case "intern":
		return "https://chat.intern-ai.org.cn/api/v1", nil
	case "volcengine", "ark", "volcark", "bytedance":
		return "https://ark.cn-beijing.volces.com/api/v3", nil
	default:
		return "", fmt.Errorf("unsupported llm provider: %s (use intern, deepseek, openai, zhipu, google, or volcengine)", provider)
	}
}

func streamMultimodalWithOverride(
	ctx context.Context,
	override *LLMOverride,
	messages []MultimodalMessage,
	onDelta func(string) error,
) error {
	base, err := providerBaseURL(override.Provider)
	if err != nil {
		return err
	}
	body, err := json.Marshal(MultimodalChatRequest{
		Model:    override.Model,
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	endpoint := chatCompletionsEndpoint(base, override.Provider)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Authorization", "Bearer "+override.APIKey)

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("multimodal llm %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return parseMultimodalSSE(resp.Body, onDelta)
}

func streamMultimodalWithGateway(
	ctx context.Context,
	coreURL string,
	messages []MultimodalMessage,
	onDelta func(string) error,
) error {
	if coreURL == "" {
		return fmt.Errorf("no gateway URL configured for multimodal chat")
	}
	body, err := json.Marshal(MultimodalChatRequest{
		Model:    "auto",
		Messages: messages,
		Stream:   true,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		strings.TrimRight(coreURL, "/")+"/v1/chat/completions",
		bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/event-stream")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		body := strings.TrimSpace(string(b))
		if resp.StatusCode == http.StatusNotFound {
			return fmt.Errorf("multimodal gateway endpoint not found at %s/v1/chat/completions (is the gateway service running?)", strings.TrimRight(coreURL, "/"))
		}
		return fmt.Errorf("multimodal gateway %d: %s", resp.StatusCode, body)
	}
	return parseMultimodalSSE(resp.Body, onDelta)
}

func parseMultimodalSSE(r io.Reader, onDelta func(string) error) error {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		payload := strings.TrimPrefix(line, "data: ")
		if payload == "[DONE]" {
			return nil
		}
		var chunk struct {
			Choices []struct {
				Delta struct {
					Content string `json:"content"`
				} `json:"delta"`
			} `json:"choices"`
		}
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			continue
		}
		if len(chunk.Choices) == 0 {
			continue
		}
		if d := chunk.Choices[0].Delta.Content; d != "" {
			if err := onDelta(d); err != nil {
				return err
			}
		}
	}
	return sc.Err()
}
