package llmclient

// Tier selects which LLM profile Core requests from Model Gateway.
type Tier int

const (
	// TierFast is for simple, low-accuracy tasks (smart import, classification).
	TierFast Tier = iota
	// TierComplex is for RAG chat and other high-accuracy generation.
	TierComplex
)
