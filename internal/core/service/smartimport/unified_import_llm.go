package smartimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/multimodal"
	"github.com/graydraft/core/service/promptmd"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/textutil"
)

const (
	extImportLLMUnified    = "import_llm_unified"
	extImportLLMUnifiedOK       = "import_llm_unified_ok"
	extImportLLMUnifiedFailCnt  = "import_llm_unified_fail_count"
	extImportLLMBodyHash        = "import_llm_body_hash"
)

// UnifiedImportParams is input for the single combined Fast LLM call per import.
type UnifiedImportParams struct {
	Content             string
	UserNote            string
	AttachmentNames     []string
	WorkspaceBackground string
	Skeleton            DocumentSkeleton
	ForceSingle         bool
	HintKBType          string
	RulesKBType         string
	RulesTitle          string
	RulesConfidence     float64
	RulesReason         string
	MixedContent        bool
}

// UnifiedImportResult holds title, semantic pivot, classification, and optional split plan.
type UnifiedImportResult struct {
	Title        string
	Pivot        SemanticPivotResult
	MappingDelta model.MappingDelta
	KBType       string
	Confidence   float64
	Reason       string
	SplitPlan    *SplitPlan
}

type unifiedImportLLMJSON struct {
	Title              string                  `json:"title"`
	Domain             string                  `json:"domain"`
	Entities           []string                `json:"entities"`
	Scene              string                  `json:"scene"`
	Style              string                  `json:"style"`
	GlobalSummary      string                  `json:"global_summary"`
	StructuredMarkdown string                  `json:"structured_markdown"`
	Outline            []VirtualOutlineSection `json:"outline"`
	KbType             string                  `json:"kb_type"`
	Confidence         float64                 `json:"confidence"`
	Reason             string                  `json:"reason"`
	DocAnalysis        DocAnalysis             `json:"doc_analysis"`
	SplittingStrategy  string                  `json:"splitting_strategy"`
	Segments           []struct {
		Range              [2]int   `json:"range"`
		Title              string   `json:"title"`
		Tags               []string `json:"tags"`
		ContextEnhancement string   `json:"context_enhancement"`
		SearchIntent       string   `json:"search_intent"`
		SemanticSummary    string   `json:"semantic_summary"`
	} `json:"segments"`
	LogicSkeleton []LogicSkeletonSection `json:"logic_skeleton"`
	KeyPhrases    []string               `json:"key_phrases"`
}

const unifiedSnippetMaxRunes = 12000

