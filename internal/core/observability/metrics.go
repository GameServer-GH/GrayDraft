package observability

import (
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

const namespace = "graydraft"

var (
	enabled bool

	httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "http_requests_total",
			Help:      "HTTP requests processed by Core.",
		},
		[]string{"method", "route", "code"},
	)
	httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "http_request_duration_seconds",
			Help:      "HTTP request latency in seconds.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{"method", "route"},
	)

	retrievalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retrieval_requests_total",
			Help:      "Retrieval search operations.",
		},
		[]string{"kb_type", "cache_hit", "query_intent", "status"},
	)
	retrievalDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "retrieval_duration_seconds",
			Help:      "Retrieval search latency in seconds.",
			Buckets:   []float64{0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2, 5},
		},
		[]string{"kb_type"},
	)
	retrievalChunks = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "retrieval_chunks_returned",
			Help:      "Number of chunks returned per retrieval.",
			Buckets:   []float64{0, 1, 2, 3, 5, 10, 15, 20, 30, 50},
		},
		[]string{"kb_type"},
	)
	retrievalStrategy = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retrieval_strategy_total",
			Help:      "Retrieval strategy prefix (first segment before '+').",
		},
		[]string{"strategy_prefix"},
	)

	chatRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_requests_total",
			Help:      "RAG chat stream sessions.",
		},
		[]string{"kb_type", "answer_cache_hit", "retrieval_cache_hit", "query_intent", "status"},
	)
	chatDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_duration_seconds",
			Help:      "End-to-end chat stream latency in seconds.",
			Buckets:   []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60, 120},
		},
		[]string{"kb_type", "answer_cache_hit"},
	)
	chatRetrievalGate = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_retrieval_gate_total",
			Help:      "Chat retrieval gate decisions.",
		},
		[]string{"kb_type", "useful", "reason"},
	)
	chatWaterfall = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_waterfall_total",
			Help:      "Chat retrieval waterfall tiers.",
		},
		[]string{"kb_type", "tier", "suffix"},
	)
	chatLLMEmpty = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_llm_empty_total",
			Help:      "Chat LLM returned an empty answer.",
		},
		[]string{"kb_type", "chat_mode"},
	)
	ugcTaskIndexLag = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "ugc_task_index_lag_seconds",
			Help:      "Lag from UGC task creation to queryable task status.",
			Buckets:   []float64{1, 2, 5, 10, 30, 60, 120, 300, 600},
		},
		[]string{"kb_type", "readiness"},
	)

	ugcNotifyTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "ugc_notify_total",
			Help:      "UGC notify dispatches.",
		},
		[]string{"channel", "status"},
	)

	backfillOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "backfill_operations_total",
			Help:      "Backfill and index rebuild operations by type and status.",
		},
		[]string{"operation", "status"},
	)
	backfillDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "backfill_duration_seconds",
			Help:      "Backfill operation duration in seconds.",
			Buckets:   []float64{1, 5, 10, 30, 60, 120, 300, 600, 1800},
		},
		[]string{"operation"},
	)

	dlqMessages = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Namespace: namespace,
			Name:      "dlq_messages",
			Help:      "Current message count in JetStream DLQ stream (GRAYDRAFT_UGC_DLQ).",
		},
	)

	// V1-32 / V1-34: phase-aware performance metrics
	chatFirstTokenLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "chat_first_token_latency_seconds",
			Help:      "Time from chat request start to first LLM token.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 1.5, 2, 3, 5, 10, 30},
		},
		[]string{"kb_type", "mode"},
	)
	chatFallbackTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "chat_fallback_total",
			Help:      "Chat degradation / fallback counts by reason.",
		},
		[]string{"kb_type", "reason"},
	)
	retrievalCacheHitRatio = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "retrieval_cache_lookup_total",
			Help:      "Retrieval cache lookups by hit level.",
		},
		[]string{"kb_type", "level"},
	)
	answerCacheHitRatio = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "answer_cache_lookup_total",
			Help:      "Answer cache lookups by hit type.",
		},
		[]string{"kb_type", "hit"},
	)
	embeddingLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "embedding_latency_seconds",
			Help:      "Embedding API latency including retry.",
			Buckets:   []float64{0.05, 0.1, 0.2, 0.5, 1, 2, 5, 10},
		},
		[]string{"provider", "model"},
	)
	rerankLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "rerank_latency_seconds",
			Help:      "Rerank API latency.",
			Buckets:   []float64{0.1, 0.2, 0.5, 1, 2, 5},
		},
		[]string{"provider", "model"},
	)
	llmLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: namespace,
			Name:      "llm_latency_seconds",
			Help:      "LLM first-token plus total latency.",
			Buckets:   []float64{0.1, 0.25, 0.5, 1, 1.5, 2, 3, 5, 10, 30, 60},
		},
		[]string{"provider", "model", "phase"},
	)

	// V1-37: cache pollution monitoring
	cacheStaleHitTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_stale_hit_total",
			Help:      "Stale cache hits where cached index_version differs from current.",
		},
		[]string{"workspace", "kb_type", "cache_type"},
	)
	cacheInvalidationTrace = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "cache_invalidation_events_total",
			Help:      "Cache invalidation events by entity count and outcome.",
		},
		[]string{"kb_type", "outcome"},
	)
	modelTokenUsage = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: namespace,
			Name:      "model_token_usage_total",
			Help:      "Token consumption by workspace, provider, model.",
		},
		[]string{"workspace", "provider", "model"},
	)
	agentAnswerGroundedness = prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: "agent_answer_groundedness", Help: "Groundedness score of Agent answers.", Buckets: []float64{0, .25, .5, .62, .75, .9, 1}}, []string{"decision"})
	agentToolCalls          = prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "agent_tool_calls_total", Help: "Agent tool calls by outcome."}, []string{"tool", "status"})
	agentRuns               = prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "agent_runs_total", Help: "Agent runs by terminal status."}, []string{"status"})
	agentRunDuration        = prometheus.NewHistogramVec(prometheus.HistogramOpts{Namespace: namespace, Name: "agent_run_duration_seconds", Help: "End-to-end Agent run duration.", Buckets: []float64{.1, .25, .5, 1, 2, 3, 5, 10, 30, 60}}, []string{"status"})
	agentRunSteps           = prometheus.NewHistogram(prometheus.HistogramOpts{Namespace: namespace, Name: "agent_run_steps", Help: "Loop steps per Agent run.", Buckets: []float64{1, 2, 3, 4, 5, 6, 8, 10}})
	agentCanary             = prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "agent_canary_total", Help: "Agent candidate policy outcomes and circuit breaker actions."}, []string{"outcome"})
	costBudgetExceeded      = prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: namespace, Name: "cost_budget_exceeded_total", Help: "Daily token budget exceeded events."}, []string{"tenant_id", "operation"})
)

