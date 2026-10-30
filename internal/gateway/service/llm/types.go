package llm

// Message is a chat message (OpenAI-compatible).
type Message struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string or []MultimodalContentPart for vision
}

// ChatRequest is an OpenAI-style chat completion request.
type ChatRequest struct {
	Model       string    `json:"model"`
	Messages    []Message `json:"messages"`
	Stream      bool      `json:"stream"`
	MaxTokens       int     `json:"max_tokens,omitempty"`
	Temperature     float64 `json:"temperature,omitempty"`
	DisableThinking bool    `json:"disable_thinking,omitempty"`
}

// StreamDelta is emitted per token/chunk.
type StreamDelta struct {
	Content   string
	Reasoning string
	Done      bool
}
