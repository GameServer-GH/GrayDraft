package retriever

import (
	"context"
	"encoding/json"
	"regexp"
	"sort"
	"strings"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/taxonomyevolve"
)

var (
	reQueryBookTitle = regexp.MustCompile(`[《「]([^》」\n]{2,24})[》」]`)
	reQueryProper    = regexp.MustCompile(`\b([A-Z][a-z]{2,16})\b`)
	reQueryHanToken  = regexp.MustCompile(`[\p{Han}]{2,8}`)
)

// ExtractQuery4D derives retrieval filters from the user query (LLM atomic NER first, rules fill gaps).
// tax is the effective workspace taxonomy (Blueprint); YAML catalog is cold-start scaffold only.
func ExtractQuery4D(ctx context.Context, query string, cfg *config.Config, llm *llmclient.Client, tax *model.DomainTaxonomy) model.QuerySemantic4D {
	q := strings.TrimSpace(query)
	if q == "" {
		return model.QuerySemantic4D{}
	}
	if tax == nil {
		cat := model.DefaultCatalog()
		if dom := cat.InferDomainFromQuery(q); dom != "" {
			tax = cat.Get(dom)
		} else {
			tax = cat.Default()
		}
	}
	rules := extractQuery4DRules(q, tax)
	var out model.QuerySemantic4D
	usedLLM := false
	if cfg != nil && cfg.Retrieval.Semantic4DLLMEnabled && llm != nil {
		if refined, err := extractQuery4DLLM(ctx, q, llm, tax); err == nil && refined.Active() {
			out = mergeQuery4DLlmFirst(refined, rules)
			out.Method = "llm"
			usedLLM = true
		}
	}
	if !usedLLM {
		out = rules
		if out.Active() {
			out.Method = "rules"
		}
	}
	out = constrainQuery4D(out, rules, q, tax)
	// When LLM extracted entities were all filtered and rules fell back to
	// whole-query strings, clear entities to avoid blocking strict tier.
	if out.Method == "llm" && len(out.Entities) == 1 &&
		strings.TrimSpace(strings.ToLower(out.Entities[0])) == strings.TrimSpace(strings.ToLower(q)) {
		out.Entities = nil
	}
	out.Entities = model.FinalSanitizeWithMethod(out.Entities, q, out.Method)
	out = model.NormalizeQuerySemantic4D(out, tax)
	out = taxonomyevolve.AlignQuery4D(ctx, out, tax)
	// Entity alias canonicalization runs in hybrid.Search when AliasIndex is loaded.
	if tax != nil {
		if d := strings.TrimSpace(tax.Domain); d != "" && d != model.UniversalDomainKey && d != model.DefaultCatalog().DefaultDomainKey() {
			out.Domain = d
		}
	}
	return out
}

// mergeQuery4DLlmFirst keeps LLM entities/scene/style; rules only fill empty dimensions.
func mergeQuery4DLlmFirst(llm, rules model.QuerySemantic4D) model.QuerySemantic4D {
	out := llm
	if strings.TrimSpace(out.Domain) == "" {
		out.Domain = rules.Domain
	}
	if strings.TrimSpace(out.Scene) == "" {
		out.Scene = rules.Scene
	}
	if strings.TrimSpace(out.Style) == "" {
		out.Style = rules.Style
	}
	if len(out.Entities) == 0 {
		out.Entities = rules.Entities
	}
	return out
}

func constrainQuery4D(q4, rules model.QuerySemantic4D, query string, tax *model.DomainTaxonomy) model.QuerySemantic4D {
	q4.Domain = constrainQueryDomain(q4.Domain, rules.Domain, tax)
	q4.Entities = constrainQueryEntities(q4.Entities, rules.Entities, query)
	return q4
}

func constrainQueryDomain(domain, fallback string, tax *model.DomainTaxonomy) string {
	domain = strings.TrimSpace(domain)
	if tax != nil {
		if d := strings.TrimSpace(tax.Domain); d != "" && d != model.DefaultCatalog().DefaultDomainKey() {
			return d
		}
	}
	if domain == "" || len([]rune(domain)) > 16 || strings.ContainsAny(domain, "，。；;：:\n") {
		return strings.TrimSpace(fallback)
	}
	return domain
}