// Init registers Prometheus collectors when metrics are enabled.
func Init(metricsEnabled bool) {
	if !metricsEnabled || enabled {
		return
	}
	prometheus.MustRegister(
		httpRequests,
		httpDuration,
		retrievalRequests,
		retrievalDuration,
		retrievalChunks,
		retrievalStrategy,
		chatRequests,
		chatDuration,
		chatRetrievalGate,
		chatWaterfall,
		chatLLMEmpty,
		ugcTaskIndexLag,
		ugcNotifyTotal,
		backfillOperations,
		backfillDuration,
		dlqMessages,
		chatFirstTokenLatency,
		chatFallbackTotal,
		retrievalCacheHitRatio,
		answerCacheHitRatio,
		embeddingLatency,
		rerankLatency,
		llmLatency,
		modelTokenUsage,
		cacheStaleHitTotal,
		cacheInvalidationTrace,
		agentAnswerGroundedness,
		agentToolCalls,
		agentRuns,
		agentRunDuration,
		agentRunSteps,
		agentCanary,
		costBudgetExceeded,
	)
	enabled = true
}
func RecordAgentCanary(outcome string) {
	if enabled {
		agentCanary.WithLabelValues(labelOr(outcome, "unknown")).Inc()
	}
}

func RecordAgentAnswerGroundedness(score float64, rejected bool) {
	if !enabled {
		return
	}
	decision := "accepted"
	if rejected {
		decision = "rejected"
	}
	agentAnswerGroundedness.WithLabelValues(decision).Observe(score)
}