// RunUnifiedImportLLM performs title + 4D pivot + kb_type + split planning in one Fast LLM request.
func RunUnifiedImportLLM(p UnifiedImportParams, complete func(string) (string, error)) (UnifiedImportResult, error) {
	if complete == nil {
		return UnifiedImportResult{}, fmt.Errorf("llm not available")
	}
	snippet := strings.TrimSpace(p.Content)
	if len([]rune(snippet)) > unifiedSnippetMaxRunes {
		snippet = string([]rune(snippet)[:unifiedSnippetMaxRunes]) + "…"
	}
	splitHint := "本次必须 splitting_strategy=single，segments 仅一条 range 覆盖全文。"
	if !p.ForceSingle {
		splitHint = "仅当正文明显多主题（合集、分段、多段独立 FAQ）时用 multi_segment；单篇说明/时间表默认 single，勿过度拆分。"
	}
	mixedHint := ""
	if p.MixedContent {
		mixedHint = "正文混杂多主题时仍保持 splitting_strategy 合理拆分，勿过度拆段。"
	}
	hintLine := ""
	partition, _, _ := ResolveKBType(p.HintKBType)
	if partition == "" {
		return UnifiedImportResult{}, fmt.Errorf("%w", ErrWorkspaceKBTypeRequired)
	}
	hintLine = fmt.Sprintf("工作区分区已锁定 kb_type=%q（必须原样输出，必须等于当前工作区分区）。\n", partition)
	bgBlock := "(无)"
	if b := strings.TrimSpace(p.WorkspaceBackground); b != "" {
		bgBlock = b
	}
	namesBlock := strings.Join(p.AttachmentNames, "\n")
	if namesBlock == "" {
		namesBlock = "(无)"
	}
	skeleton := FormatSkeletonForLLM(p.Skeleton)
	anchors := FormatSkeletonAnchorsForLLM(p.Skeleton)
	tax := taxonomyevolve.Effective(context.Background(), p.HintKBType, nil, p.WorkspaceBackground)

	prompt := fmt.Sprintf(`你是 GrayDraft 智能导入一体化助手（语义导轨模式）。根据正文与工作区先验完成入库元数据（仅一次 JSON）。

【工作区背景 — 分类与实体须符合此领域，勿偏离】
%s

%s
规则预判：kb_type=%s, title=%q, confidence=%.2f, reason=%s
%s

【任务 1 标题】8–48 字；禁止空泛汇总标题；须符合工作区领域实体。
【任务 2 语义 4D】domain, entities(≤8), scene, style；entities 须为正文或工作区中出现的专名。
另输出 key_phrases：最多 3 个正文专有检索词（接口名、术语等），勿用泛词。
` + tax.GeneratePromptBlock() + `
【任务 3 global_summary】80–200 字。
【任务 4 结构】优先 structured_markdown；否则 outline。
%s
【任务 5 分类】kb_type 必须与工作区分区一致（见上文 hint），必须等于当前工作区分区。
【任务 6 语义导轨 logic_skeleton】%s
  为每个知识单元输出：range [start,end]（1-based，对齐下方结构锚点）、anchor_kind(h1|h2|h3|sep|)、title_hint（来自该行原文标题）、semantic_summary（20–80 字，概括该段检索主题，勿编造正文没有的事实）。
【任务 7 拆分 segments】与 logic_skeleton 一一对应（条数相同）；segments[].semantic_summary 与 logic_skeleton 一致。

文档总行数：%d
用户说明：%s
附件名：%s

%s

结构骨架（行号|kind|text）：
%s

正文：
%s

只输出一行 JSON（含 logic_skeleton）：
{"title":"","domain":"","entities":[],"scene":"","style":"","key_phrases":[],"global_summary":"","structured_markdown":"","outline":[],"kb_type":"","confidence":0.8,"reason":"","doc_analysis":{"subject":"","version":"","doc_type":"","parent_title":""},"splitting_strategy":"single|multi_segment","logic_skeleton":[{"range":[1,N],"anchor_kind":"h2","title_hint":"","semantic_summary":""}],"segments":[{"range":[1,N],"title":"","tags":[],"context_enhancement":"","search_intent":"","semantic_summary":""}]}`,
		bgBlock, hintLine, p.RulesKBType, p.RulesTitle, p.RulesConfidence, p.RulesReason, mixedHint,
		promptmd.SceneStructuredMarkdown, splitHint,
		p.Skeleton.LineCount, orDefault(p.UserNote, "(无)"), namesBlock, anchors, skeleton, orDefault(snippet, "(无)"))

	out, err := complete(prompt)
	if err != nil {
		return UnifiedImportResult{}, err
	}
	var parsed unifiedImportLLMJSON
	if err := parseJSONFromLLM(out, &parsed); err != nil {
		// Field-graded repair: attempt to salvage core fields (title, summary)
		// while rejecting structurally unsafe partial repairs.
		repaired := repairCoreFieldsFromLLMOutput(out)
		if repaired.Title != "" {
			parsed.Title = repaired.Title
		}
		if repaired.GlobalSummary != "" {
			parsed.GlobalSummary = repaired.GlobalSummary
		}
		// Structure fields (logic_skeleton, segments, range) MUST parse fully; never partial.
		// If they failed, we let buildUnifiedImportResult fall back to single-segment rules.
	}
	return buildUnifiedImportResult(parsed, p.Content, p.Skeleton, p)
}

