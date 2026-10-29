package taxonomyevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

type birthLLMResponse struct {
	Dimension string `json:"dimension"`
	Label     string `json:"label"`
	Keywords  []string `json:"keywords"`
	Reason    string `json:"reason,omitempty"`
}

// tryLabelBirth promotes orphan candidates into a new canonical label (Loop B extension).
func (s *Service) tryLabelBirth(ctx context.Context, kbType string, st *model.WorkspaceTaxonomyState, tax *model.DomainTaxonomy) bool {
	if s == nil || st == nil || tax == nil || s.llm == nil {
		return false
	}
	threshold := 8
	if s.cfg != nil && s.cfg.TaxonomyEvolve.LabelBirthDocCount > 0 {
		threshold = s.cfg.TaxonomyEvolve.LabelBirthDocCount
	}
	var orphans []model.KeywordCandidate
	for _, c := range st.Candidates {
		if c.DocCount < threshold {
			continue
		}
		if strings.TrimSpace(c.Label) != "" {
			if _, r := BestLabelSimilarity(c.Phrase, labelDefsForDim(tax, c.Dimension)); r >= 0.85 {
				continue
			}
		}
		orphans = append(orphans, c)
	}
	if len(orphans) < 2 {
		return false
	}
	born, err := birthLabelLLM(ctx, s.llm, tax, orphans)
	if err != nil || born.Label == "" {
		return false
	}
	defs := &st.Scenes
	if born.Dimension == "style" {
		defs = &st.Styles
	}
	for _, d := range *defs {
		if d.Label == born.Label {
			return false
		}
	}
	*defs = append(*defs, model.LabelDefinition{Label: born.Label, Keywords: born.Keywords})
	st.Version++
	// Remove consumed orphans
	remove := map[string]bool{}
	for _, o := range orphans {
		remove[o.Phrase] = true
	}
	var kept []model.KeywordCandidate
	for _, c := range st.Candidates {
		if !remove[c.Phrase] {
			kept = append(kept, c)
		}
	}
	st.Candidates = kept
	slog.Info("taxonomy label birth", "kb_type", kbType, "dimension", born.Dimension, "label", born.Label)
	return true
}

func birthLabelLLM(ctx context.Context, llm *llmclient.Client, tax *model.DomainTaxonomy, orphans []model.KeywordCandidate) (birthLLMResponse, error) {
	var zero birthLLMResponse
	var lines []string
	for _, o := range orphans {
		lines = append(lines, fmt.Sprintf("- %s (%s, docs=%d)", o.Phrase, o.Dimension, o.DocCount))
	}
	sceneLabels := tax.SceneLabels()
	styleLabels := tax.StyleLabels()
	prompt := fmt.Sprintf(`你是 4D 标签裂变助手。以下高频词无法归入现有标签，请创建一个简洁的新标准 label。

domain=%s
现有 scene：%s
现有 style：%s

孤儿候选词：
%s

规则：新 label 2-8 字；选择 dimension=scene 或 style；keywords 包含候选词及其同义表达。

只输出 JSON：{"dimension":"scene|style","label":"","keywords":[],"reason":""}`,
		tax.Domain, strings.Join(sceneLabels, " | "), strings.Join(styleLabels, " | "), strings.Join(lines, "\n"))

	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return zero, err
	}
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	var parsed birthLLMResponse
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return zero, err
	}
	parsed.Dimension = strings.ToLower(strings.TrimSpace(parsed.Dimension))
	if parsed.Dimension != "style" {
		parsed.Dimension = "scene"
	}
	return parsed, nil
}
