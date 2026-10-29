package workspacecontext

import (
	"encoding/json"
	"strings"

	"github.com/graydraft/core/model"
)

const entityContextBonus = 0.12

// FormatForMultimodal renders profile text for VLM/OCR prompt injection.
func FormatForMultimodal(p model.ContextProfile) string {
	if p.IsEmpty() {
		return ""
	}
	var b strings.Builder
	if d := strings.TrimSpace(p.Description); d != "" {
		b.WriteString(d)
		b.WriteByte('\n')
	}
	if len(p.Entities) > 0 {
		b.WriteString("核心实体：")
		b.WriteString(strings.Join(cleanList(p.Entities), "、"))
		b.WriteByte('\n')
	}
	for _, r := range cleanList(p.Rules) {
		b.WriteString("规则：")
		b.WriteString(r)
		b.WriteByte('\n')
	}
	for _, n := range cleanList(p.NegativeContext) {
		b.WriteString("排除：")
		b.WriteString(n)
		b.WriteByte('\n')
	}
	return strings.TrimSpace(b.String())
}

// FormatForWorkspaceContext is the flattened string used for query embedding prefix and BFF transport.
func FormatForWorkspaceContext(p model.ContextProfile) string {
	if p.IsEmpty() {
		return ""
	}
	return FormatForMultimodal(p)
}

// FormatEmbeddingPrefix returns the prefix prepended to chunk text before embedding.
func FormatEmbeddingPrefix(kbType string, p model.ContextProfile) string {
	if p.IsEmpty() {
		return ""
	}
	label := strings.TrimSpace(p.Description)
	if label == "" {
		label = strings.TrimSpace(kbType)
	}
	ents := strings.Join(cleanList(p.Entities), " ")
	if ents == "" {
		if label == "" {
			return ""
		}
		return "【" + label + "】| "
	}
	if label == "" {
		label = strings.TrimSpace(kbType)
	}
	return "【" + label + "】" + ents + " | "
}

// ApplyContextualPrefix prepends workspace context to chunk text for embedding.
func ApplyContextualPrefix(text, prefix string) string {
	text = strings.TrimSpace(text)
	if prefix == "" {
		return text
	}
	if text == "" {
		return strings.TrimSpace(prefix)
	}
	return prefix + text
}

// EntityContextBonus adds confidence when query aligns with profile entities (O-20 light).
func EntityContextBonus(query string, p model.ContextProfile, chunks []model.Chunk) float64 {
	if p.IsEmpty() || len(chunks) == 0 {
		return 0
	}
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return 0
	}
	for _, ent := range cleanList(p.Entities) {
		el := strings.ToLower(ent)
		if el == "" {
			continue
		}
		if !strings.Contains(q, el) && !containsRuneFold(q, ent) {
			continue
		}
		for _, c := range chunks {
			body := strings.ToLower(c.Content + " " + c.DocTitle)
			if strings.Contains(body, el) || strings.Contains(body, q) {
				return entityContextBonus
			}
		}
	}
	return 0
}

// ProfileFromExtensions reads workspace_context_profile from UGC extensions.
func ProfileFromExtensions(ext map[string]any) (model.ContextProfile, bool) {
	if ext == nil {
		return model.ContextProfile{}, false
	}
	raw, ok := ext["workspace_context_profile"]
	if !ok || raw == nil {
		return model.ContextProfile{}, false
	}
	switch v := raw.(type) {
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return model.ContextProfile{}, false
		}
		var p model.ContextProfile
		if json.Unmarshal(b, &p) != nil {
			return model.ContextProfile{}, false
		}
		return p, !p.IsEmpty()
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return model.ContextProfile{}, false
		}
		var p model.ContextProfile
		if json.Unmarshal(b, &p) != nil {
			return model.ContextProfile{}, false
		}
		return p, !p.IsEmpty()
	}
}

func containsRuneFold(hay, needle string) bool {
	return strings.Contains(strings.ToLower(hay), strings.ToLower(needle))
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	seen := map[string]struct{}{}
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		key := strings.ToLower(s)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s)
	}
	return out
}

// MergeProfile prefers request profile; falls back to stored when request is empty.
func MergeProfile(req *model.ContextProfile, stored model.ContextProfile, kbType string) model.ContextProfile {
	if req != nil && !req.IsEmpty() {
		out := *req
		if out.KBType == "" {
			out.KBType = kbType
		}
		return out
	}
	if !stored.IsEmpty() {
		if stored.KBType == "" {
			stored.KBType = kbType
		}
		return stored
	}
	return model.ContextProfile{KBType: kbType}
}

// ResolveWorkspaceContextText returns embedding/chat context string (profile beats legacy string).
func ResolveWorkspaceContextText(profile model.ContextProfile, legacy string) string {
	if formatted := FormatForWorkspaceContext(profile); formatted != "" {
		return formatted
	}
	return strings.TrimSpace(legacy)
}
