package workspace

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveLLMOverride_auto(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(SettingsPath(dir), []byte(`{"vendors":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := ResolveLLMOverride(dir, "auto")
	if err != nil || o != nil {
		t.Fatalf("want nil override, got %v err=%v", o, err)
	}
}

func TestResolveLLMOverride_workspace(t *testing.T) {
	dir := t.TempDir()
	raw := `{
  "vendors": [{
    "id": "v1",
    "kindId": "deepseek",
    "apiKey": "sk-test",
    "enabled": true,
    "models": [{"id": "m1", "name": "deepseek-chat", "enabled": true}]
  }]
}`
	if err := os.WriteFile(SettingsPath(dir), []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	o, err := ResolveLLMOverride(dir, "v1:m1")
	if err != nil {
		t.Fatal(err)
	}
	if o == nil || o.Provider != "deepseek" || o.Model != "deepseek-chat" || o.APIKey != "sk-test" {
		t.Fatalf("unexpected override: %+v", o)
	}
	_, err = ResolveLLMOverride(filepath.Clean(dir), "v1:missing")
	if err == nil {
		t.Fatal("expected error for missing model")
	}
}
