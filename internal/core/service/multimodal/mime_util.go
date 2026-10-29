package multimodal

import (
	"encoding/base64"
	"fmt"
	"strings"
)

func mediaDataURL(mime string, raw []byte) string {
	m := strings.TrimSpace(mime)
	if m == "" {
		m = "application/octet-stream"
	}
	return fmt.Sprintf("data:%s;base64,%s", m, base64.StdEncoding.EncodeToString(raw))
}

func normalizeMime(mime, filename string, raw []byte) string {
	m := strings.ToLower(strings.TrimSpace(mime))
	if m != "" && m != "application/octet-stream" {
		return m
	}
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".png"):
		return "image/png"
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		return "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		return "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		return "image/webp"
	case strings.HasSuffix(lower, ".pdf"):
		return "application/pdf"
	case strings.HasSuffix(lower, ".mp4"):
		return "video/mp4"
	case strings.HasSuffix(lower, ".webm"):
		return "video/webm"
	case strings.HasSuffix(lower, ".txt"), strings.HasSuffix(lower, ".md"):
		return "text/plain"
	}
	if len(raw) > 4 && raw[0] == 0x89 && raw[1] == 0x50 {
		return "image/png"
	}
	return m
}

func isPlainText(mime string) bool {
	return strings.HasPrefix(mime, "text/") || mime == "application/json" || mime == "application/markdown"
}

func looksLikeText(raw []byte) bool {
	if len(raw) == 0 || len(raw) > 2<<20 {
		return false
	}
	sample := raw
	if len(sample) > 8192 {
		sample = sample[:8192]
	}
	for _, b := range sample {
		if b == 0 {
			return false
		}
	}
	return true
}
