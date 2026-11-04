package smartimport

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"os"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/agentlog"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/embedder"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/multimodal"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/ugc"
	"github.com/graydraft/core/service/workspacecontext"
)

// ErrBelowMinConfidence is returned when submit is blocked by low confidence.
type ErrBelowMinConfidence struct {
	Response *model.SmartImportResponse
}

func (e *ErrBelowMinConfidence) Error() string {
	return e.Response.Message
}

// Service runs agent-assisted smart import.
type Service struct {
	ugc          *ugc.Service
	docs         *postgres.DocumentRepo
	chunks       *postgres.ChunkRepo
	llm          *llmclient.Client
	embed        *embedder.Client
	multimodal   *multimodal.Processor
	minConf      float64
	agentLogRoot string
	cacheMgr     *cache.Manager
	answerCache  *answercache.Manager
}

// NewService creates smart import service.
func NewService(ugcSvc *ugc.Service, docs *postgres.DocumentRepo, chunks *postgres.ChunkRepo, llm *llmclient.Client, emb *embedder.Client, mm *multimodal.Processor, cfg *config.Config) *Service {
	min := 0.55
	if cfg != nil && cfg.SmartImport.MinConfidence > 0 {
		min = cfg.SmartImport.MinConfidence
	}
	logRoot := strings.TrimSpace(os.Getenv("AGENT_LOG_ROOT"))
	if logRoot == "" {
		logRoot = "."
	}
	return &Service{
		ugc: ugcSvc, docs: docs, chunks: chunks, llm: llm, embed: emb, multimodal: mm,
		minConf: min, agentLogRoot: logRoot,
	}
}

// WithCaches enables best-effort retrieval/answer cache purge after successful submit.
func (s *Service) WithCaches(cacheMgr *cache.Manager, answerCache *answercache.Manager) *Service {
	if s == nil {
		return nil
	}
	s.cacheMgr = cacheMgr
	s.answerCache = answerCache
	return s
}

// Run analyzes raw input and optionally submits UGC ingest.
func (s *Service) Run(ctx context.Context, req model.SmartImportRequest) (*model.SmartImportResponse, error) {
	sess, _ := agentlog.StartFromContext(ctx, s.agentLogRoot, "smart-import")
	var runErr error
	defer func() {
		if sess != nil {
			sess.Finish(runErr)
		}
	}()
	if sess != nil {
		sess.Step("request_in", map[string]any{
			"request": agentlog.SanitizeRequest(req),
		})
	}

	if err := validateSmartImportInput(req); err != nil {
		runErr = err
		return s.baseResp(s.minConf, false, false, err.Error()), err
	}
	mmSummary, mmResults, err := s.preprocessRequest(ctx, &req)
	if err != nil {
		runErr = err
		return s.baseResp(s.minConf, false, false, err.Error()), err
	}
	if sess != nil {
		mmFields := map[string]any{
			"extractions": mmSummary,
			"raw_len":     len(req.RawText),
		}
		if len(mmResults) > 0 {
			texts := make([]map[string]any, 0, len(mmResults))
			for _, r := range mmResults {
				texts = append(texts, map[string]any{
					"filename": r.Filename,
					"route":    r.Route,
					"md5":      r.MD5,
					"text":     r.Text,
					"cached":   r.Cached,
				})
			}
			mmFields["extraction_texts"] = texts
		}
		sess.Step("multimodal_preprocess", mmFields)
	}
	s.runUnifiedImportLLM(ctx, &req, sess, unifiedForceSingle(req))
	minRequired := s.minConf
	if req.MinConfidence != nil && *req.MinConfidence > 0 {
		minRequired = *req.MinConfidence
	}
	if strings.TrimSpace(req.TargetDocID) != "" {
		req.Split = false
	}

	var resp *model.SmartImportResponse
	if req.Split {
		resp, runErr = s.runSplit(ctx, req, minRequired, sess)
	} else {
		resp, runErr = s.runSingle(ctx, req, minRequired, sess)
	}
	if resp != nil && len(mmSummary) > 0 {
		resp.MultimodalExtractions = mmSummary
	}
	if resp != nil {
		resp.Readiness = importReadiness(req.Submit, resp)
	}
	if sess != nil && resp != nil {
		sess.StepData("response_out", resp)
	}
	return resp, runErr
}

