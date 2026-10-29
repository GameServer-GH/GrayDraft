package agentlog

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSanitizeJSON_attachments(t *testing.T) {
	raw := json.RawMessage(`{"attachments":[{"filename":"a.jpg","data_base64":"` + strings.Repeat("x", 200) + `"}]}`)
	out := SanitizeJSON(raw)
	if strings.Contains(string(out), strings.Repeat("x", 50)) {
		t.Fatal("expected redacted attachment data_base64")
	}
	if !strings.Contains(string(out), "redacted") {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestSanitizeJSON_mediaBase64(t *testing.T) {
	raw := json.RawMessage(`{"media_image_base64":"` + strings.Repeat("y", 500) + `"}`)
	out := SanitizeJSON(raw)
	if strings.Contains(string(out), strings.Repeat("y", 50)) {
		t.Fatal("expected redacted media_image_base64")
	}
	if !strings.Contains(string(out), "redacted") {
		t.Fatalf("unexpected: %s", out)
	}
}

func TestSanitizeJSON_keepsDataURL(t *testing.T) {
	raw := json.RawMessage(`{"attachments":[{"filename":"img.webp","data_url":"https://example.com/img.webp"}]}`)
	out := SanitizeJSON(raw)
	if !strings.Contains(string(out), "https://example.com/img.webp") {
		t.Fatal("data_url should be preserved for debugging")
	}
}