func RecordAgentToolCall(tool, status string) {
	if !enabled {
		return
	}
	agentToolCalls.WithLabelValues(labelOr(tool, "unknown"), labelOr(status, "unknown")).Inc()
}
func RecordAgentRun(status string, duration time.Duration, steps, toolCalls int) {
	if !enabled {
		return
	}
	status = labelOr(status, "unknown")
	agentRuns.WithLabelValues(status).Inc()
	agentRunDuration.WithLabelValues(status).Observe(duration.Seconds())
	agentRunSteps.Observe(float64(steps))
}

// SetDLQMessages updates the DLQ backlog gauge (O-03 alert source).
func SetDLQMessages(n float64) {
	if !enabled {
		return
	}
	dlqMessages.Set(n)
}

// Enabled reports whether RAG metrics are registered.
func Enabled() bool {
	return enabled
}

// RecordHTTP increments HTTP metrics.
func RecordHTTP(method, route, code string, duration time.Duration) {
	if !enabled {
		return
	}
	httpRequests.WithLabelValues(method, route, code).Inc()
	httpDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

// RetrievalRecord is one search observation.
type RetrievalRecord struct {
	KBType      string
	CacheHit    string
	QueryIntent string
	Strategy    string
	Chunks      int
	Duration    time.Duration
	Err         error
}

// RecordRetrieval records retrieval RAG metrics and returns fields for structured logs.
func RecordRetrieval(r RetrievalRecord) map[string]any {
	kb := labelKB(r.KBType)
	hit := labelOr(r.CacheHit, "none")
	intent := labelOr(r.QueryIntent, "unknown")
	status := "ok"
	if r.Err != nil {
		status = "error"
	}
	if enabled {
		retrievalRequests.WithLabelValues(kb, hit, intent, status).Inc()
		retrievalDuration.WithLabelValues(kb).Observe(r.Duration.Seconds())
		if r.Err == nil {
			retrievalChunks.WithLabelValues(kb).Observe(float64(r.Chunks))
			retrievalStrategy.WithLabelValues(strategyPrefix(r.Strategy)).Inc()
		}
	}
	return map[string]any{
		"kb_type": kb, "cache_hit": hit, "query_intent": intent,
		"strategy": r.Strategy, "chunks": r.Chunks,
		"duration_ms": r.Duration.Milliseconds(), "status": status,
	}
}

// ChatRecord is one chat stream observation.
type ChatRecord struct {
	KBType            string
	AnswerCacheHit    string
	RetrievalCacheHit string
	QueryIntent       string
	Strategy          string
	ChunksUsed        int
	Duration          time.Duration
	Err               error
}

// ChatGateRecord is one retrieval usefulness decision inside chat.
type ChatGateRecord struct {
	KBType string
	Useful bool
	Reason string
}

// ChatWaterfallRecord is the 4D waterfall tier used by chat retrieval.
type ChatWaterfallRecord struct {
	KBType string
	Tier   int
	Suffix string
}

// RecordChat records chat RAG metrics.
func RecordChat(r ChatRecord) map[string]any {
	kb := labelKB(r.KBType)
	ansHit := labelOr(r.AnswerCacheHit, "none")
	retHit := labelOr(r.RetrievalCacheHit, "none")
	intent := labelOr(r.QueryIntent, "unknown")
	status := "ok"
	if r.Err != nil {
		status = "error"
	}
	if enabled {
		chatRequests.WithLabelValues(kb, ansHit, retHit, intent, status).Inc()
		chatDuration.WithLabelValues(kb, ansHit).Observe(r.Duration.Seconds())
	}
	return map[string]any{
		"kb_type": kb, "answer_cache_hit": ansHit, "retrieval_cache_hit": retHit,
		"query_intent": intent, "strategy": r.Strategy, "chunks_used": r.ChunksUsed,
		"duration_ms": r.Duration.Milliseconds(), "status": status,
	}
}

// RecordChatGate records useful gate decisions, including no-useful cases.
func RecordChatGate(r ChatGateRecord) {
	if !enabled {
		return
	}
	useful := "false"
	if r.Useful {
		useful = "true"
	}
	chatRetrievalGate.WithLabelValues(labelKB(r.KBType), useful, labelOr(r.Reason, "unknown")).Inc()
}

// RecordChatWaterfall records waterfall tiers, including wf_soft fallback.
func RecordChatWaterfall(r ChatWaterfallRecord) {
	if !enabled || r.Tier <= 0 {
		return
	}
	chatWaterfall.WithLabelValues(labelKB(r.KBType), labelOr(fmtInt(r.Tier), "0"), labelOr(r.Suffix, "strict")).Inc()
}

// RecordChatLLMEmpty records empty LLM outputs.
func RecordChatLLMEmpty(kbType, chatMode string) {
	if !enabled {
		return
	}
	chatLLMEmpty.WithLabelValues(labelKB(kbType), labelOr(chatMode, "unknown")).Inc()
}

// RecordUGCTaskIndexLag records how long an ingest task took to become queryable.
func RecordUGCTaskIndexLag(kbType, readiness string, createdAt any, now time.Time) {
	if !enabled {
		return
	}
	if now.IsZero() {
		now = time.Now()
	}
	created, ok := createdAt.(time.Time)
	if !ok || created.IsZero() || now.Before(created) {
		return
	}
	ugcTaskIndexLag.WithLabelValues(labelKB(kbType), labelOr(readiness, "unknown")).Observe(now.Sub(created).Seconds())
}

// RecordUGCNotify records UGC dispatch metrics.
func RecordUGCNotify(channel string, err error) {
	if !enabled {
		return
	}
	status := "ok"
	if err != nil {
		status = "error"
	}
	ugcNotifyTotal.WithLabelValues(labelOr(channel, "unknown"), status).Inc()
}

// RecordBackfill records a backfill/rebuild operation.
func RecordBackfill(operation, status string, duration time.Duration) {
	if !enabled {
		return
	}
	backfillOperations.WithLabelValues(labelOr(operation, "unknown"), labelOr(status, "ok")).Inc()
	if duration > 0 {
		backfillDuration.WithLabelValues(labelOr(operation, "unknown")).Observe(duration.Seconds())
	}
}

func fmtInt(n int) string {
	if n == 0 {
		return "0"
	}
	return strconv.Itoa(n)
}

// LogRetrieval emits a structured RAG retrieval log line.
func LogRetrieval(r RetrievalRecord) {
	f := RecordRetrieval(r)
	slog.Info("rag.retrieval",
		"kb_type", f["kb_type"],
		"cache_hit", f["cache_hit"],
		"query_intent", f["query_intent"],
		"strategy", f["strategy"],
		"chunks", f["chunks"],
		"duration_ms", f["duration_ms"],
		"status", f["status"],
	)
}

// LogChat emits a structured RAG chat log line.
func LogChat(r ChatRecord) {
	f := RecordChat(r)
	slog.Info("rag.chat",
		"kb_type", f["kb_type"],
		"answer_cache_hit", f["answer_cache_hit"],
		"retrieval_cache_hit", f["retrieval_cache_hit"],
		"query_intent", f["query_intent"],
		"strategy", f["strategy"],
		"chunks_used", f["chunks_used"],
		"duration_ms", f["duration_ms"],
		"status", f["status"],
	)
}

func labelKB(kb string) string {
	if strings.TrimSpace(kb) == "" {
		return "_all"
	}
	return strings.TrimSpace(kb)
}

func labelOr(v, def string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return def
	}
	if len(v) > 32 {
		return v[:32]
	}
	return v
}

