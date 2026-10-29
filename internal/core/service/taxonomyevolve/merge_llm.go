package taxonomyevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

type mergeLLMResponse struct {
	Action   string `json:"action"` // add | merge | ignore
	TargetKW string `json:"target_keyword,omitempty"`
	Reason   string `json:"reason,omitempty"`
}

// resolveAmbiguousKeyword asks Fast LLM when string similarity is inconclusive.
func resolveAmbiguousKeyword(ctx context.Context, llm *llmclient.Client, phrase, label, dimension string, existing []string) (action string, targetKW string, err error) {
	if llm == nil {
		return "add", "", nil
	}
	exist := strings.Join(existing, "、")
	if exist == "" {
		exist = "(无)"
	}
	prompt := fmt.Sprintf(`你是 4D 关键词映射合并助手。新词与现有词相似度处于模糊区间，请裁决。

维度：%s
标准标签：%s
新关键词：%s
现有关键词：%s

规则：
- merge：新词与某一现有关词语义相同，应合并到该词（不新增 mapping 条目）
- add：新词表达独特概念，应作为新关键词加入该标签（保持 mapping 精简）
- ignore：新词噪声或与标签无关

只输出一行 JSON：{"action":"add|merge|ignore","target_keyword":"","reason":""}`,
		dimension, label, phrase, exist)

	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return "add", "", err
	}
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	var parsed mergeLLMResponse
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return "add", "", err
	}
	action = strings.ToLower(strings.TrimSpace(parsed.Action))
	if action == "" {
		action = "add"
	}
	return action, strings.TrimSpace(parsed.TargetKW), nil
}

// compressMappingLLM merges candidate buffer into a slim keyword set (Loop B).
func compressMappingLLM(ctx context.Context, llm *llmclient.Client, tax *model.DomainTaxonomy, candidates []model.KeywordCandidate, maxPerLabel int) ([]model.LabelDefinition, []model.LabelDefinition, error) {
	if llm == nil || tax == nil {
		return nil, nil, fmt.Errorf("llm or taxonomy required")
	}
	var candLines []string
	for _, c := range candidates {
		candLines = append(candLines, fmt.Sprintf("%s/%s: %s (docs=%d)", c.Dimension, c.Label, c.Phrase, c.DocCount))
	}
	sceneKW := formatLabelKeywords(tax.Scenes)
	styleKW := formatLabelKeywords(tax.Styles)
	prompt := fmt.Sprintf(`你是 4D Mapping 压缩助手。将候选新词合并进现有体系，保持每个标签下关键词不超过 %d 个。

domain=%s
现有 scene 映射：
%s
现有 style 映射：
%s

候选新词（待合并）：
%s

要求：合并语义重复词；丢弃噪声；重要新词可替换低频旧词。

只输出 JSON：{"scenes":[{"label":"","keywords":[]}],"styles":[{"label":"","keywords":[]}]}`,
		maxPerLabel, tax.Domain, sceneKW, styleKW, strings.Join(candLines, "\n"))

	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return nil, nil, err
	}
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	var parsed struct {
		Scenes []model.LabelDefinition `json:"scenes"`
		Styles []model.LabelDefinition `json:"styles"`
	}
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return nil, nil, err
	}
	return parsed.Scenes, parsed.Styles, nil
}

func formatLabelKeywords(defs []model.LabelDefinition) string {
	var lines []string
	for _, d := range defs {
		if d.Label == "" {
			continue
		}
		lines = append(lines, d.Label+": "+strings.Join(d.Keywords, "、"))
	}
	if len(lines) == 0 {
		return "(无)"
	}
	return strings.Join(lines, "\n")
}
