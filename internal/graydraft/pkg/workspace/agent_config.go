package workspace

import "encoding/json"

const settingsKeyAgentConfig = "agentConfig"

// AgentConfigSettings is persisted per workspace (4.0).
type AgentConfigSettings struct {
	// UnstructuredPrompt is workspace domain instructions (natural language, stacked on system base).
	UnstructuredPrompt string `json:"unstructuredPrompt"`
	// ContextProfile is structured workspace background (description, entities, rules).
	ContextProfile ContextProfileSettings `json:"contextProfile,omitempty"`
}

// ReadAgentUnstructuredPrompt loads editable workspace agent prompt from settings.json.
func ReadAgentUnstructuredPrompt(wsRoot string) (string, error) {
	raw, err := readSettingsBytes(SettingsPath(wsRoot))
	if err != nil {
		return "", err
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		return "", err
	}
	if ac, ok := obj[settingsKeyAgentConfig].(map[string]any); ok {
		if s, ok := ac["unstructuredPrompt"].(string); ok && s != "" {
			return s, nil
		}
	}
	// legacy flat key
	if v, ok := obj[settingsKeyMultimodalWorkspacePrompt]; ok {
		if s, ok := v.(string); ok {
			return s, nil
		}
	}
	return "", nil
}

// MergeAgentConfig updates settings with agentConfig.unstructuredPrompt (keeps legacy key in sync).
func MergeAgentConfig(settings json.RawMessage, unstructuredPrompt string) json.RawMessage {
	var obj map[string]any
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &obj)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	obj[settingsKeyAgentConfig] = map[string]any{
		"unstructuredPrompt": unstructuredPrompt,
	}
	obj[settingsKeyMultimodalWorkspacePrompt] = unstructuredPrompt
	out, err := json.Marshal(obj)
	if err != nil {
		return settings
	}
	return out
}

// WriteAgentUnstructuredPrompt persists agent config prompt.
func WriteAgentUnstructuredPrompt(wsRoot, unstructuredPrompt string) error {
	path := SettingsPath(wsRoot)
	raw, err := readSettingsBytes(path)
	if err != nil {
		return err
	}
	merged := MergeAgentConfig(raw, unstructuredPrompt)
	return WriteSettings(path, merged)
}