func strategyPrefix(strategy string) string {
	if strategy == "" {
		return "unknown"
	}
	if i := strings.Index(strategy, "+"); i > 0 {
		return strategy[:i]
	}
	if len(strategy) > 40 {
		return strategy[:40]
	}
	return strategy
}

// RecordChatFirstToken records the first-token latency in chat mode.
func RecordChatFirstToken(kbType, mode string, latSeconds float64) {
	if !enabled {
		return
	}
	chatFirstTokenLatency.WithLabelValues(labelKB(kbType), labelOr(mode, "unknown")).Observe(latSeconds)
}

// RecordChatFallback logs a chat fallback event.
func RecordChatFallback(kbType, reason string) {
	if !enabled {
		return
	}
	chatFallbackTotal.WithLabelValues(labelKB(kbType), labelOr(reason, "unknown")).Inc()
}

// RecordRetrievalCacheLookup records a retrieval cache hit/miss.
func RecordRetrievalCacheLookup(kbType, level string) {
	if !enabled {
		return
	}
	retrievalCacheHitRatio.WithLabelValues(labelKB(kbType), labelOr(level, "miss")).Inc()
}

// RecordAnswerCacheLookup records an answer cache hit/miss.
func RecordAnswerCacheLookup(kbType, hit string) {
	if !enabled {
		return
	}
	answerCacheHitRatio.WithLabelValues(labelKB(kbType), labelOr(hit, "miss")).Inc()
}

