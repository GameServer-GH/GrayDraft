package nexus

// LLMOverride uses workspace credentials instead of Nexus model_gateway.
type LLMOverride struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	APIKey   string `json:"api_key"`
}