func (s *Service) llmFn(ctx context.Context, req model.SmartImportRequest, sess *agentlog.Session) func(string) (string, error) {
	var base func(string) (string, error)
	if !req.UseLLM {
		return nil
	}
	if req.LLMOverride.Valid() {
		o := &llmclient.Override{
			Provider: req.LLMOverride.Provider,
			Model:    req.LLMOverride.Model,
			APIKey:   req.LLMOverride.APIKey,
		}
		base = func(prompt string) (string, error) {
			return o.Complete(ctx, prompt)
		}
	} else if s.llm != nil {
		base = func(prompt string) (string, error) {
			return s.llm.Complete(ctx, prompt)
		}
	}
	if base == nil {
		return nil
	}
	if sess == nil {
		return base
	}
	var n int
	return func(prompt string) (string, error) {
		n++
		sess.Step("llm_call_start", map[string]any{
			"call":        n,
			"prompt_len":  len(prompt),
			"prompt_head": truncateRunes(prompt, 1500),
		})
		out, err := base(prompt)
		sess.Step("llm_call_done", map[string]any{
			"call":          n,
			"error":         errString(err),
			"response_len":  len(out),
			"response_head": truncateRunes(out, 1500),
		})
		return out, err
	}
}

func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func (s *Service) runSingle(ctx context.Context, req model.SmartImportRequest, minRequired float64, sess *agentlog.Session) (*model.SmartImportResponse, error) {
	sk := ScanSkeleton(req.RawText)
	preview, err := s.analyzeOne(ctx, req, minRequired, req.RawText, req.HintKBType, req.Filename, req.MimeType, sess)
	if err != nil {
		return s.baseResp(minRequired, false, false, err.Error()), err
	}
	if sess != nil {
		sess.Step("analyze_result", map[string]any{
			"kb_type":      preview.KBType,
			"title":        preview.Title,
			"confidence":   preview.Confidence,
			"action":       preview.Action,
			"method":       preview.Method,
			"content_head": truncateRunes(preview.Content, 500),
		})
	}
	if hint := importTitleHint(req.Extensions); hint != "" {
		preview.Title = hint
	} else if t := strings.TrimSpace(sk.Analysis.ParentTitle); t != "" && (strings.TrimSpace(preview.Title) == "" || isInvalidTitle(preview.Title)) {
		preview.Title = t
	}

	resp := s.baseResp(minRequired, preview.ImportAllowed, false, "")
	resp.Preview = &preview
	resp.Confidence = preview.Confidence
	resp.DocAnalysis = toModelDocAnalysis(sk.Analysis)
	resp.SplittingStrategy = strategySingle
	if kb := preview.KBType; kb != "" && req.TargetDocID == "" {
		resp.SimilarDocuments = s.findSimilarDocuments(ctx, kb, preview.Title, preview.Content)
	}

	if !req.Submit {
		return resp, nil
	}
	if !preview.ImportAllowed {
		resp.Message = fmt.Sprintf("import rejected: rule checks failed (confidence=%.2f, min=%.2f), reason: %s", preview.Confidence, minRequired, preview.Reason)
		return resp, &ErrBelowMinConfidence{Response: resp}
	}
	if preview.Action == "unchanged" {
		resp.Message = "content unchanged, skip ingest"
		return resp, nil
	}

	ext := MergeSegmentExtensions(req.Extensions, sk.Analysis, singleSegment(req.RawText, sk.Analysis), 1, 1)
	ext = MergeDocumentExtensions(ext, req.Extensions)
	task, err := s.submitPreview(ctx, req, preview, ext)
	if err != nil {
		return resp, err
	}
	if sess != nil {
		sess.Step("submit_result", map[string]any{
			"channel": task.Channel,
			"doc_id":  task.DocID,
			"task_id": task.TaskID,
		})
	}
	resp.Task = task
	resp.Tasks = []model.UGCNotifyResponse{*task}
	s.purgeCachesAfterSubmit(ctx, preview.KBType)
	return resp, nil
}

