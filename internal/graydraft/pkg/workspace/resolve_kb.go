package workspace

import (
	"encoding/json"
)

// ReadNexusKbType loads nexusKbType from settings.json in a workspace root.
func ReadNexusKbType(root string) (string, error) {
	raw, err := readSettingsBytes(SettingsPath(root))
	if err != nil {
		return "", err
	}
	return NexusKbSettings(json.RawMessage(raw)), nil
}
