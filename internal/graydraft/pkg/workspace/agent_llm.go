package workspace

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/gocommon/sealed"
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
		provider := strings.ToLower(strings.TrimSpace(v.KindID))
		key := strings.TrimSpace(v.APIKey)
		if key == "" && provider == "intern" {
			key = sealed.Intern()
		}
		if key == "" {
			return nil, fmt.Errorf("vendor %s has no api key", vendorID)
		}
		if provider == "anthropic" {
			return nil, fmt.Errorf("anthropic is not supported for workspace LLM override; use intern, deepseek, openai, zhipu, or google")
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

// EnsureBuiltinInternVendor puts 书生 in the configured vendor list and fills the
// built-in sealed key when that vendor has none. An existing non-empty key is kept.
func EnsureBuiltinInternVendor(settings json.RawMessage) (json.RawMessage, bool, error) {
	m := map[string]any{}
	if len(settings) > 0 && json.Valid(settings) {
		if err := json.Unmarshal(settings, &m); err != nil {
			return settings, false, err
		}
	}
	key := strings.TrimSpace(sealed.Intern())
	vendors, _ := m["vendors"].([]any)
	changed := false
	found := false
	for i, item := range vendors {
		vm, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if strings.ToLower(strings.TrimSpace(stringField(vm["kindId"]))) != "intern" {
			continue
		}
		found = true
		if strings.TrimSpace(stringField(vm["apiKey"])) == "" && key != "" {
			vm["apiKey"] = key
			changed = true
		}
		models, _ := vm["models"].([]any)
		if len(models) == 0 {
			vm["models"] = defaultInternModels()
			changed = true
		}
		vendors[i] = vm
	}
	if !found {
		vendors = append([]any{map[string]any{
			"id":      "builtin-intern",
			"kindId":  "intern",
			"apiKey":  key,
			"enabled": true,
			"models":  defaultInternModels(),
		}}, vendors...)
		changed = true
	}
	if !changed {
		return settings, false, nil
	}
	m["vendors"] = vendors
	b, err := json.Marshal(m)
	if err != nil {
		return settings, false, err
	}
	return b, true, nil
}

func defaultInternModels() []any {
	return []any{
		map[string]any{"id": "builtin-intern-s2", "name": "intern-s2", "enabled": true},
		map[string]any{"id": "builtin-intern-s1-mini", "name": "intern-s1-mini", "enabled": true},
	}
}

func stringField(v any) string {
	s, _ := v.(string)
	return s
}
