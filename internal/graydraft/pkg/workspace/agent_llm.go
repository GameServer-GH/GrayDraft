package workspace

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/graydraft/pkg/nexus"
)

type settingsVendor struct {
	ID      string `json:"id"`
	KindID  string `json:"kindId"`
	APIKey  string `json:"apiKey"`
	Enabled bool   `json:"enabled"`
	Models  []struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Enabled bool   `json:"enabled"`
	} `json:"models"`
}

// ResolveLLMOverride maps agent_model "auto" → nil (Nexus gateway); "vendorId:modelId" → workspace key.
func ResolveLLMOverride(wsRoot, agentModel string) (*nexus.LLMOverride, error) {
	agentModel = strings.TrimSpace(agentModel)
	if agentModel == "" || agentModel == "auto" {
		return nil, nil
	}
	vendorID, modelID, ok := strings.Cut(agentModel, ":")
	if !ok || vendorID == "" || modelID == "" {
		return nil, fmt.Errorf("invalid agent_model: %q", agentModel)
	}

	raw, err := readSettingsBytes(SettingsPath(wsRoot))
	if err != nil {
		return nil, err
	}
	var doc struct {
		Vendors []settingsVendor `json:"vendors"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse settings: %w", err)
	}

	for _, v := range doc.Vendors {
		if v.ID != vendorID || !v.Enabled {
			continue
		}
		key := strings.TrimSpace(v.APIKey)
		if key == "" {
			return nil, fmt.Errorf("vendor %s has no api key", vendorID)
		}
		provider := strings.ToLower(strings.TrimSpace(v.KindID))
		if provider == "anthropic" {
			return nil, fmt.Errorf("anthropic is not supported for workspace LLM override; use deepseek, openai, zhipu, or google")
		}
		for _, m := range v.Models {
			if m.ID != modelID || !m.Enabled {
				continue
			}
			name := strings.TrimSpace(m.Name)
			if name == "" {
				return nil, fmt.Errorf("model %s has no name", modelID)
			}
			return &nexus.LLMOverride{
				Provider: provider,
				Model:    name,
				APIKey:   key,
			}, nil
		}
		return nil, fmt.Errorf("model not found or disabled: %s", modelID)
	}
	return nil, fmt.Errorf("vendor not found or disabled: %s", vendorID)
}
