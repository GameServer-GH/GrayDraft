package chat

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/observability/trace"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/confidence"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/retriever"
	"github.com/graydraft/core/service/textutil"
	"github.com/graydraft/core/service/workspacecontext"
)

// CatalogMetaGetter loads kb catalog version for cache keys.
type CatalogMetaGetter interface {
	GetCatalogMeta(ctx context.Context, kbType string) (postgres.CatalogMeta, error)
}

// Service orchestrates RAG chat with streaming LLM output.
type Service struct {
	cfg         *config.Config
	retriever   *retriever.Hybrid
	llm         *llmclient.Client
	answerCache *answercache.Manager
	catalog     CatalogMetaGetter
	docs        ContextProfileGetter
}

// ContextProfileGetter loads stored workspace background by kb_type.
type ContextProfileGetter interface {
	GetContextProfile(ctx context.Context, kbType string) (model.ContextProfile, error)
}

// NewService creates chat service.
func NewService(
	cfg *config.Config,
	r *retriever.Hybrid,
	llm *llmclient.Client,
	ac *answercache.Manager,
	catalog CatalogMetaGetter,
	docs ContextProfileGetter,
) *Service {
	return &Service{cfg: cfg, retriever: r, llm: llm, answerCache: ac, catalog: catalog, docs: docs}
}

func (s *Service) resolveProfile(ctx context.Context, req model.ChatRequest) model.ContextProfile {
	stored := model.ContextProfile{KBType: req.KBType}
	if s.docs != nil && req.KBType != "" && (req.ContextProfile == nil || req.ContextProfile.IsEmpty()) {
		if p, err := s.docs.GetContextProfile(ctx, req.KBType); err == nil {
			stored = p
		}
	}
	return workspacecontext.MergeProfile(req.ContextProfile, stored, req.KBType)
}

func profilePtr(p model.ContextProfile) *model.ContextProfile {
	if p.IsEmpty() {
		return nil
	}
	out := p
	return &out
}

// StreamEventCallback receives SSE events for the handler.
type StreamEventCallback func(model.ChatStreamEvent) error

func chunkPreviews(chunks []model.Chunk) []model.RetrievalChunkPreview {
	if len(chunks) == 0 {
		return nil
	}
	previews := make([]model.RetrievalChunkPreview, 0, len(chunks))
	for i, c := range chunks {
		snippet := c.Content
		if len([]rune(snippet)) > 500 {
			r := []rune(snippet)
			snippet = string(r[:500]) + "…"
		}
		previews = append(previews, model.RetrievalChunkPreview{
			Rank:     i + 1,
			DocTitle: c.DocTitle,
			ChunkID:  c.ID,
			Score:    c.Score,
			Snippet:  snippet,
		})
	}
	return previews
}