func (s *Service) runSplit(ctx context.Context, req model.SmartImportRequest, minRequired float64, sess *agentlog.Session) (*model.SmartImportResponse, error) {
	forceSingle := isMultimodalConsolidated(req.Extensions)
	plan := PlanSegments(req.RawText, req.UseLLM, s.llmFn(ctx, req, sess), forceSingle, req.Extensions)
	if sess != nil {
		sess.StepData("split_plan", plan)
	}
	if len(req.SegmentsOverride) > 0 {
		plan.Segments = applySegmentOverrides(req.RawText, req.SegmentsOverride, plan.Segments)
		plan.Strategy = strategyMulti
	}

	// MinHash+LSH cluster dedup for batch imports (optimization plan 1.6)
	if len(plan.Segments) > 3 {
		before := len(plan.Segments)
		plan.Segments = MergeClusteredSegments(plan.Segments, req.RawText, ScanSkeleton(req.RawText))
		if sess != nil && len(plan.Segments) < before {
			sess.Step("minhash_dedup", map[string]any{
				"before": before,
				"after":  len(plan.Segments),
				"merged": before - len(plan.Segments),
			})
		}
	}

	plan = preserveSourcePlan(req.RawText, plan)
	if req.Extensions == nil {
		req.Extensions = model.Extensions{}
	}
	req.Extensions["source_integrity_method"] = plan.Method
	if data, err := json.Marshal(plan); err == nil {
		req.Extensions[extImportLLMSplitPlanJSON] = string(data)
	}
	resp := s.baseResp(minRequired, false, true, "")
	resp.DocAnalysis = toModelDocAnalysis(plan.DocAnalysis)
	resp.SplittingStrategy = plan.Strategy
	resp.SplitMethod = plan.Method

	if hint := strings.TrimSpace(req.HintKBType); hint != "" && req.TargetDocID == "" {
		title := plan.DocAnalysis.ParentTitle
		resp.SimilarDocuments = s.findSimilarDocuments(ctx, hint, title, req.RawText)
	}

	var minSeen = 1.0
	total := len(plan.Segments)

	for i, seg := range plan.Segments {
		preview, err := s.analyzeSegment(ctx, req, minRequired, seg, plan.DocAnalysis, req.HintKBType, req.Filename, req.MimeType, sess)
		if err != nil {
			resp.Skipped = append(resp.Skipped, model.SmartImportSkipped{
				Index: i, Reason: err.Error(), Confidence: 0,
			})
			minSeen = 0
			continue
		}
		if sess != nil {
			sess.Step("analyze_result", map[string]any{
				"segment":      i + 1,
				"total":        total,
				"kb_type":      preview.KBType,
				"title":        preview.Title,
				"confidence":   preview.Confidence,
				"action":       preview.Action,
				"method":       preview.Method,
				"content_head": truncateRunes(preview.Content, 500),
			})
		}
		if preview.Confidence < minSeen {
			minSeen = preview.Confidence
		}
		item := model.SmartImportItem{
			Index:              i,
			Preview:            preview,
			ImportAllowed:      preview.ImportAllowed,
			LineRange:          seg.LineRange,
			ContextEnhancement: seg.ContextEnhancement,
		}
		resp.Items = append(resp.Items, item)
	}

	resp.Confidence = minSeen
	resp.ImportAllowed = minSeen >= minRequired && len(resp.Items) > 0 && anyImportable(resp.Items)
	analysis := applyChildDerivedParent(plan.DocAnalysis, resp.Items)
	resp.DocAnalysis = toModelDocAnalysis(analysis)

	if !req.Submit {
		if len(resp.Items) == 0 {
			resp.Message = "no segments after split"
		}
		return resp, nil
	}

	if !resp.ImportAllowed {
		resp.Message = fmt.Sprintf("import rejected: rule checks failed on all segments (min_confidence=%.2f, required=%.2f, importable=%d/total=%d)", minSeen, minRequired, countImportable(resp.Items), len(resp.Items))
		return resp, &ErrBelowMinConfidence{Response: resp}
	}

	for _, item := range resp.Items {
		if !item.ImportAllowed {
			resp.Skipped = append(resp.Skipped, model.SmartImportSkipped{
				Index:      item.Index,
				Reason:     fmt.Sprintf("confidence %.2f < %.2f", item.Preview.Confidence, minRequired),
				Confidence: item.Preview.Confidence,
			})
			continue
		}
		if item.Preview.Action == "unchanged" {
			resp.Skipped = append(resp.Skipped, model.SmartImportSkipped{
				Index: item.Index, Reason: "unchanged", Confidence: item.Preview.Confidence,
			})
			continue
		}
		seg := plan.Segments[item.Index]
		ext := MergeSegmentExtensions(req.Extensions, analysis, seg, item.Index+1, total)
		ext = MergeDocumentExtensions(ext, req.Extensions)
		task, err := s.submitPreview(ctx, req, item.Preview, ext)
		if err != nil {
			resp.Skipped = append(resp.Skipped, model.SmartImportSkipped{
				Index: item.Index, Reason: err.Error(), Confidence: item.Preview.Confidence,
			})
			continue
		}
		if sess != nil {
			sess.Step("submit_result", map[string]any{
				"segment": item.Index + 1,
				"channel": task.Channel,
				"doc_id":  task.DocID,
				"task_id": task.TaskID,
			})
		}
		resp.Tasks = append(resp.Tasks, *task)
	}

	if len(resp.Tasks) == 0 {
		resp.ImportAllowed = false
		resp.Message = "no segment submitted (all below threshold or unchanged)"
		return resp, &ErrBelowMinConfidence{Response: resp}
	}
	resp.Message = fmt.Sprintf("submitted %d segment(s)", len(resp.Tasks))
	s.purgeCachesAfterSubmit(ctx, strings.TrimSpace(req.HintKBType))
	return resp, nil
}