func constrainQueryEntities(entities, fallback []string, query string) []string {
	qLower := strings.ToLower(strings.TrimSpace(query))
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || len([]rune(s)) < 2 || len([]rune(s)) > 16 {
			return
		}
		key := strings.ToLower(s)
		if seen[key] {
			return
		}
		if qLower != "" && !strings.Contains(qLower, key) {
			return
		}
		seen[key] = true
		out = append(out, s)
	}
	for _, e := range entities {
		add(e)
	}
	if len(out) == 0 {
		for _, e := range fallback {
			add(e)
		}
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func extractQuery4DRules(q string, tax *model.DomainTaxonomy) model.QuerySemantic4D {
	cat := model.DefaultCatalog()
	var out model.QuerySemantic4D
	workspaceTax := tax != nil && strings.TrimSpace(tax.Domain) != "" && tax.Domain != cat.DefaultDomainKey()
	if workspaceTax {
		tax.ApplyQueryHeuristics(q, &out)
		out.Domain = tax.Domain
	} else {
		dom := cat.InferDomainFromQuery(q)
		if dom == "" {
			dom = cat.DefaultDomainKey()
		}
		tax = cat.Get(dom)
		tax.ApplyQueryHeuristics(q, &out)
		if out.Domain == "" {
			out.Domain = dom
		}
	}
	out.Entities = model.FinalSanitize(extractQueryEntities(q), q)
	if out.Active() {
		out.Method = "rules"
	}
	return out
}

func extractQueryEntities(q string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || len([]rune(s)) < 2 || len([]rune(s)) > 24 || seen[s] {
			return
		}
		if model.LooksLikeConversationalFragment(s) {
			return
		}
		if model.IsStopEntity(s) {
			return
		}
		seen[s] = true
		out = append(out, s)
	}
	for _, m := range reQueryBookTitle.FindAllStringSubmatch(q, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, m := range reQueryProper.FindAllStringSubmatch(q, -1) {
		if len(m) > 1 {
			add(m[1])
		}
	}
	for _, p := range strings.FieldsFunc(q, func(r rune) bool {
		return r == ' ' || r == '，' || r == ',' || r == '、' || r == '。' || r == '？' || r == '?'
	}) {
		p = strings.Trim(p, "「」『』\"'《》")
		if len([]rune(p)) >= 2 && len([]rune(p)) <= 12 && !model.LooksLikeConversationalFragment(p) {
			add(p)
		}
	}
	for _, tok := range reQueryHanToken.FindAllString(q, -1) {
		add(tok)
	}
	for _, tok := range reEntityPrefixCandidates(q) {
		add(tok)
	}
	if len(out) > 6 {
		out = out[:6]
	}
	return out
}

func reEntityPrefixCandidates(q string) []string {
	var out []string
	for _, tok := range reQueryHanToken.FindAllString(q, -1) {
		r := []rune(tok)
		if len(r) >= 4 {
			out = append(out, string(r[:4]))
		}
	}
	return out
}

type query4dLLMResponse struct {
	Domain    string   `json:"domain"`
	Entities  []string `json:"entities"`
	Scene     string   `json:"scene"`
	Style     string   `json:"style"`
	Reasoning string   `json:"reasoning,omitempty"`
}

func extractQuery4DLLM(ctx context.Context, query string, llm *llmclient.Client, tax *model.DomainTaxonomy) (model.QuerySemantic4D, error) {
	if tax == nil {
		tax = model.DefaultCatalog().Default()
	}
	prompt := `你是检索查询分析助手。从用户问题中提取 4D 语义标签（仅基于问题本身，勿编造）：
- domain：领域（如 ` + tax.Domain + ` 等，无则空字符串）
- entities：仅提取专有名词（模型名、论文名、接口名、课程名等专名，最多 6 个）
- scene、style：必须从下列字典择一；无把握则留空字符串

` + tax.GeneratePromptBlock() + `

entities 负向约束（必须遵守，原子 NER）：
- 严禁包含动词、请求语或语气词（如：介绍、一下、如何、什么、吗、请、推荐、告诉）
- 只输出纯净专名（例：「介绍一下 InternVL」→ entities 仅 ["莫特斐"]）
- 不要输出整句或长短语；每个 entity 2–12 字为宜
	- 不要包含「什么」「如何」「一下」等泛词；不要编造问题未提及的专名
	- 如果问题泛化且未明确指明任何专名，entities 必须返回空数组 []

用户问题：
` + query + `

只输出一行 JSON：{"domain":"","entities":[],"scene":"","style":"","reasoning":""}`

	out, err := llm.Complete(ctx, prompt)
	if err != nil {
		return model.QuerySemantic4D{}, err
	}
	out = strings.TrimSpace(out)
	if i := strings.Index(out, "{"); i >= 0 {
		out = out[i:]
	}
	if j := strings.LastIndex(out, "}"); j >= 0 {
		out = out[:j+1]
	}
	var parsed query4dLLMResponse
	if err := json.Unmarshal([]byte(out), &parsed); err != nil {
		return model.QuerySemantic4D{}, err
	}
	q4 := model.QuerySemantic4D{
		Domain:    strings.TrimSpace(parsed.Domain),
		Scene:     strings.TrimSpace(parsed.Scene),
		Style:     strings.TrimSpace(parsed.Style),
		Entities:  parsed.Entities,
		Method:    "llm",
		Reasoning: strings.TrimSpace(parsed.Reasoning),
	}
	q4.Entities = model.FinalSanitizeWithMethod(q4.Entities, query, "llm")
	return model.NormalizeQuerySemantic4D(q4, tax), nil
}

func mergeQuery4D(base, llm model.QuerySemantic4D) model.QuerySemantic4D {
	if llm.Domain != "" {
		base.Domain = llm.Domain
	}
	if llm.Scene != "" {
		base.Scene = llm.Scene
	}
	if llm.Style != "" {
		base.Style = llm.Style
	}
	if len(llm.Entities) > 0 {
		seen := map[string]bool{}
		var ents []string
		for _, e := range append(llm.Entities, base.Entities...) {
			if e == "" || seen[e] {
				continue
			}
			seen[e] = true
			ents = append(ents, e)
		}
		if len(ents) > 6 {
			ents = ents[:6]
		}
		base.Entities = model.FinalSanitize(ents, "")
	}
	return base
}

// BoostStyleMatch adds a small score bonus when doc 4D style aligns with query style.
func BoostStyleMatch(chunks []model.Chunk, q model.QuerySemantic4D) {
	BoostSemantic4D(chunks, q)
}

// BoostSemantic4D boosts scores when doc 4D aligns with query (Tier-3 soft filter; no SQL WHERE).
func BoostSemantic4D(chunks []model.Chunk, q model.QuerySemantic4D) {
	qd := strings.ToLower(strings.TrimSpace(q.Domain))
	qs := strings.ToLower(strings.TrimSpace(q.Scene))
	qst := strings.ToLower(strings.TrimSpace(q.Style))
	if qd == "" && qs == "" && qst == "" && len(q.Entities) == 0 {
		return
	}
	for i := range chunks {
		ext, _ := chunks[i].Extensions["semantic_tags_4d"].(map[string]any)
		if ext == nil {
			continue
		}
		bonus := 0.0
		if qd != "" {
			if dd, _ := ext["domain"].(string); dd != "" {
				ddl := strings.ToLower(dd)
				if ddl == qd || strings.Contains(ddl, qd) || strings.Contains(qd, ddl) {
					bonus += 0.06
				}
			}
		}
		if qs != "" {
			if ds, _ := ext["scene"].(string); ds != "" {
				dsl := strings.ToLower(ds)
				if strings.Contains(dsl, qs) || strings.Contains(qs, dsl) {
					bonus += 0.08
				}
			}
		}
		if qst != "" {
			if dst, _ := ext["style"].(string); dst != "" {
				dstl := strings.ToLower(dst)
				if strings.Contains(dstl, qst) || strings.Contains(qst, dstl) {
					bonus += 0.04
				}
			}
		}
		if len(q.Entities) > 0 {
			if ents, ok := ext["entities"].([]any); ok {
				for _, qe := range q.Entities {
					qel := strings.ToLower(strings.TrimSpace(qe))
					if qel == "" {
						continue
					}
					for _, raw := range ents {
						ent, _ := raw.(string)
						entl := strings.ToLower(strings.TrimSpace(ent))
						if entl == "" {
							continue
						}
						if strings.Contains(entl, qel) || strings.Contains(qel, entl) {
							bonus += 0.10
							break
						}
					}
				}
			}
		}
		if bonus > 0 {
			if chunks[i].Score > 0 {
				chunks[i].Score = min(1.0, chunks[i].Score+bonus)
			}
			if chunks[i].VectorScore > 0 {
				chunks[i].VectorScore = min(1.0, chunks[i].VectorScore+bonus*0.5)
			}
		}
	}
	sort.SliceStable(chunks, func(i, j int) bool {
		if chunks[i].Score == chunks[j].Score {
			return chunks[i].ID < chunks[j].ID
		}
		return chunks[i].Score > chunks[j].Score
	})
}