// Stream runs retrieval then streams LLM tokens.
func (s *Service) Stream(ctx context.Context, req model.ChatRequest, emit StreamEventCallback) error {
	// V1-35: OpenTelemetry span
	sctx, chatSpan := trace.StartSpan(ctx, "chat.stream", trace.SpanAttrs(req.KBType, "")...)
	defer trace.RecordSpanSuccess(chatSpan)
	ctx = sctx
	mode := NormalizeChatMode(req.ChatMode)
	topK := req.TopK
	if topK == 0 {
		topK = s.cfg.Chat.MaxContextChunks
	}
	if topK == 0 {
		topK = s.cfg.Retrieval.DefaultTopK
	}

	// ---- answer cache lookup ----
	skipAnswer := req.SkipAnswerCache || req.SkipCache
	if !skipAnswer && s.answerCache != nil && s.answerCache.Enabled() && s.catalog != nil {
		meta, err := s.catalog.GetCatalogMeta(ctx, req.KBType)
		if err == nil {
			sem4dFP := ""
			if s.retriever != nil {
				sem4dFP = s.retriever.QueryCache4DFingerprint(ctx, req.Query)
			}
			params := answercache.LookupParams{
				Query:                 req.Query,
				KBType:                req.KBType,
				TopK:                  topK,
				CatalogVersion:        meta.Version,
				CatalogUpdated:        meta.LatestDocUpdatedAt,
				Semantic4DFingerprint: sem4dFP,
			}
			if entry, tier, ok := s.answerCache.Lookup(ctx, params); ok && IsAnswerCacheable(entry.Answer, entry.ChunksUsed) {
				return s.streamCachedAnswer(emit, entry, tier, mode)
			}
		}
	}
	// V1-40: skip answer cache for timeliness queries
	if !skipAnswer && confidence.IsTimelinessQuery(req.Query, req.KBType) {
		skipAnswer = true
	}

	// ---- apply chat budget (V1-23) ----
	if s.cfg.Chat.LLMTotalTimeoutMs > 0 {
		cctx, cancel := context.WithTimeout(ctx, time.Duration(s.cfg.Chat.LLMTotalTimeoutMs)*time.Millisecond)
		ctx = cctx
		defer cancel()
	}

	// ---- retrieval ----
	profile := s.resolveProfile(ctx, req)
	wcText := workspacecontext.ResolveWorkspaceContextText(profile, req.WorkspaceContext)
	var profilePtr *model.ContextProfile
	if !profile.IsEmpty() {
		profilePtr = &profile
	}

	searchReq := model.SearchRequest{
		Query:            req.Query,
		KBType:           req.KBType,
		KBTypes:          req.KBTypes,
		TopK:             topK,
		SkipCache:        req.SkipCache,
		WorkspaceContext: wcText,
		ContextProfile:   profilePtr,
	}

	searchResp, err := s.retriever.Search(ctx, searchReq)
	if err != nil {
		return emit(model.ChatStreamEvent{Type: "error", Message: err.Error(), Confidence: 0, ChatMode: mode})
	}

	chunks := searchResp.Chunks
	confScore := confidence.CalculateFinalConfidenceWithIntent(req.Query, searchResp.QueryIntent, chunks)
	confScore += workspacecontext.EntityContextBonus(req.Query, profile, chunks)
	if confScore > 1 {
		confScore = 1
	}
	if confScore == 0 && len(chunks) > 0 {
		confScore = confidence.FromRetrieval(chunks, searchResp.Total)
	}
	if s.cfg.Chat.MaxContextChunks > 0 && len(chunks) > s.cfg.Chat.MaxContextChunks {
		chunks = chunks[:s.cfg.Chat.MaxContextChunks]
	}

	// ---- grade evaluation (replaces binary gate) ----
	gd := EvaluateRetrievalGrade(chunks, searchResp.Total, req.Query, searchResp.QueryIntent, nil)
	grade := gd.Grade

	// Observability
	observability.RecordChatGate(observability.ChatGateRecord{
		KBType: req.KBType,
		Useful: grade.IsUseful(),
		Reason: gd.Reason,
	})
	if searchResp.RetrievalTrace != nil {
		searchResp.RetrievalTrace.RetrievalGrade = string(grade)
		observability.RecordChatWaterfall(observability.ChatWaterfallRecord{
			KBType: req.KBType,
			Tier:   searchResp.RetrievalTrace.WaterfallTier,
			Suffix: searchResp.RetrievalTrace.WaterfallSuffix,
		})
	}

	// ---- coverage evaluation ----
	// TODO: Wire CoverageLLM and CoverageNLIService when available.
	// For now, use the base coverage from GradeDecision.
	var missingTopics []string

	// ---- emit retrieval_grade event ----
	gradeLabel := map[confidence.RetrievalGrade]string{
		confidence.GradeA: "高可信",
		confidence.GradeB: "中可信",
		confidence.GradeC: "低可信",
	}[grade]
	if grade == confidence.GradeD {
		gradeLabel = "无参考"
	}

	chunkSummaries := buildChunkSummaries(chunks, grade)
	_ = emit(model.ChatStreamEvent{
		Type:           "retrieval_grade",
		RetrievalGrade: string(grade),
		CoverageScore:  gd.CoverageScore,
		GradeReason:    gd.Reason,
		MissingTopics:  missingTopics,
		ChunksSummary:  chunkSummaries,
		Message:        fmt.Sprintf("检索分级: %s (%s)", grade, gradeLabel),
	})

	// ---- emit thinking event ----
	_ = emit(model.ChatStreamEvent{
		Type:           "thinking",
		RetrievalGrade: string(grade),
		Message:        fmt.Sprintf("正在基于 %d 条%s参考整合回答...", len(chunks), gradeLabel),
	})

	// ---- build meta event ----
	metaEv := model.ChatStreamEvent{
		Type:                   "meta",
		AgentProfile:           req.AgentProfile,
		ChatMode:               mode,
		Strategy:               searchResp.Strategy,
		CacheHit:               searchResp.CacheHit,
		QueryIntent:            searchResp.QueryIntent,
		FulltextWeight:         searchResp.FulltextWeight,
		VectorWeight:           searchResp.VectorWeight,
		Total:                  searchResp.Total,
		ChunksUsed:             len(chunks),
		Confidence:             confScore,
		GateScore:              gd.Score,
		GateThreshold:          gd.Threshold,
		GateReason:             gd.Reason,
		Sources:                SourcesFromChunks(chunks),
		Citations:              CitationsFromChunks(chunks),
		QuerySemantic4D:        searchResp.QuerySemantic4D,
		Semantic4DFilter:       searchResp.Semantic4DFilter,
		RetrievalTrace:         searchResp.RetrievalTrace,
		RetrievalChunkPreviews: chunkPreviews(chunks),
		// v2 fields
		RetrievalGrade: string(grade),
		CoverageScore:  gd.CoverageScore,
		GradeReason:    gd.Reason,
		MissingTopics:  missingTopics,
		ChunksSummary:  chunkSummaries,
	}

	// ---- route by mode + grade ----
	switch {
	case mode == ChatModeKBOnly:
		metaEv.Message = fmt.Sprintf("kb_only: grade %s", grade)
		_ = emit(metaEv)
		return s.streamKBOnlyV2(ctx, req, topK, emit, chunks, searchResp, confScore, mode, grade, gd, profilePtr)
	case !grade.IsUseful() && grade == confidence.GradeD:
		// D: zero hits — general LLM with coverage warning
		metaEv.Message = "hybrid: no kb hits, using general llm"
		_ = emit(metaEv)
		return s.streamHybridGeneralV2(ctx, req, topK, emit, chunks, searchResp, confScore, mode, grade, gd, profilePtr)
	case !grade.IsUseful() && grade == confidence.GradeC:
		// C: low confidence — inject sparse context + general LLM
		metaEv.Message = "hybrid: low confidence kb, llm with sparse context"
		_ = emit(metaEv)
		return s.streamHybridWithGrade(ctx, req, topK, emit, chunks, searchResp, confScore, mode, grade, gd, profilePtr, missingTopics)
	default:
		// A/B: high/medium confidence — full RAG
		metaEv.Message = "hybrid: rag with kb context"
		_ = emit(metaEv)
		if !s.llmAvailable(req) {
			return s.streamKBOnlyV2(ctx, req, topK, emit, chunks, searchResp, confScore, mode, grade, gd, profilePtr)
		}
		return s.streamHybridWithGrade(ctx, req, topK, emit, chunks, searchResp, confScore, mode, grade, gd, profilePtr, missingTopics)
	}
}