func (s *Service) analyzeSegment(
	ctx context.Context,
	req model.SmartImportRequest,
	minRequired float64,
	seg PlannedSegment,
	analysis DocAnalysis,
	hint, filename, mime string,
	sess *agentlog.Session,
) (model.SmartImportPreview, error) {
	preview, err := s.analyzeOne(ctx, req, minRequired, seg.Content, hint, filename, mime, sess)
	if err != nil {
		return model.SmartImportPreview{}, err
	}
	if t := resolvePreviewTitle(seg.Title, seg.Content, analysis); t != "" {
		preview.Title = t
	}
	return preview, nil
}

func (s *Service) analyzeOne(ctx context.Context, req model.SmartImportRequest, minRequired float64, content, hint, filename, mime string, sess *agentlog.Session) (model.SmartImportPreview, error) {
	profile := s.resolveContextProfile(ctx, &req)
	bg := workspacecontext.FormatForMultimodal(profile)
	draft, err := Analyze(content, filename, mime, hint, bg, req.UseLLM, s.llmFn(ctx, req, sess), req.Extensions)

	if err != nil {
		return model.SmartImportPreview{}, err
	}

	// For multimodal imports, the VLM-extracted content has no markdown headings
	// and the machine-extracted title rarely matches content literally.
	// Use the multimodal title hint (which IS the "## {title}" heading in the
	// merged content) so confidence checks on title and entity-trace pass.
	if isMultimodalImport(req.Extensions) {
		// For multimodal imports, Analyze.extractTitle often picks up
		// markdown table artifacts (pipe-separated headers) or VLM prose
		// fragments instead of the real document title. Use the multimodal
		// title hint which comes from InferConsolidatedTitle — it examines
		// the VLM text before markdown table consolidation.
		if hintTitle := importTitleHint(req.Extensions); hintTitle != "" {
			needsOverride := draft.Title == "" || !titleAppearsInContent(draft.Title, content)
			// Always override if Analyze extracted a table header
			// (e.g. "| 队伍编号 | 角色组合 |") or an invalid title.
			if !needsOverride && (strings.Contains(draft.Title, "|") || isInvalidTitle(draft.Title)) {
				needsOverride = true
			}
			if needsOverride {
				draft.Title = hintTitle
			}
		}
	}

	// Validate entities against taxonomy alias library (plan 2.2)
	if req.UseLLM && req.Extensions != nil && unifiedImportCacheHit(req.Extensions) {
		if m, ok := req.Extensions[extImportLLMUnified].(map[string]any); ok {
			if entities, ok := m["entities"].([]string); ok {
				validated, _ := ValidateEntityList(entities, content, hint, bg)
				m["entities"] = validated
				m["entities_validated"] = true
			}
		}
	}
	// Compute rule-based confidence label (optimization plan chapter 6)
	// For multimodal VLM-extracted content, pre-compute adapted signals:
	// 1. Entity density using content-derived terms (not hardcoded keyword list)
	// 2. Structure detection via semantic patterns (not markdown headings)
	sk := ScanSkeleton(content)
	ngramRate := NgramRepeatRate(content)
	entityTraceRatio := entityTraceRatioFromDraft(draft, content)
	cr := EvaluateConfidence(sk, content, draft.Title, entityTraceRatio, ngramRate)

	if isMultimodalImport(req.Extensions) && cr.Label == LabelReject {
		// Multimodal VLM/OCR content needs adapted quality assessment.
		// Standard EvaluateConfidence uses hardcoded keyword lists and
		// markdown heading detection — neither applies to VLM output.

		// Adapt 1: EntityOk via content-derived term discovery.
		// Instead of the default 25-verb keyword list, learn terms from
		// the VLM text itself: repeated CJK bigrams become domain terms.
		// "术语" ×4, "组合" ×6, "InternVL" ×2 → automatically recognized.
		if !cr.EntityOk {
			adaptedDensity := CalcInfoDensityWithTerms(content,
				NewContentDerivedTermProvider(content, nil))
			if adaptedDensity.Score >= 0.06 {
				cr.EntityOk = true
			}
		}

		// Adapt 2: StructureOk via semantic pattern detection.
		// VLM-extracted game tables, team compositions, etc. have no
		// markdown headings but are clearly structured (colon-separated
		// key:value lines, tabular repetition, list patterns).
		if !cr.StructureOk && hasSemanticStructure(content) {
			cr.StructureOk = true
		}

		// Adapt 3: EntityTraceOk for multimodal — if the title appears
		// in content AND we found content-derived entities, the trace is valid.
		if !cr.EntityTraceOk && cr.TitleOk && EntityCountFromContent(content) > 0 {
			cr.EntityTraceOk = true
		}

		// Adapt 4: NoiseOk for multimodal — VLM-extracted game guides,
		// tables, and structured content naturally repeat domain terms
		// and structural labels (e.g. team presets, stat tables).
		// High ngramRepeatRate does not indicate noise/OCR loops when
		// the content has semantic structure and recognized entities.
		if !cr.NoiseOk && cr.StructureOk && cr.EntityOk {
			cr.NoiseOk = true
		}
		// Recompute failed checks and label after patching
		var fails []string
		if !cr.StructureOk {
			fails = append(fails, "structure")
		}
		if !cr.EntityOk {
			fails = append(fails, "entity")
		}
		if !cr.NoiseOk {
			fails = append(fails, "noise")
		}
		if !cr.TitleOk {
			fails = append(fails, "title")
		}
		if !cr.EntityTraceOk {
			fails = append(fails, "entity_trace")
		}
		cr.FailedChecks = fails

		failCount := len(fails)
		switch {
		case cr.ShortExempt:
			// already set, preserve
		case failCount == 0:
			cr.Label = LabelPass
			cr.Explain = "all checks passed (multimodal adapted)"
		case failCount <= 2 && cr.NoiseOk:
			cr.Label = LabelWarning
			cr.Explain = "multimodal: " + strings.Join(fails, ", ") + " (non-critical, waived)"
		default:
			// Genuine reject — >2 failures or noise failure
		}
	}

	dedup, err := ResolveDedup(ctx, s.docs, draft, req.TargetDocID)
	if err != nil {
		return model.SmartImportPreview{}, err
	}

	importAllowed := cr.Label != LabelReject && dedup.Action != "unchanged"
	if dedup.Action == "blocked_similar" {
		importAllowed = false
	}

	kbType := draft.KBType
	if IsWorkspacePartition(strings.TrimSpace(hint)) {
		kbType = strings.TrimSpace(hint)
	}

	// LLM cleaning trigger control (optimization plan 8.2)
	// - density < 0.06 → Reject (handled by confidence label)
	// - density 0.06-0.10 → trigger LLM cleaning if available
	// - density > 0.10 → skip LLM, only markdown stripping + text normalization
	// Use taxonomy-aware domain term provider for density-based
	// LLM cleaning decisions (noise detection, marketing patterns).
	termProvider := NewDomainTermProvider(hint, bg)
	density := CalcInfoDensityWithTerms(content, termProvider)
	needsLLMCleaning := density.Score >= 0.06 && density.Score < 0.10
	hasNoiseSignals := ngramRate > 0.5 || HasURL(content) || HasMarketingPatterns(content)

	if needsLLMCleaning || (hasNoiseSignals && density.Score > 0.10) {
		// Mark that LLM noise cleaning should be applied before ingest.
		// The cleaning itself is deferred to Worker (gate check) to avoid
		// adding latency to the preview/analyze path.
		if req.Extensions == nil {
			req.Extensions = model.Extensions{}
		}
		req.Extensions["needs_llm_cleaning"] = true
		if needsLLMCleaning {
			req.Extensions["cleaning_reason"] = "low_density"
		} else if HasMarketingPatterns(content) {
			req.Extensions["cleaning_reason"] = "marketing_patterns"
		} else {
			req.Extensions["cleaning_reason"] = "noise_signals"
		}
	}

	// Log confidence check results
	if sess != nil {
		sess.Step("confidence_check", map[string]any{
			"label":              string(cr.Label),
			"structure_ok":       cr.StructureOk,
			"entity_ok":          cr.EntityOk,
			"noise_ok":           cr.NoiseOk,
			"title_ok":           cr.TitleOk,
			"entity_trace_ok":    cr.EntityTraceOk,
			"short_exempt":       cr.ShortExempt,
			"failed_checks":      cr.FailedChecks,
			"explain":            cr.Explain,
			"info_density":       density.Score,
			"ngram_repeat":       ngramRate,
			"needs_llm_cleaning": needsLLMCleaning || hasNoiseSignals,
			"content_head":       truncateRunes(content, 300),
			"title_used":         draft.Title,
			"is_multimodal":      isMultimodalImport(req.Extensions),
		})
	}

	return model.SmartImportPreview{
		KBType:          kbType,
		Title:           draft.Title,
		Content:         draft.Content,
		ContentHash:     draft.ContentHash,
		MimeType:        draft.MimeType,
		Action:          dedup.Action,
		ExistingDocID:   dedup.ExistingDocID,
		ExistingVersion: dedup.ExistingVersion,
		Confidence:      draft.Confidence,
		ImportAllowed:   importAllowed,
		Reason:          draft.Reason + "; " + dedup.Reason + " [" + string(cr.Label) + "] " + cr.Explain,
		Method:          draft.Method,
	}, nil
}

