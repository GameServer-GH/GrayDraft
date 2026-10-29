package model

// LLMOverride uses caller-supplied credentials (e.g. GrayDraft workspace settings).
// When nil, Core uses model_gateway from config.
type LLMOverride struct {
	Provider string `json:"provider"` // deepseek | openai | zhipu | google
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}

func (o *LLMOverride) Valid() bool {
	return o != nil && o.Provider != "" && o.Model != "" && o.APIKey != ""
}