func buildUnifiedImportResult(parsed unifiedImportLLMJSON, raw string, sk DocumentSkeleton, p UnifiedImportParams) (UnifiedImportResult, error) {
	title := textutil.StripEmojis(strings.TrimSpace(parsed.Title))
	if title == "" || multimodal.IsImportIntentNote(title) || multimodal.IsGenericConsolidatedTitle(title) {
		title = strings.TrimSpace(p.RulesTitle)
	}
	if len([]rune(title)) > 120 {
		title = string([]rune(title)[:120])
	}

	tax := taxonomyevolve.Effective(context.Background(), p.HintKBType, nil, p.WorkspaceBackground)
	domain := textutil.StripEmojis(strings.TrimSpace(parsed.Domain))
	if domain == "" {
		domain = tax.Domain
	}
	sceneRaw := textutil.StripEmojis(strings.TrimSpace(parsed.Scene))
	styleRaw := textutil.StripEmojis(strings.TrimSpace(parsed.Style))
	pivot := SemanticPivotResult{
		Tags4D: SemanticTags4D{
			Domain:   domain,
			Scene:    taxonomyevolve.AlignScene(context.Background(), sceneRaw, tax),
			Style:    taxonomyevolve.AlignStyle(context.Background(), styleRaw, tax),
			Entities: cleanEntityList(parsed.Entities, p.Content),
		},
		GlobalSummary:      textutil.StripEmojis(strings.TrimSpace(parsed.GlobalSummary)),
		StructuredMarkdown: strings.TrimSpace(parsed.StructuredMarkdown),
		Outline:            normalizeOutline(parsed.Outline, sk.LineCount),
	}

	kbType, conf, reason := ResolveKBType(p.HintKBType)
	if kbType == "" {
		kbType = p.RulesKBType
	}
	// KB type whitelist: validate that LLM-returned kb_type is a valid gd_* partition.
	// Reject any fabricated value (plan 2.4, 2.8).
	if parsed.KbType != "" && parsed.KbType != kbType {
		if IsWorkspacePartition(parsed.KbType) {
			kbType = parsed.KbType
		}
		// Non-gd_* or unknown partition: silently use hint_kb_type (logged in caller)
	}
	if conf <= 0 {
		conf = p.RulesConfidence
	}
	if parsed.Confidence > 0 && parsed.Confidence <= 1 {
		conf = parsed.Confidence
	}
	reason = strings.TrimSpace(p.RulesReason)
	if r := strings.TrimSpace(parsed.Reason); r != "" {
		reason = r + "; workspace locked"
	} else {
		reason += "; workspace kb_type locked"
	}
	llmD := llmDraft{Title: title, Confidence: conf, Reason: reason}
	kbType, title, conf, reason = mergeLLMRefinement(kbType, p.RulesTitle, conf, reason, p.Content, llmD, true)

	splitResp := llmSplitResponse{
		DocAnalysis:       parsed.DocAnalysis,
		SplittingStrategy: parsed.SplittingStrategy,
	}
	for _, s := range parsed.Segments {
		splitResp.Segments = append(splitResp.Segments, struct {
			Range              [2]int   `json:"range"`
			Title              string   `json:"title"`
			Tags               []string `json:"tags"`
			ContextEnhancement string   `json:"context_enhancement"`
			SearchIntent       string   `json:"search_intent"`
			SemanticSummary    string   `json:"semantic_summary"`
		}{
			Range: s.Range, Title: s.Title, Tags: s.Tags,
			ContextEnhancement: s.ContextEnhancement, SearchIntent: s.SearchIntent, SemanticSummary: s.SemanticSummary,
		})
	}
	var plan SplitPlan
	var err error
	if len(parsed.LogicSkeleton) > 0 {
		analysis := mergeAnalysis(sk.Analysis, parsed.DocAnalysis)
		plan, err = buildSplitPlanFromLogicSkeleton(parsed.LogicSkeleton, raw, sk, analysis)
		if err != nil {
			plan, err = buildSplitPlanFromLLMResponse(splitResp, raw, sk)
		}
	} else {
		plan, err = buildSplitPlanFromLLMResponse(splitResp, raw, sk)
	}
	if err != nil {
		plan = SplitPlan{
			DocAnalysis: mergeAnalysis(sk.Analysis, parsed.DocAnalysis),
			Strategy:    strategySingle,
			Method:      "unified_llm_fallback",
			Segments:    []PlannedSegment{singleSegment(raw, sk.Analysis)},
		}
	} else {
		plan = alignSplitPlanToSkeletonRails(plan, sk, raw)
	}
	if p.ForceSingle {
		plan.Strategy = strategySingle
		plan.Segments = []PlannedSegment{singleSegment(raw, mergeAnalysis(sk.Analysis, parsed.DocAnalysis))}
		plan.Method = "unified_llm"
	} else {
		plan.Method = "unified_llm"
	}

	delta := model.MappingDelta{
		Scene:      pivot.Tags4D.Scene,
		Style:      pivot.Tags4D.Style,
		KeyPhrases: cleanKeyPhrases(parsed.KeyPhrases),
	}
	return UnifiedImportResult{
		Title:        title,
		Pivot:        pivot,
		MappingDelta: delta,
		KBType:       kbType,
		Confidence:   conf,
		Reason:       reason,
		SplitPlan:    &plan,
	}, nil
}