// entityTraceRatioFromDraft computes what fraction of draft entities can be traced to content.
func entityTraceRatioFromDraft(draft Draft, content string) float64 {
	// Entities are stored in extensions after unified LLM; for rule-only drafts, check title.
	if draft.Title != "" && strings.Contains(content, draft.Title) {
		return 1.0
	}
	entityCount := EntityCountFromContent(content)
	if entityCount == 0 {
		return 0
	}
	return 0.5 // conservative estimate for rule-based path
}

func (s *Service) submitPreview(ctx context.Context, req model.SmartImportRequest, preview model.SmartImportPreview, extensions model.Extensions) (*model.UGCNotifyResponse, error) {
	if !preview.ImportAllowed {
		return nil, errors.New("segment not allowed for import")
	}
	kbType := preview.KBType
	if hint := strings.TrimSpace(req.HintKBType); IsWorkspacePartition(hint) {
		kbType = hint
	}
	profile := s.resolveContextProfile(ctx, &req)
	extensions = attachWorkspaceContextExtensions(extensions, profile)
	if delta := mappingDeltaFromExtensions(extensions); len(delta.KeyPhrases) > 0 {
		taxonomyevolve.EnqueueMappingDelta(ctx, kbType, delta)
	}
	taxonomyevolve.EnqueueBlueprint(ctx, kbType, &profile, workspacecontext.FormatForWorkspaceContext(profile))

	notify := model.UGCNotifyRequest{
		TenantID:   req.TenantID,
		KBID:       req.KBID,
		KBType:     kbType,
		Title:      preview.Title,
		Content:    preview.Content,
		SourceURL:  req.SourceURL,
		MimeType:   preview.MimeType,
		GenerateQA: req.GenerateQA,
		Tags:       []string{"smart-import"},
		Extensions: extensions,
	}
	if preview.Action == "update" && preview.ExistingDocID != "" {
		notify.DocID = preview.ExistingDocID
		if strings.TrimSpace(req.TargetDocID) != "" {
			notify.Tags = append(notify.Tags, "agent-update-selected")
		} else {
			notify.Tags = append(notify.Tags, "version-update", "split-import")
		}
	} else {
		notify.Tags = append(notify.Tags, "split-import")
	}
	return s.ugc.Notify(ctx, notify)
}

