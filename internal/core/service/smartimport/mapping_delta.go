package smartimport

import (
	"github.com/graydraft/core/model"
)

func attachMappingDelta(ext model.Extensions, delta model.MappingDelta) model.Extensions {
	if ext == nil {
		ext = model.Extensions{}
	}
	if len(delta.KeyPhrases) == 0 {
		return ext
	}
	ext["mapping_delta"] = map[string]any{
		"scene":       delta.Scene,
		"style":       delta.Style,
		"key_phrases": delta.KeyPhrases,
	}
	return ext
}

func mappingDeltaFromExtensions(ext model.Extensions) model.MappingDelta {
	if ext == nil {
		return model.MappingDelta{}
	}
	raw, ok := ext["mapping_delta"].(map[string]any)
	if !ok {
		return model.MappingDelta{}
	}
	var delta model.MappingDelta
	if s, ok := raw["scene"].(string); ok {
		delta.Scene = s
	}
	if s, ok := raw["style"].(string); ok {
		delta.Style = s
	}
	if arr, ok := raw["key_phrases"].([]any); ok {
		for _, v := range arr {
			if s, ok := v.(string); ok && s != "" {
				delta.KeyPhrases = append(delta.KeyPhrases, s)
			}
		}
	}
	return delta
}
