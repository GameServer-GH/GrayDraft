package workspace

import (
	"encoding/json"
	"os"
)

const (
	settingsKeyMultimodalWorkspacePrompt = "multimodalWorkspacePrompt"
)

// MultimodalPromptSettings is stored in settings.json (4.0 workspace override).
type MultimodalPromptSettings struct {
	WorkspacePrompt string `json:"workspacePrompt"`
}

// ReadMultimodalWorkspacePrompt loads workspace VLM override from settings.json.
func ReadMultimodalWorkspacePrompt(wsRoot string) (string, error) {
	return ReadAgentUnstructuredPrompt(wsRoot)
}

// MergeMultimodalWorkspacePrompt updates settings object with workspace prompt.
func MergeMultimodalWorkspacePrompt(settings json.RawMessage, prompt string) json.RawMessage {
	var obj map[string]any
	if len(settings) > 0 {
		_ = json.Unmarshal(settings, &obj)
	}
	if obj == nil {
		obj = map[string]any{}
	}
	obj[settingsKeyMultimodalWorkspacePrompt] = prompt
	out, err := json.Marshal(obj)
	if err != nil {
		return settings
	}
	return out
}

// WriteMultimodalWorkspacePrompt persists only the prompt field into settings.json.
func WriteMultimodalWorkspacePrompt(wsRoot, prompt string) error {
	return WriteAgentUnstructuredPrompt(wsRoot, prompt)
}

// ReadSettingsFile returns raw settings.json bytes.
func ReadSettingsFile(wsRoot string) ([]byte, error) {
	return os.ReadFile(SettingsPath(wsRoot))
}