// streamKBOnlyV2 handles kb_only mode with grade awareness.
func (s *Service) streamKBOnlyV2(
	ctx context.Context,
	req model.ChatRequest,
	topK int,
	emit StreamEventCallback,
	chunks []model.Chunk,
	searchResp *model.SearchResponse,
	confScore float64,
	mode string,
	grade confidence.RetrievalGrade,
	gd confidence.GradeDecision,
	profile *model.ContextProfile,
) error {
	strategy := searchResp.Strategy

	switch grade {
	case confidence.GradeD:
		// Zero hits: coverage boundary with entity list
		answer := KBCoverageBoundary(nil)
		strategy += "+kb_only_miss"
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: 0})
		s.storeAnswerV2(ctx, req, topK, mode, grade, answercache.Entry{
			Answer:         answer,
			Strategy:       strategy,
			ChunksUsed:     0,
			QueryIntent:    searchResp.QueryIntent,
			FulltextWeight: searchResp.FulltextWeight,
			VectorWeight:   searchResp.VectorWeight,
			RetrievalHit:   searchResp.CacheHit,
		})
		return emit(s.doneEventV2(mode, strategy, 0, answer, nil, searchResp, grade, gd))

	case confidence.GradeC:
		// Sparse hits: three-part output
		strategy += "+kb_only_sparse"
		if s.llmAvailable(req) {
			messages := BuildUnifiedMessages(req.Query, req.KBType, chunks, grade, profile, ChatModeKBOnly, gd.MissingTopics)
			answer := s.streamAndCollect(ctx, req, messages, llmclient.TierFast, emit, confScore)
			answer = textutil.NormalizeKBMarkdown(answer)
			if answer == "" {
				answer = KBAnswer(chunks)
				strategy += "+raw_fallback"
			}
			s.storeAnswerV2(ctx, req, topK, mode, grade, answercache.Entry{
				Answer:         answer,
				Strategy:       strategy,
				ChunksUsed:     len(chunks),
				QueryIntent:    searchResp.QueryIntent,
				FulltextWeight: searchResp.FulltextWeight,
				VectorWeight:   searchResp.VectorWeight,
				RetrievalHit:   searchResp.CacheHit,
			})
			return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
		}
		answer := KBAnswer(chunks)
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: confScore})
		return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))

	case confidence.GradeB:
		// Partial hits
		strategy += "+kb_only_partial"
		if s.llmAvailable(req) {
			messages := BuildUnifiedMessages(req.Query, req.KBType, chunks, grade, profile, ChatModeKBOnly, gd.MissingTopics)
			answer := s.streamAndCollect(ctx, req, messages, llmclient.TierFast, emit, confScore)
			answer = textutil.NormalizeKBMarkdown(answer)
			if answer == "" {
				answer = KBAnswer(chunks)
				strategy += "+raw_fallback"
			}
			s.storeAnswerV2(ctx, req, topK, mode, grade, answercache.Entry{
				Answer:         answer,
				Strategy:       strategy,
				ChunksUsed:     len(chunks),
				QueryIntent:    searchResp.QueryIntent,
				FulltextWeight: searchResp.FulltextWeight,
				VectorWeight:   searchResp.VectorWeight,
				RetrievalHit:   searchResp.CacheHit,
			})
			return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
		}
		answer := KBAnswer(chunks)
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: confScore})
		return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))

	default:
		// GradeA: full integration
		strategy += "+kb_only_llm"
		if s.llmAvailable(req) {
			messages := BuildUnifiedMessages(req.Query, req.KBType, chunks, grade, profile, ChatModeKBOnly, gd.MissingTopics)
			answer := s.streamAndCollect(ctx, req, messages, llmclient.TierFast, emit, confScore)
			answer = textutil.NormalizeKBMarkdown(answer)
			if answer == "" {
				if s.cfg.Chat.SearchOnlyFallback {
					answer = KBAnswer(chunks)
					strategy += "+raw_fallback"
				} else {
					return emit(model.ChatStreamEvent{Type: "error", Message: "kb_only llm returned empty answer", Confidence: confScore, ChatMode: mode})
				}
			}
			s.storeAnswerV2(ctx, req, topK, mode, grade, answercache.Entry{
				Answer:         answer,
				Strategy:       strategy,
				ChunksUsed:     len(chunks),
				QueryIntent:    searchResp.QueryIntent,
				FulltextWeight: searchResp.FulltextWeight,
				VectorWeight:   searchResp.VectorWeight,
				RetrievalHit:   searchResp.CacheHit,
			})
			return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
		}
		answer := KBAnswer(chunks)
		strategy += "+kb_only"
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: confScore})
		return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
	}
}

