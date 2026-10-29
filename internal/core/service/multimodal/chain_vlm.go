package multimodal

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/promptmd"
)

const chainContextMaxRunes = 900

// chainContextParams carries prior-image context for multi-image sequential VLM.
type chainContextParams struct {
	Index        int // 1-based
	Total        int
	PriorContext string
}

func (p *Processor) chainVLMEnabled() bool {
	if p == nil || p.cfg == nil {
		return true
	}
	return p.cfg.Multimodal.ChainVLMEnabled
}

func shouldChainVLM(attachments []model.SmartImportAttachment) bool {
	visual := 0
	for _, att := range attachments {
		if attachmentLooksVisual(att) {
			visual++
		}
	}
	return visual >= 2
}

func attachmentLooksVisual(att model.SmartImportAttachment) bool {
	m := strings.ToLower(strings.TrimSpace(att.MimeType))
	if strings.HasPrefix(m, "image/") || strings.HasPrefix(m, "video/") {
		return true
	}
	ext := strings.ToLower(filepath.Ext(att.Filename))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".heic", ".heif",
		".mp4", ".mov", ".webm", ".mkv":
		return true
	default:
		return false
	}
}

func appendChainVLMPrompt(base string, chain *chainContextParams) string {
	if chain == nil || chain.Total < 2 || strings.TrimSpace(chain.PriorContext) == "" {
		return base
	}
	block := fmt.Sprintf(`

【多图链式理解 — 第 %d/%d 张】
前序图片已识别内容（仅供关联同一主题/角色/活动，勿整段照抄；本图须写本图新事实）：
%s

%s`,
		chain.Index, chain.Total, strings.TrimSpace(chain.PriorContext), promptmd.SceneChainVLM)
	return base + block
}

func buildChainContext(filename string, res ExtractionResult, index, total int) string {
	summary := strings.TrimSpace(res.ChainSummary)
	if summary == "" {
		summary = chainSummaryFromDisplay(res.Text)
	}
	if summary == "" {
		return ""
	}
	summary = truncateRunes(summary, chainContextMaxRunes)
	name := strings.TrimSpace(filename)
	if name == "" {
		name = fmt.Sprintf("图%d", index)
	}
	return fmt.Sprintf("（图 %d/%d · %s）\n%s", index, total, name, summary)
}

func chainSummaryFromDisplay(display string) string {
	display = strings.TrimSpace(display)
	if display == "" {
		return ""
	}
	// Prefer first paragraph as rolling summary.
	if i := strings.Index(display, "\n\n"); i > 0 {
		return strings.TrimSpace(display[:i])
	}
	return truncateRunes(display, 400)
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	return string([]rune(s)[:max]) + "…"
}

func extractChainSummary(rawModelOut string, parsed parsedExtraction) string {
	rawModelOut = stripCodeFences(strings.TrimSpace(rawModelOut))
	var obj map[string]any
	if json.Unmarshal([]byte(rawModelOut), &obj) == nil {
		if v, ok := obj["summary"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	} else if obj2, ok := tryParseJSONObject(rawModelOut); ok {
		if v, ok := obj2["summary"].(string); ok {
			if s := strings.TrimSpace(v); s != "" {
				return s
			}
		}
	}
	display := FormatExtractionText(mergeTextAndStructured(parsed.Text, parsed.StructuredKV))
	return chainSummaryFromDisplay(display)
}