func toModelDocAnalysis(a DocAnalysis) *model.SmartImportDocAnalysis {
	if a.ParentTitle == "" && a.Subject == "" && a.DocType == "" {
		return nil
	}
	return &model.SmartImportDocAnalysis{
		Subject:     a.Subject,
		Version:     a.Version,
		DocType:     a.DocType,
		ParentTitle: a.ParentTitle,
	}
}

func (s *Service) baseResp(minRequired float64, allowed, split bool, msg string) *model.SmartImportResponse {
	return &model.SmartImportResponse{
		Confidence:            0,
		MinConfidenceRequired: minRequired,
		ImportAllowed:         allowed,
		Split:                 split,
		Message:               msg,
		Items:                 []model.SmartImportItem{},
		Tasks:                 []model.UGCNotifyResponse{},
		Skipped:               []model.SmartImportSkipped{},
	}
}

// isMultimodalImport reports whether the request went through multimodal
// attachment processing (VLM/OCR extraction of images/videos).
// Uses the explicit multimodal_processed flag set unconditionally after
// VLM/OCR processing completes, not attachment filenames (which are often
// machine-generated and meaningless) or title hints (which may be absent).
func isMultimodalImport(ext model.Extensions) bool {
	if ext == nil {
		return false
	}
	v, ok := ext["multimodal_processed"].(bool)
	return ok && v
}