// streamHybridWithGrade handles hybrid mode with grade-based context injection.
func (s *Service) streamHybridWithGrade(
	ctx context.Context,
	req model.ChatRequest,
	topK int,
	emit StreamEventCallback,
	chunks []model.Chunk,
	searchResp *model.SearchResponse,
	confScore float64,
	mode string,
	grade confidence.RetrievalGrade,
	gd confidence.GradeDecision,
	profile *model.ContextProfile,
	missingTopics []string,
) error {
	if !s.llmAvailable(req) {
		answer := "知识库中未检索到足够相关内容，且当前未配置 LLM，无法继续回答。"
		strategy := searchResp.Strategy + "+hybrid_no_llm"
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: confScore})
		return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
	}

	messages := BuildUnifiedMessages(req.Query, req.KBType, chunks, grade, profile, mode, missingTopics)
	answer := s.streamAndCollect(ctx, req, messages, llmclient.TierComplex, emit, confScore)

	strategy := searchResp.Strategy + "+hybrid_rag"
	answerStr := textutil.NormalizeKBMarkdown(answer)

	if answerStr == "" {
		observability.RecordChatLLMEmpty(req.KBType, mode)
	}

	// Generate sentence labels (placeholder — full NLI integration is TODO)
	sentenceLabels := labelSentences(answerStr, chunks)

	// Detect contradictions
	conflicts := confidence.DetectContradictions(chunks, nil)
	warnings := confidence.FormatContradictionWarnings(conflicts)

	// Store answer cache
	s.storeAnswerV2(ctx, req, topK, mode, grade, answercache.Entry{
		Answer:         answerStr,
		Strategy:       strategy,
		ChunksUsed:     len(chunks),
		QueryIntent:    searchResp.QueryIntent,
		FulltextWeight: searchResp.FulltextWeight,
		VectorWeight:   searchResp.VectorWeight,
		RetrievalHit:   searchResp.CacheHit,
	})

	// Build done event with extended fields
	doneEv := s.doneEventV2Extended(mode, strategy, confScore, answerStr, chunks, searchResp, grade, gd)
	doneEv.SentenceLabels = sentenceLabels
	doneEv.Warnings = warnings

	return emit(doneEv)
}

