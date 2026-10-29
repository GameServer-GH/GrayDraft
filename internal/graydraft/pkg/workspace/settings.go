package workspace

import (
	"bytes"
	"encoding/json"
	"os"
)

// ReadSettingsRaw loads settings.json bytes.
func ReadSettingsRaw(path string) (json.RawMessage, error) {
	b, err := readSettingsBytes(path)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(b), nil
}

// WriteSettings 将 JSON 写入 settings.json（覆盖）；raw 须为合法 JSON 对象。
func WriteSettings(path string, raw json.RawMessage) error {
	if len(bytes.TrimSpace(raw)) == 0 {
		raw = json.RawMessage(`{}`)
	}
	if !json.Valid(raw) {
		return os.ErrInvalid
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, raw, "", "  "); err != nil {
		return err
	}
	_ = buf.WriteByte('\n')
	return os.WriteFile(path, buf.Bytes(), 0o644)
}