func isMultimodalConsolidated(ext model.Extensions) bool {
	if ext == nil {
		return false
	}
	v, ok := ext["multimodal_consolidated"].(bool)
	return ok && v
}

func anyImportable(items []model.SmartImportItem) bool {
	for _, it := range items {
		if it.ImportAllowed {
			return true
		}
	}
	return false
}

func countImportable(items []model.SmartImportItem) int {
	n := 0
	for _, it := range items {
		if it.ImportAllowed {
			n++
		}
	}
	return n
}

func importReadiness(submitted bool, resp *model.SmartImportResponse) *model.SmartImportReadiness {
	if resp == nil || !submitted {
		return &model.SmartImportReadiness{
			Queryable: false,
			Status:    "preview",
			Message:   "not submitted; no ingest task created",
		}
	}
	tasks := append([]model.UGCNotifyResponse(nil), resp.Tasks...)
	if resp.Task != nil && len(tasks) == 0 {
		tasks = append(tasks, *resp.Task)
	}
	if len(tasks) == 0 {
		return &model.SmartImportReadiness{
			Queryable: false,
			Status:    "no_task",
			Message:   "submitted request created no ingest task",
		}
	}
	ready := true
	var taskIDs, docIDs []string
	for _, t := range tasks {
		if t.TaskID != "" {
			taskIDs = append(taskIDs, t.TaskID)
		}
		if t.DocID != "" {
			docIDs = append(docIDs, t.DocID)
		}
		if t.Channel != "sync_fallback" || t.DocID == "" {
			ready = false
		}
	}
	if ready {
		return &model.SmartImportReadiness{
			Queryable: true,
			Status:    "ready",
			Message:   "ingested synchronously; content is queryable after cache purge/refresh",
			TaskIDs:   taskIDs,
			DocIDs:    docIDs,
		}
	}
	return &model.SmartImportReadiness{
		Queryable: false,
		Status:    "queued",
		Message:   "queued for async ingest; poll task status until completed and chunk_count > 0 before asking",
		TaskIDs:   taskIDs,
		DocIDs:    docIDs,
	}
}

