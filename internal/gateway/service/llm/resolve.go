package llm

import (
	"strings"

	"github.com/graydraft/gateway/config"
)

const (
	modelAliasDefault = "default"
	modelAliasFast    = "fast"
	modelAliasComplex = "complex"
	modelAliasPro     = "pro"
)

// resolveBinding picks provider binding and API model id from the request model field.
// Empty/default/fast → llm_fast; complex/pro or explicit complex model name → llm (pro).
// The bool is true when the complex tier was selected (enables flash fallback on API error).
func resolveBinding(cfg *config.Config, requested string) (config.ModelBinding, string, bool) {
	fast := cfg.Models.LLMFast
	complex := cfg.Models.LLM
	req := strings.TrimSpace(strings.ToLower(requested))

	switch req {
	case "", modelAliasDefault, modelAliasFast, "auto":
		return fast, pickModel(requested, fast.Model), false
	case modelAliasComplex, modelAliasPro:
		return complex, pickModel(requested, complex.Model), true
	}

	if requested != "" {
		if strings.EqualFold(requested, fast.Model) {
			return fast, fast.Model, false
		}
		if strings.EqualFold(requested, complex.Model) {
			return complex, complex.Model, true
		}
		// Explicit unknown model id: use complex provider with caller's model name.
		return complex, requested, true
	}
	return fast, fast.Model, false
}

func pickModel(requested, defaultModel string) string {
	r := strings.TrimSpace(requested)
	if r == "" || strings.EqualFold(r, modelAliasDefault) || strings.EqualFold(r, modelAliasFast) ||
		strings.EqualFold(r, modelAliasComplex) || strings.EqualFold(r, modelAliasPro) {
		return defaultModel
	}
	return requested
}