func cleanKeyPhrases(in []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, p := range in {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] || len([]rune(p)) > 32 {
			continue
		}
		seen[p] = true
		out = append(out, p)
		if len(out) >= 3 {
			break
		}
	}
	return out
}

func buildSplitPlanFromLLMResponse(parsed llmSplitResponse, raw string, sk DocumentSkeleton) (SplitPlan, error) {
	strategy := strings.TrimSpace(parsed.SplittingStrategy)
	if strategy == "" {
		strategy = strategySingle
	}
	if strategy != strategyMulti {
		strategy = strategySingle
	}
	analysis := mergeAnalysis(sk.Analysis, parsed.DocAnalysis)
	var segs []PlannedSegment
	for _, s := range parsed.Segments {
		start, end := s.Range[0], s.Range[1]
		if start < 1 || end < start || end > sk.LineCount {
			continue
		}
		content := ExtractByLineRange(raw, start, end)
		if len([]rune(content)) < 20 || isClosingFluff(content) || isAttachmentFilenameOnlySegment(content) {
			continue
		}
		t := textutil.StripEmojis(strings.TrimSpace(s.Title))
		if t == "" || !titleAppearsInContent(t, content) {
			t = segmentTitleFromRange(raw, start, end, analysis)
		}
		seg := PlannedSegment{
			Content:            content,
			LineRange:          [2]int{start, end},
			Title:              displayTitle(analysis.ParentTitle, t),
			Tags:               s.Tags,
			ContextEnhancement: textutil.StripEmojis(strings.TrimSpace(s.ContextEnhancement)),
			SearchIntent:       textutil.StripEmojis(strings.TrimSpace(s.SearchIntent)),
			SemanticSummary:    textutil.StripEmojis(strings.TrimSpace(s.SemanticSummary)),
		}
		if seg.ContextEnhancement == "" {
			seg.ContextEnhancement = buildContextEnhancement(analysis, seg.Title)
		}
		if seg.SemanticSummary == "" {
			seg.SemanticSummary = seg.Title
		}
		segs = append(segs, seg)
	}
	if len(segs) == 0 {
		return SplitPlan{}, fmt.Errorf("no valid segments")
	}
	return SplitPlan{DocAnalysis: analysis, Strategy: strategy, Segments: segs}, nil
}

const extImportLLMSplitPlanJSON = "import_llm_split_plan_json"

func storeUnifiedImportCache(ext model.Extensions, fullBody string, u UnifiedImportResult) {
	if ext == nil {
		return
	}
	ext[extImportLLMUnifiedOK] = true
	if fullBody != "" {
		sum := sha256.Sum256([]byte(normalizeRaw(fullBody)))
		ext[extImportLLMBodyHash] = hex.EncodeToString(sum[:])
	}
	ext[extImportLLMUnified] = map[string]any{
		"title": u.Title, "kb_type": u.KBType, "confidence": u.Confidence, "reason": u.Reason,
		"domain": u.Pivot.Tags4D.Domain, "scene": u.Pivot.Tags4D.Scene, "style": u.Pivot.Tags4D.Style,
		"entities": u.Pivot.Tags4D.Entities, "global_summary": u.Pivot.GlobalSummary,
	}
	if u.SplitPlan != nil {
		if b, err := json.Marshal(u.SplitPlan); err == nil {
			ext[extImportLLMSplitPlanJSON] = string(b)
		}
	}
	ext["import_enrichment_complete"] = true
}

func unifiedImportFailCount(ext model.Extensions) int {
	if ext == nil {
		return 0
	}
	switch v := ext[extImportLLMUnifiedFailCnt].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	default:
		return 0
	}
}

func unifiedImportCacheHit(ext model.Extensions) bool {
	if ext == nil {
		return false
	}
	ok, _ := ext[extImportLLMUnifiedOK].(bool)
	return ok
}