// RecordEmbeddingLatency records embedding API latency.
func RecordEmbeddingLatency(provider, model string, latSeconds float64) {
	if !enabled {
		return
	}
	embeddingLatency.WithLabelValues(labelOr(provider, "unknown"), labelOr(model, "unknown")).Observe(latSeconds)
}

// RecordRerankLatency records rerank API latency.
func RecordRerankLatency(provider, model string, latSeconds float64) {
	if !enabled {
		return
	}
	rerankLatency.WithLabelValues(labelOr(provider, "unknown"), labelOr(model, "unknown")).Observe(latSeconds)
}

// RecordLLMLatency records LLM first-token or total latency.
func RecordLLMLatency(provider, model, phase string, latSeconds float64) {
	if !enabled {
		return
	}
	llmLatency.WithLabelValues(labelOr(provider, "unknown"), labelOr(model, "unknown"), labelOr(phase, "unknown")).Observe(latSeconds)
}

// RecordModelTokenUsage records token consumption dimension.
func RecordModelTokenUsage(workspace, provider, model string, tokens float64) {
	if !enabled {
		return
	}
	modelTokenUsage.WithLabelValues(labelOr(workspace, "unknown"), labelOr(provider, "unknown"), labelOr(model, "unknown")).Add(tokens)
}

// RecordCostBudgetExceeded increments the daily budget alert counter.
func RecordCostBudgetExceeded(tenantID, operation string) {
	if !enabled {
		return
	}
	costBudgetExceeded.WithLabelValues(labelOr(tenantID, "unknown"), labelOr(operation, "unknown")).Inc()
}

// RecordCacheStaleHit records a stale cache hit (V1-37).
func RecordCacheStaleHit(workspace, kbType, cacheType string) {
	if !enabled {
		return
	}
	cacheStaleHitTotal.WithLabelValues(labelOr(workspace, "unknown"), labelKB(kbType), labelOr(cacheType, "unknown")).Inc()
}

// RecordCacheInvalidation records a cache invalidation event (V1-37).
func RecordCacheInvalidation(kbType, outcome string, entityCount int) {
	if !enabled {
		return
	}
	cacheInvalidationTrace.WithLabelValues(labelKB(kbType), labelOr(outcome, "unknown")).Inc()
}
