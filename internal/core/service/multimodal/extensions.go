package multimodal

import (
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
)

// MediaRefURL builds API path for visual RAG response (4.0 §5.2).
func MediaRefURL(md5 string) string {
	return fmt.Sprintf("/api/v1/multimodal/media/%s", md5)
}

// ApplyToExtensions merges multimodal assets into document extensions.
func ApplyToExtensions(ext model.Extensions, workspaceID string, results []ExtractionResult) model.Extensions {
	if ext == nil {
		ext = model.Extensions{}
	}
	var mm DocumentExtensions
	if raw, ok := ext["multimodal"]; ok {
		if m, ok := raw.(map[string]any); ok {
			mm = documentExtensionsFromMap(m)
		}
	}
	if workspaceID != "" {
		mm.WorkspaceID = workspaceID
	}
	var kvParts []string
	if strings.TrimSpace(mm.StructuredKVText) != "" {
		kvParts = append(kvParts, mm.StructuredKVText)
	}
	for _, r := range results {
		mm.Assets = append(mm.Assets, AssetMeta{
			Filename:     r.Filename,
			MimeType:     r.MimeType,
			MD5:          r.MD5,
			PHash:        r.PHash,
			MediaURL:     r.MediaURL,
			Route:        r.Route,
			Timestamps:   r.Timestamps,
			StructuredKV: r.StructuredKV,
		})
		if strings.TrimSpace(r.StructuredKV) != "" {
			kvParts = append(kvParts, r.StructuredKV)
		}
	}
	mm.StructuredKVText = strings.TrimSpace(strings.Join(kvParts, "\n"))
	ext["multimodal"] = mm
	return ext
}

func documentExtensionsFromMap(m map[string]any) DocumentExtensions {
	var mm DocumentExtensions
	if s, ok := m["workspace_id"].(string); ok {
		mm.WorkspaceID = s
	}
	if s, ok := m["structured_kv_text"].(string); ok {
		mm.StructuredKVText = s
	}
	if arr, ok := m["assets"].([]any); ok {
		for _, item := range arr {
			row, ok := item.(map[string]any)
			if !ok {
				continue
			}
			a := AssetMeta{}
			if v, ok := row["filename"].(string); ok {
				a.Filename = v
			}
			if v, ok := row["mime_type"].(string); ok {
				a.MimeType = v
			}
			if v, ok := row["md5"].(string); ok {
				a.MD5 = v
			}
			if v, ok := row["phash"].(string); ok {
				a.PHash = v
			}
			if v, ok := row["media_url"].(string); ok {
				a.MediaURL = v
			}
			if v, ok := row["route"].(string); ok {
				a.Route = v
			}
			if v, ok := row["structured_kv"].(string); ok {
				a.StructuredKV = v
			}
			if ts, ok := row["timestamps"].([]any); ok {
				for _, t := range ts {
					if s, ok := t.(string); ok {
						a.Timestamps = append(a.Timestamps, s)
					}
				}
			}
			mm.Assets = append(mm.Assets, a)
		}
	}
	return mm
}