// useLegacyImportLLM is false when unified import LLM already produced metadata for this request.
func useLegacyImportLLM(ext model.Extensions) bool {
	if unifiedImportCacheHit(ext) {
		return false
	}
	if ext == nil {
		return true
	}
	v, tried := ext[extImportLLMUnifiedOK]
	if !tried {
		return true
	}
	ok, _ := v.(bool)
	return !ok
}

func draftFromUnifiedCache(ext model.Extensions, content, filename, mime string) (Draft, bool) {
	if !unifiedImportCacheHit(ext) {
		return Draft{}, false
	}
	if h, _ := ext[extImportLLMBodyHash].(string); h != "" {
		sum := sha256.Sum256([]byte(normalizeRaw(content)))
		if hex.EncodeToString(sum[:]) != h {
			return Draft{}, false
		}
	}
	m, _ := ext[extImportLLMUnified].(map[string]any)
	if m == nil {
		return Draft{}, false
	}
	title, _ := m["title"].(string)
	kbType, _ := m["kb_type"].(string)
	conf, _ := m["confidence"].(float64)
	reason, _ := m["reason"].(string)
	if title == "" {
		title = extractTitle(content)
	}
	sum := sha256.Sum256([]byte(content))
	return Draft{
		KBType: kbType, Title: title, Content: content,
		ContentHash: hex.EncodeToString(sum[:]), MimeType: inferMime(filename, mime),
		Confidence: conf, Method: "unified_llm", Reason: reason,
	}, true
}

func loadCachedSplitPlan(ext model.Extensions, raw string, forceSingle bool) (SplitPlan, bool) {
	if !unifiedImportCacheHit(ext) || ext == nil {
		return SplitPlan{}, false
	}
	rawJSON, _ := ext[extImportLLMSplitPlanJSON].(string)
	if rawJSON == "" {
		return SplitPlan{}, false
	}
	var plan SplitPlan
	if err := json.Unmarshal([]byte(rawJSON), &plan); err != nil || len(plan.Segments) == 0 {
		return SplitPlan{}, false
	}
	if forceSingle {
		sk := ScanSkeleton(normalizeRaw(raw))
		plan.Strategy = strategySingle
		plan.Segments = []PlannedSegment{singleSegment(raw, mergeAnalysis(sk.Analysis, plan.DocAnalysis))}
	}
	return plan, true
}

func applyUnifiedTitleToRaw(req *model.SmartImportRequest, title string) {
	title = strings.TrimSpace(title)
	if title == "" || multimodal.IsGenericConsolidatedTitle(title) {
		return
	}
	if req.Extensions == nil {
		req.Extensions = model.Extensions{}
	}
	req.Extensions["import_title_hint"] = title
	raw := req.RawText
	if md := multimodal.ExtractMarkdownH1(raw); md != "" {
		if strings.TrimSpace(md) == title {
			return
		}
		lines := strings.Split(raw, "\n")
		for i, line := range lines {
			trim := strings.TrimSpace(line)
			if strings.HasPrefix(trim, "# ") {
				lines[i] = "# " + title
				req.RawText = strings.TrimSpace(strings.Join(lines, "\n"))
				return
			}
		}
	}
	req.RawText = strings.TrimSpace("# " + title + "\n\n" + raw)
}

// repairCoreFieldsFromLLMOutput attempts to salvage core fields from malformed JSON.
// Only repairs title and global_summary — structure fields (logic_skeleton, range, segments)
// must parse fully and are never partially repaired.
type coreFieldRepair struct {
	Title         string
	GlobalSummary string
}

var (
	reRepairTitle         = regexp.MustCompile(`"title"\s*:\s*"([^"]+)"`)
	reRepairGlobalSummary = regexp.MustCompile(`"global_summary"\s*:\s*"([^"]+)"`)
)

func repairCoreFieldsFromLLMOutput(out string) coreFieldRepair {
	var r coreFieldRepair
	if m := reRepairTitle.FindStringSubmatch(out); len(m) > 1 {
		r.Title = strings.TrimSpace(m[1])
	}
	if m := reRepairGlobalSummary.FindStringSubmatch(out); len(m) > 1 {
		r.GlobalSummary = strings.TrimSpace(m[1])
	}
	return r
}

