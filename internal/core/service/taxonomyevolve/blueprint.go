package taxonomyevolve

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
)

type blueprintLLMResponse struct {
	Domain string                  `json:"domain"`
	Scenes []model.LabelDefinition `json:"scenes"`
	Styles []model.LabelDefinition `json:"styles"`
	Reason string                  `json:"reason,omitempty"`
}

func (s *Service) enqueueBlueprint(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) {
	if s == nil || !s.blueprintEnabled() || kbType == "" {
		return
	}
	s.blueprintMu.Lock()
	if s.blueprintInflight[kbType] {
		s.blueprintMu.Unlock()
		return
	}
	s.blueprintInflight[kbType] = true
	s.blueprintMu.Unlock()

	go func() {
		defer func() {
			s.blueprintMu.Lock()
			delete(s.blueprintInflight, kbType)
			s.blueprintMu.Unlock()
		}()
		bg, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()
		if err := s.RunBlueprint(bg, kbType, profile, workspaceText); err != nil {
			slog.Warn("taxonomy blueprint", "kb_type", kbType, "error", err)
		}
	}()
}

func (s *Service) blueprintValidatorEnabled() bool {
	if s == nil || s.cfg == nil || !s.blueprintEnabled() {
		return false
	}
	return s.cfg.TaxonomyEvolve.BlueprintValidatorEnabled
}

func (s *Service) blueprintEnabled() bool {
	if s == nil || s.cfg == nil {
		return false
	}
	if !s.cfg.TaxonomyEvolve.Enabled {
		return false
	}
	return s.cfg.TaxonomyEvolve.BlueprintEnabled
}

// MaybeEnqueueBlueprint triggers blueprint when workspace has profile/docs but no ready blueprint.
func (s *Service) MaybeEnqueueBlueprint(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) {
	if !s.blueprintEnabled() || kbType == "" {
		return
	}
	st, _ := s.repo.GetWorkspaceTaxonomy(ctx, kbType)
	if st != nil && (st.BlueprintReady() || st.BlueprintPending()) {
		return
	}
	hasProfile := profile != nil && !profile.IsEmpty()
	hasText := strings.TrimSpace(workspaceText) != ""
	if !hasProfile && !hasText && s.repo != nil {
		if docs, err := s.repo.ListRecentDocSummaries(ctx, kbType, 3); err == nil && len(docs) > 0 {
			hasText = true
		}
	}
	if !hasProfile && !hasText {
		return
	}
	s.enqueueBlueprint(ctx, kbType, profile, workspaceText)
}

// RunBlueprint generates workspace-specific domain/scene/style labels via LLM.
func (s *Service) RunBlueprint(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) error {
	if s == nil || s.repo == nil || s.llm == nil || kbType == "" {
		return nil
	}
	st, err := s.repo.GetWorkspaceTaxonomy(ctx, kbType)
	if err != nil {
		return err
	}
	if st != nil && st.BlueprintReady() {
		return nil
	}
	if st == nil {
		st = &model.WorkspaceTaxonomyState{
			KBType:          kbType,
			Version:         1,
			Candidates:      []model.KeywordCandidate{},
			KeywordHits:     map[string]int{},
			BlueprintStatus: model.BlueprintStatusPending,
		}
	} else {
		st.BlueprintStatus = model.BlueprintStatusPending
		st.BlueprintError = ""
	}
	if err := s.repo.SaveWorkspaceTaxonomy(ctx, st); err != nil {
		return err
	}
	s.resolver.Invalidate(kbType)

	limit := 5
	if s.cfg != nil && s.cfg.TaxonomyEvolve.BlueprintMinDocs > 0 {
		limit = s.cfg.TaxonomyEvolve.BlueprintMinDocs
	}
	summaries, _ := s.repo.ListRecentDocSummaries(ctx, kbType, limit)
	if profile == nil || profile.IsEmpty() {
		p, _ := s.repo.GetContextProfile(ctx, kbType)
		profile = &p
	}
	parsed, err := generateBlueprintLLM(ctx, s.llm, profile, workspaceText, summaries)
	if err != nil {
		st.BlueprintStatus = model.BlueprintStatusFailed
		st.BlueprintError = err.Error()
		_ = s.repo.SaveWorkspaceTaxonomy(ctx, st)
		s.resolver.Invalidate(kbType)
		return err
	}
	if s.blueprintValidatorEnabled() {
		if vErr := validateBlueprintLabels(parsed); vErr != nil {
			slog.Warn("blueprint validation failed, retrying", "kb_type", kbType, "error", vErr)
			retry, rErr := generateBlueprintLLM(ctx, s.llm, profile, workspaceText, summaries)
			if rErr == nil {
				if vErr2 := validateBlueprintLabels(retry); vErr2 == nil {
					parsed = retry
				} else {
					slog.Warn("blueprint validation still weak after retry", "kb_type", kbType, "error", vErr2)
					parsed = retry
				}
			}
		}
	}
	st.Domain = strings.TrimSpace(parsed.Domain)
	st.Scenes = trimBlueprintDefs(parsed.Scenes, 10)
	st.Styles = trimBlueprintDefs(parsed.Styles, 8)
	st.BlueprintStatus = model.BlueprintStatusReady
	st.BlueprintError = ""
	st.BlueprintAt = time.Now().UTC()
	st.Version++
	if st.Domain == "" {
		st.Domain = model.DefaultCatalog().ResolveKey(profile, workspaceText, "")
	}
	if err := s.repo.SaveWorkspaceTaxonomy(ctx, st); err != nil {
		return err
	}
	s.resolver.Invalidate(kbType)
	s.clearAlignCache()
	s.EnqueueShadowBackfill(ctx, kbType, st.Version)
	slog.Info("taxonomy blueprint ready", "kb_type", kbType, "domain", st.Domain, "scenes", len(st.Scenes), "styles", len(st.Styles))
	return nil
}