func (s *Service) purgeCachesAfterSubmit(ctx context.Context, kbType string) {
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return
	}
	// Precision invalidation (optimization plan 7.1):
	// Phase 1: attempt targeted invalidation by KB type (existing behavior).
	// Phase 2: delayed full purge is deferred — the caller uses kb_type
	// which already provides partition-level isolation.
	if s.cacheMgr != nil {
		_, _ = s.cacheMgr.RedisPurgeByKBType(ctx, kbType)
	}
	if s.answerCache != nil {
		_, _ = s.answerCache.RedisPurgeByKBType(ctx, kbType)
	}
}

// purgeCachesByKeywords performs precision cache invalidation using document keywords.
// Extracts named entities and title words from the submitted content to target
// only relevant cache entries (optimization plan 10.1).
func (s *Service) purgeCachesByKeywords(ctx context.Context, kbType, title, content string) {
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return
	}
	// Extract keywords: named entities + title bigrams
	keywords := extractInvalidationKeywords(title, content)
	if len(keywords) == 0 {
		// Fall back to full kbType purge if no keywords extracted
		s.purgeCachesAfterSubmit(ctx, kbType)
		return
	}
	// Attempt targeted purge per keyword
	for _, kw := range keywords {
		if s.cacheMgr != nil {
			_, _ = s.cacheMgr.RedisPurgeByKBType(ctx, kbType) // current impl is kbType-level
		}
		_ = kw // future: use keyword-specific cache key patterns
	}
}

// extractInvalidationKeywords extracts entities and title ngrams for cache invalidation.
func extractInvalidationKeywords(title, content string) []string {
	var kw []string
	seen := map[string]bool{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" || seen[s] || len([]rune(s)) < 2 {
			return
		}
		seen[s] = true
		kw = append(kw, s)
	}
	// Title bigrams
	for _, t := range bigramTokens(title) {
		add(t)
	}
	// Named entities from content
	for _, m := range reNamedEnt.FindAllString(content, -1) {
		add(strings.TrimSpace(m))
	}
	if len(kw) > 10 {
		kw = kw[:10]
	}
	return kw
}

func importableSegmentTitles(items []model.SmartImportItem) []string {
	var titles []string
	for _, it := range items {
		if !it.ImportAllowed || it.Preview.Action == "unchanged" {
			continue
		}
		if t := strings.TrimSpace(it.Preview.Title); t != "" {
			titles = append(titles, t)
		}
	}
	return titles
}

func applyChildDerivedParent(analysis DocAnalysis, items []model.SmartImportItem) DocAnalysis {
	if childTitles := importableSegmentTitles(items); len(childTitles) > 1 {
		if pt := SummarizeParentTitleFromChildTitles(childTitles); pt != "" {
			analysis.ParentTitle = pt
		}
	}
	return analysis
}