// streamHybridGeneralV2 handles zero-hit hybrid mode (Grade D).
func (s *Service) streamHybridGeneralV2(
	ctx context.Context,
	req model.ChatRequest,
	topK int,
	emit StreamEventCallback,
	chunks []model.Chunk,
	searchResp *model.SearchResponse,
	confScore float64,
	mode string,
	grade confidence.RetrievalGrade,
	gd confidence.GradeDecision,
	profile *model.ContextProfile,
) error {
	if !s.llmAvailable(req) {
		answer := "知识库中未检索到足够相关内容，且当前未配置 LLM，无法继续回答。"
		strategy := searchResp.Strategy + "+hybrid_no_llm"
		_ = emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(answer), Confidence: confScore})
		return emit(s.doneEventV2(mode, strategy, confScore, answer, chunks, searchResp, grade, gd))
	}

	// Even for Grade D, inject the "no KB" signal via unified prompt
	messages := BuildUnifiedMessages(req.Query, req.KBType, nil, grade, profile, mode, gd.MissingTopics)
	answer := s.streamAndCollect(ctx, req, messages, llmclient.TierComplex, emit, confScore)

	strategy := searchResp.Strategy + "+hybrid_general_llm"
	answerStr := textutil.NormalizeKBMarkdown(answer)

	if answerStr == "" {
		observability.RecordChatLLMEmpty(req.KBType, mode)
	}
	// No answer cache for Grade D (generic knowledge may change)
	return emit(s.doneEventV2(mode, strategy, confScore, answerStr, chunks, searchResp, grade, gd))
}

// streamAndCollect streams LLM tokens via emit and returns the collected answer.
func (s *Service) streamAndCollect(
	ctx context.Context,
	req model.ChatRequest,
	messages []llmclient.Message,
	tier llmclient.Tier,
	emit StreamEventCallback,
	confScore float64,
) string {
	var answer strings.Builder
	firstToken := true
	start := time.Now()
	streamErr := s.streamLLMWithTier(ctx, req, messages, tier, func(delta string) error {
		if firstToken {
			firstToken = false
			firstLat := time.Since(start).Seconds()
			mode := NormalizeChatMode(req.ChatMode)
			observability.RecordChatFirstToken(req.KBType, mode, firstLat)
		}
		clean := textutil.StripEmojis(delta)
		answer.WriteString(clean)
		if clean == "" {
			return nil
		}
		return emit(model.ChatStreamEvent{Type: "chunk", Delta: clean})
	})
	if streamErr != nil {
		observability.RecordChatFallback(req.KBType, "llm_stream_error")
		return ""
	}
	return answer.String()
}