func generateBlueprintLLM(ctx context.Context, llm *llmclient.Client, profile *model.ContextProfile, workspaceText string, docs []model.DocSummarySnippet) (blueprintLLMResponse, error) {
	var zero blueprintLLMResponse
	if llm == nil {
		return zero, fmt.Errorf("llm required")
	}
	profText := ""
	if profile != nil {
		profText = strings.TrimSpace(profile.Description)
		if len(profile.Entities) > 0 {
			profText += "\n实体类型：" + strings.Join(profile.Entities, "、")
		}
	}
	if workspaceText != "" {
		profText += "\n" + workspaceText
	}
	var docLines []string
	for i, d := range docs {
		if i >= 5 {
			break
		}
		line := strings.TrimSpace(d.Title)
		if s := strings.TrimSpace(d.Summary); s != "" {
			if line != "" {
				line += " — "
			}
			line += s
		}
		if line != "" {
			docLines = append(docLines, fmt.Sprintf("%d. %s", i+1, line))
		}
	}
	globalStyles := model.GlobalStyleScaffold()
	var styleNames []string
	for _, s := range globalStyles {
		if s.Label != "" {
			styleNames = append(styleNames, s.Label)
		}
	}
	prompt := fmt.Sprintf(`你是知识库「4D 语义蓝图」设计助手。根据工作区背景与样例文档，设计该库专属的语义坐标系。

工作区背景：
%s

样例文档（最多 5 篇）：
%s

全局通用 style（可复用，不必重复发明）：%s

任务：
1. domain：用 2-8 个字概括本库业务领域（可创造 YAML 中不存在的名称，如「影视解说」「医疗指南」）
2. scenes：本库用户关心的 4-8 个场景标签（中文短 label，如「剧情复盘」「参数建议」）
3. styles：本库常见的 3-6 个体裁标签；优先从全局 style 里选，不足再新增

约束：
- label 必须简短（≤8 字），禁止空 label
- scenes/styles 的 keywords 可留空数组（后续系统自动学习同义词）
- 不要输出与资料无关的套话

只输出一行 JSON：
{"domain":"","scenes":[{"label":"","keywords":[]}],"styles":[{"label":"","keywords":[]}],"reason":""}`,
		profText, strings.Join(docLines, "\n"), strings.Join(styleNames, " | "))

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
	var parsed blueprintLLMResponse
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return zero, err
	}
	return parsed, nil
}

func trimBlueprintDefs(defs []model.LabelDefinition, maxN int) []model.LabelDefinition {
	var out []model.LabelDefinition
	seen := map[string]bool{}
	for _, d := range defs {
		d.Label = strings.TrimSpace(d.Label)
		if d.Label == "" || seen[d.Label] {
			continue
		}
		seen[d.Label] = true
		out = append(out, model.LabelDefinition{Label: d.Label, Keywords: d.Keywords})
		if len(out) >= maxN {
			break
		}
	}
	return out
}