// doneEventV2 builds the standard done event.
func (s *Service) doneEventV2(
	mode, strategy string,
	confScore float64,
	answer string,
	chunks []model.Chunk,
	searchResp *model.SearchResponse,
	grade confidence.RetrievalGrade,
	gd confidence.GradeDecision,
) model.ChatStreamEvent {
	return s.doneEventV2Extended(mode, strategy, confScore, answer, chunks, searchResp, grade, gd)
}

// doneEventV2Extended builds done event with answer confidence and grade fields.
func (s *Service) doneEventV2Extended(
	mode, strategy string,
	confScore float64,
	answer string,
	chunks []model.Chunk,
	searchResp *model.SearchResponse,
	grade confidence.RetrievalGrade,
	gd confidence.GradeDecision,
) model.ChatStreamEvent {
	ev := model.ChatStreamEvent{
		Type:                   "done",
		Done:                   true,
		Strategy:               strategy,
		Confidence:             confScore,
		ChatMode:               mode,
		Sources:                SourcesFromChunks(chunks),
		Citations:              CitationsFromChunks(chunks),
		ChunksUsed:             len(chunks),
		RetrievalChunkPreviews: chunkPreviews(chunks),
		RetrievalGrade:         string(grade),
		CoverageScore:          gd.CoverageScore,
		GradeReason:            gd.Reason,
		MissingTopics:          gd.MissingTopics,
	}
	if s.cfg != nil && s.cfg.Chat.AnswerSelfCheck && strings.TrimSpace(answer) != "" {
		ctxTexts := make([]string, 0, len(chunks))
		for _, c := range chunks {
			ctxTexts = append(ctxTexts, c.Content)
		}
		ac := confidence.EvaluateAnswerConfidence(answer, ctxTexts, confScore, grade, ev.SentenceLabels)
		ev.AnswerConfidence = ac.Score
		if detail, err := json.Marshal(ac); err == nil {
			ev.AnswerConfidenceDetail = detail
		}
	}
	return ev
}

func (s *Service) streamCachedAnswer(emit StreamEventCallback, entry *answercache.Entry, tier, mode string) error {
	strategy := entry.Strategy
	if strategy == "" {
		strategy = "answer_cache"
	} else {
		strategy += "+answer_cache"
	}
	cachedConf := 0.72
	if entry.ChunksUsed > 0 {
		cachedConf = 0.78
	}
	_ = emit(model.ChatStreamEvent{
		Type:           "meta",
		AgentProfile:   EduWorkspaceProfile,
		Strategy:       strategy,
		AnswerCacheHit: tier,
		CacheHit:       entry.RetrievalHit,
		QueryIntent:    entry.QueryIntent,
		FulltextWeight: entry.FulltextWeight,
		VectorWeight:   entry.VectorWeight,
		ChunksUsed:     entry.ChunksUsed,
		Confidence:     cachedConf,
		ChatMode:       mode,
		Message:        "answer cache replay",
	})
	if err := emit(model.ChatStreamEvent{Type: "chunk", Delta: textutil.StripEmojis(entry.Answer)}); err != nil {
		return err
	}
	return emit(model.ChatStreamEvent{Type: "done", Done: true, Strategy: strategy, AnswerCacheHit: tier, Confidence: cachedConf, ChatMode: mode})
}

// storeAnswerV2 writes answer cache with grade awareness.
func (s *Service) storeAnswerV2(
	ctx context.Context,
	req model.ChatRequest,
	topK int,
	mode string,
	grade confidence.RetrievalGrade,
	entry answercache.Entry,
) {
	// kb_only mode now caches for A/B/C grades
	if req.SkipAnswerCache || req.SkipCache {
		return
	}

	// Grade D never caches (generic knowledge may change)
	if grade == confidence.GradeD {
		return
	}

	if s.answerCache == nil || !s.answerCache.Enabled() || s.catalog == nil || entry.Answer == "" {
		return
	}
	if !IsAnswerCacheable(entry.Answer, entry.ChunksUsed) {
		return
	}
	meta, err := s.catalog.GetCatalogMeta(ctx, req.KBType)
	if err != nil {
		return
	}
	sem4dFP := ""
	if s.retriever != nil {
		sem4dFP = s.retriever.QueryCache4DFingerprint(ctx, req.Query)
	}
	params := answercache.LookupParams{
		Query:                 req.Query,
		KBType:                req.KBType,
		TopK:                  topK,
		CatalogVersion:        meta.Version,
		CatalogUpdated:        meta.LatestDocUpdatedAt,
		Semantic4DFingerprint: sem4dFP,
		Grade:                 string(grade),
	}

	switch {
	case grade == confidence.GradeC:
		s.answerCache.StoreHalfTTL(ctx, params, &entry)
	case grade == confidence.GradeA:
		s.answerCache.Store(ctx, params, &entry)
		s.answerCache.EvictLowerGrades(ctx, params)
	default: // GradeB
		s.answerCache.Store(ctx, params, &entry)
	}
}

// buildChunkSummaries creates per-chunk grade summaries.
func buildChunkSummaries(chunks []model.Chunk, grade confidence.RetrievalGrade) []model.ChunkSummary {
	if len(chunks) == 0 {
		return nil
	}
	gradeLabel := map[confidence.RetrievalGrade]string{
		confidence.GradeA: "高可信",
		confidence.GradeB: "中可信",
		confidence.GradeC: "低可信",
	}[grade]
	if grade == confidence.GradeD {
		gradeLabel = "无参考"
	}

	summaries := make([]model.ChunkSummary, 0, len(chunks))
	for _, c := range chunks {
		summaries = append(summaries, model.ChunkSummary{
			ChunkID:    c.ID,
			DocTitle:   c.DocTitle,
			GradeLabel: gradeLabel,
		})
	}
	return summaries
}

// labelSentences performs basic sentence splitting and labels sentences as "kb" or "supplement".
// TODO: Replace with NLI-based sentence labeling when CoverageNLIService is available.
func labelSentences(answer string, chunks []model.Chunk) []model.SentenceLabel {
	if answer == "" || len(chunks) == 0 {
		return nil
	}

	// Build premise from all chunks
	var premise []string
	for _, c := range chunks {
		if c.Content != "" {
			premise = append(premise, c.Content)
		}
	}
	if len(premise) == 0 {
		return nil
	}
	ctx := strings.ToLower(strings.Join(premise, " "))

	// Split answer into sentences
	sentences := splitSentences(answer)
	if len(sentences) <= 1 {
		// Single sentence: check if grounded in context
		if confidence.FromAnswer(answer, premise, 0.5) > 0.3 {
			return []model.SentenceLabel{{Sentence: answer, Source: "kb"}}
		}
		return []model.SentenceLabel{{Sentence: answer, Source: "supplement"}}
	}

	labels := make([]model.SentenceLabel, 0, len(sentences))
	for _, s := range sentences {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		label := "supplement"
		if sentenceGroundedInContext(s, ctx) {
			label = "kb"
		}
		labels = append(labels, model.SentenceLabel{
			Sentence: s,
			Source:   label,
		})
	}
	return labels
}

func sentenceGroundedInContext(sentence, ctx string) bool {
	terms := confidence.QueryTerms(sentence)
	hit := 0
	for _, t := range terms {
		if strings.Contains(ctx, strings.ToLower(t)) {
			hit++
		}
	}
	if len(terms) == 0 {
		return false
	}
	return float64(hit)/float64(len(terms)) >= 0.3
}

func splitSentences(text string) []string {
	var sentences []string
	current := strings.Builder{}
	for _, r := range text {
		current.WriteRune(r)
		if r == '。' || r == '！' || r == '？' || r == '\n' || r == '?' || r == '!' {
			s := strings.TrimSpace(current.String())
			if s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if current.Len() > 0 {
		s := strings.TrimSpace(current.String())
		if s != "" {
			sentences = append(sentences, s)
		}
	}
	return sentences
}

// ValidateRequest checks required fields.
func ValidateRequest(req model.ChatRequest) error {
	if req.Query == "" {
		return fmt.Errorf("query is required")
	}
	if req.ChatMode != "" && req.ChatMode != ChatModeKBOnly && req.ChatMode != ChatModeHybrid {
		return fmt.Errorf("invalid chat_mode: use kb_only or hybrid")
	}
	return nil
}
