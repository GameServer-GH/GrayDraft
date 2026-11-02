package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/graydraft/gocommon/sealed"
	"gopkg.in/yaml.v3"
)

// Config holds Core service settings.
type Config struct {
	App struct {
		Name string `yaml:"name"`
		Env  string `yaml:"env"`
	} `yaml:"app"`
	HTTP struct {
		Port int `yaml:"port"`
	} `yaml:"http"`
	GRPC struct {
		Port int `yaml:"port"`
	} `yaml:"grpc"`
	Database struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
		Name     string `yaml:"name"`
		SSLMode  string `yaml:"sslmode"`
	} `yaml:"database"`
	Retrieval struct {
		DefaultTopK             int     `yaml:"default_top_k"`
		MaxTopK                 int     `yaml:"max_top_k"`
		RRFConstantK            int     `yaml:"rrf_constant_k"`
		VectorWeight            float64 `yaml:"vector_weight"`
		FulltextWeight          float64 `yaml:"fulltext_weight"`
		StructuredVectorWeight  float64 `yaml:"structured_vector_weight"` // 4.0 dual-vector RRF weight
		AutoEmbed               bool    `yaml:"auto_embed"`
		RerankEnabled           bool    `yaml:"rerank_enabled"`
		RerankL1Pool            int     `yaml:"rerank_l1_pool"`
		RerankL1TopN            int     `yaml:"rerank_l1_top_n"`
		RerankL2TopN            int     `yaml:"rerank_l2_top_n"`
		Semantic4DFilterEnabled bool    `yaml:"semantic_4d_filter_enabled"`
		Semantic4DLLMEnabled    bool    `yaml:"semantic_4d_llm_enabled"`
		Semantic4DStrictFilter  bool    `yaml:"semantic_4d_strict_filter"`
		Semantic4DEntityStrict  bool    `yaml:"semantic_4d_entity_strict"` // query 含实体时，带 4D 的文档须命中实体
		RerankSkipMinScore      float64 `yaml:"rerank_skip_min_score"`     // L1 高分且 4D 匹配时可跳过 rerank
	} `yaml:"retrieval"`
	Classifier struct {
		Enabled    bool `yaml:"enabled"`
		LLMEnabled bool `yaml:"llm_enabled"` // O-09: optional LLM intent when rules are ambiguous
		Weights    struct {
			ProperNoun struct {
				Fulltext float64 `yaml:"fulltext"`
				Vector   float64 `yaml:"vector"`
			} `yaml:"proper_noun"`
			Fact struct {
				Fulltext float64 `yaml:"fulltext"`
				Vector   float64 `yaml:"vector"`
			} `yaml:"fact"`
			Concept struct {
				Fulltext float64 `yaml:"fulltext"`
				Vector   float64 `yaml:"vector"`
			} `yaml:"concept"`
			Hybrid struct {
				Fulltext float64 `yaml:"fulltext"`
				Vector   float64 `yaml:"vector"`
			} `yaml:"hybrid"`
		} `yaml:"weights"`
		GlobalBoostWeight float64 `yaml:"global_boost_weight"`
	} `yaml:"classifier"`
	ModelGateway struct {
		URL                  string `yaml:"url"`
		TimeoutSeconds       int    `yaml:"timeout_seconds"`
		StreamTimeoutSeconds int    `yaml:"stream_timeout_seconds"`
		EmbeddingDim         int    `yaml:"embedding_dim"`
		EmbeddingModel       string `yaml:"embedding_model"`
		LLMModelFast         string `yaml:"llm_model_fast"`
		LLMModel             string `yaml:"llm_model"` // complex / high-accuracy
		RerankModel          string `yaml:"rerank_model"`
	} `yaml:"model_gateway"`
	Chat struct {
		OverviewModel           string  `yaml:"overview_model"`
		OverviewHedgeAfterMS    int     `yaml:"overview_hedge_after_ms"`
		MaxContextChunks        int     `yaml:"max_context_chunks"`
		SearchOnlyFallback      bool    `yaml:"search_only_fallback"`
		FeedbackDownvotePenalty float64 `yaml:"feedback_downvote_penalty"`
		FeedbackUpvoteBoost     float64 `yaml:"feedback_upvote_boost"`
		FeedbackDeleteBelow     float64 `yaml:"feedback_delete_below"`
		AnswerSelfCheck         bool    `yaml:"answer_self_check"` // O-10
		QueryEmbeddingTimeoutMs int     `yaml:"query_embedding_timeout_ms"`
		RetrievalTimeoutMs      int     `yaml:"retrieval_timeout_ms"`
		RerankTimeoutMs         int     `yaml:"rerank_timeout_ms"`
		LLMFirstTokenTimeoutMs  int     `yaml:"llm_first_token_timeout_ms"`
		LLMTotalTimeoutMs       int     `yaml:"llm_total_timeout_ms"`
		MaxContextTokens        int     `yaml:"max_context_tokens"`
	} `yaml:"chat"`
	AIWorker struct {
		URL            string `yaml:"url"`
		TimeoutSeconds int    `yaml:"timeout_seconds"`
	} `yaml:"ai_worker"`
	NATS struct {
		URL           string `yaml:"url"`
		Enabled       bool   `yaml:"enabled"`
		UGCSubject    string `yaml:"ugc_subject"`
		StageSubjects struct {
			Accepted       string `yaml:"accepted"`
			Parsed         string `yaml:"parsed"`
			MediaExtracted string `yaml:"media_extracted"`
			Chunked        string `yaml:"chunked"`
			Embedded       string `yaml:"embedded"`
			Indexed        string `yaml:"indexed"`
			QA             string `yaml:"qa"`
		} `yaml:"stage_subjects"`
		ConnectTimeoutSeconds int `yaml:"connect_timeout_seconds"`
		JetStream             struct {
			Enabled             bool     `yaml:"enabled"`
			Stream              string   `yaml:"stream"`
			Subjects            []string `yaml:"subjects"`
			StageStreamsEnabled bool     `yaml:"stage_streams_enabled"`
			Storage             string   `yaml:"storage"` // file | memory
			MaxAgeHours         int      `yaml:"max_age_hours"`
			MaxBytes            int64    `yaml:"max_bytes"`
			Replicas            int      `yaml:"replicas"`
			DLQStream           string   `yaml:"dlq_stream"`
			DLQSubject          string   `yaml:"dlq_subject"`
			DLQSubjects         []string `yaml:"dlq_subjects"`
		} `yaml:"jetstream"`
	} `yaml:"nats"`
	UGC struct {
		SyncFallback bool `yaml:"sync_fallback"`
	} `yaml:"ugc"`
	Redis struct {
		Host     string `yaml:"host"`
		Port     int    `yaml:"port"`
		Password string `yaml:"password"`
		DB       int    `yaml:"db"`
	} `yaml:"redis"`
	Cache struct {
		Enabled                 bool    `yaml:"enabled"`
		RedisEnabled            bool    `yaml:"redis_enabled"`
		L3Enabled               bool    `yaml:"l3_enabled"`
		SimilarityThreshold     float64 `yaml:"similarity_threshold"`
		TTLSeconds              int     `yaml:"ttl_seconds"`
		L1MaxItems              int     `yaml:"l1_max_items"`
		SemanticScanLimit       int     `yaml:"semantic_scan_limit"`
		SemanticIndexMax        int     `yaml:"semantic_index_max"`
		EntityPoolTTLSeconds    int     `yaml:"entity_pool_ttl_seconds"`
		InvalidationSubject     string  `yaml:"invalidation_subject"`
		FineGrainedInvalidation bool    `yaml:"fine_grained_invalidation"`
	} `yaml:"cache"`
	AnswerCache struct {
		Enabled          bool    `yaml:"enabled"`
		RedisEnabled     bool    `yaml:"redis_enabled"`
		TTLSeconds       int     `yaml:"ttl_seconds"`
		L1MaxItems       int     `yaml:"l1_max_items"`
		ShadowMode       bool    `yaml:"shadow_mode"`
		ShadowSampleRate float64 `yaml:"shadow_sample_rate"`
		PrefetchEnabled  bool    `yaml:"prefetch_enabled"`
	} `yaml:"answer_cache"`
	SmartImport struct {
		MinConfidence float64 `yaml:"min_confidence"`
	} `yaml:"smart_import"`
	TaxonomyEvolve struct {
		Enabled                    bool    `yaml:"enabled"`
		BlueprintEnabled           bool    `yaml:"blueprint_enabled"`
		BlueprintValidatorEnabled  bool    `yaml:"blueprint_validator_enabled"`
		BlueprintMinDocs           int     `yaml:"blueprint_min_docs"`
		CandidatePromoteDocCount   int     `yaml:"candidate_promote_doc_count"`
		CompressCandidateThreshold int     `yaml:"compress_candidate_threshold"`
		MaxKeywordsPerLabel        int     `yaml:"max_keywords_per_label"`
		SimilarityMergeMin         float64 `yaml:"similarity_merge_min"`
		SimilarityDistinctMax      float64 `yaml:"similarity_distinct_max"`
		EmbedAlignMin              float64 `yaml:"embed_align_min"`
		LabelBirthDocCount         int     `yaml:"label_birth_doc_count"`
		BackfillBatchSize          int     `yaml:"backfill_batch_size"`
		LLMMergeEnabled            bool    `yaml:"llm_merge_enabled"`
		CompressEnabled            bool    `yaml:"compress_enabled"`
	} `yaml:"taxonomy_evolve"`
	Multimodal struct {
		Enabled                bool    `yaml:"enabled"`
		BaseURL                string  `yaml:"base_url"`
		APIKey                 string  `yaml:"api_key"`
		VLPlusModel            string  `yaml:"vl_plus_model"`
		VLMaxModel             string  `yaml:"vl_max_model"`
		BasePrompt             string  `yaml:"base_prompt"`
		MaxFileBytes           int64   `yaml:"max_file_bytes"`
		TimeoutSeconds         int     `yaml:"timeout_seconds"`
		ForceVLMOnly           bool    `yaml:"force_vlm_only"`    // 跳过 triage/OCR，一律用 VLMax 识图
		ChainVLMEnabled        bool    `yaml:"chain_vlm_enabled"` // 多图顺序链式 VLM（前图摘要注入后图）
		Concurrency            int     `yaml:"concurrency"`       // 并发处理附件数，默认 3
		DocIntelEnabled        bool    `yaml:"doc_intel_enabled"`
		DocIntelBaseURL        string  `yaml:"doc_intel_base_url"`
		DocIntelAPIKey         string  `yaml:"doc_intel_api_key"`
		DocIntelModel          string  `yaml:"doc_intel_model"`
		StructuredVectorWeight float64 `yaml:"structured_vector_weight"`
	} `yaml:"multimodal"`
	Observability struct {
		MetricsEnabled  bool   `yaml:"metrics_enabled"`
		OtelEnabled     bool   `yaml:"otel_enabled"`
		OtelEndpoint    string `yaml:"otel_endpoint"`     // OTLP gRPC endpoint; empty → stdout exporter
		OtelServiceName string `yaml:"otel_service_name"` // default graydraft-core
	} `yaml:"observability"`
	Auth struct {
		JWTSecret     string `yaml:"jwt_secret"`
		AdminUser     string `yaml:"admin_user"`
		AdminPassword string `yaml:"admin_password"`
		TokenTTLHours int    `yaml:"token_ttl_hours"`
	} `yaml:"auth"`
	DynamicConfig struct {
		Enabled        bool   `yaml:"enabled"`
		Backend        string `yaml:"backend"`         // redis_pubsub | file_watch
		RefreshSeconds int    `yaml:"refresh_seconds"` // default 30
	} `yaml:"dynamic_config"`
	RateLimit struct {
		Enabled                          bool  `yaml:"enabled"`
		PerWorkspaceIngestQPS            int   `yaml:"per_workspace_ingest_qps"`
		PerWorkspaceChatConcurrency      int   `yaml:"per_workspace_chat_concurrency"`
		PerWorkspaceEmbeddingTokensDaily int64 `yaml:"per_workspace_embedding_tokens_daily"`
		MaxTaskChunks                    int   `yaml:"max_task_chunks"`
	} `yaml:"rate_limit"`
	Budget struct {
		Enabled         bool  `yaml:"enabled"`
		DailyChatLimit  int   `yaml:"daily_chat_limit"`  // O-15
		DailyTokenLimit int64 `yaml:"daily_token_limit"` // P1.5 model token budget alert
	} `yaml:"budget"`
	GlobalIndex struct {
		AutoRefreshHours  int  `yaml:"auto_refresh_hours"` // O-17; 0=disabled
		RefreshConcurrent bool `yaml:"refresh_concurrent"`
	} `yaml:"global_index"`
	StorageTier struct {
		ColdAfterDays      int  `yaml:"cold_after_days"` // O-18
		ExcludeColdDefault bool `yaml:"exclude_cold_default"`
	} `yaml:"storage_tier"`
	RAGOps struct {
		Enabled         bool    `yaml:"enabled"`
		StrikeDays      int     `yaml:"strike_days"`
		StrikeThreshold int     `yaml:"strike_threshold"`
		QualityBelow    float64 `yaml:"quality_below"`
		DefaultAction   string  `yaml:"default_action"` // rechunk | demote
	} `yaml:"ragops"`
}

// Load reads YAML from path and overlays environment variables.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	overlayEnv(&cfg)
	applyDefaults(&cfg)
	if cfg.Chat.OverviewHedgeAfterMS < 0 {
		return nil, fmt.Errorf("chat.overview_hedge_after_ms must be nonnegative")
	}
	if cfg.Chat.OverviewModel != "fast" && cfg.Chat.OverviewModel != "complex" {
		return nil, fmt.Errorf("chat.overview_model must be fast or complex")
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Chat.OverviewModel == "" {
		cfg.Chat.OverviewModel = "complex"
	}
	if cfg.Retrieval.DefaultTopK == 0 {
		cfg.Retrieval.DefaultTopK = 10
	}
	if cfg.Retrieval.MaxTopK == 0 {
		cfg.Retrieval.MaxTopK = 50
	}
	if cfg.Retrieval.RRFConstantK == 0 {
		cfg.Retrieval.RRFConstantK = 60
	}
	if cfg.Retrieval.VectorWeight == 0 {
		cfg.Retrieval.VectorWeight = 0.5
	}
	if cfg.Retrieval.FulltextWeight == 0 {
		cfg.Retrieval.FulltextWeight = 0.5
	}
	if cfg.Retrieval.RerankL1Pool == 0 {
		cfg.Retrieval.RerankL1Pool = 50
	}
	if cfg.Retrieval.RerankL1TopN == 0 {
		cfg.Retrieval.RerankL1TopN = 15
	}
	if cfg.Retrieval.RerankL2TopN == 0 {
		cfg.Retrieval.RerankL2TopN = 5
	}
	if cfg.Retrieval.RerankSkipMinScore <= 0 {
		cfg.Retrieval.RerankSkipMinScore = 0.78
	}
	if cfg.ModelGateway.TimeoutSeconds == 0 {
		cfg.ModelGateway.TimeoutSeconds = 10
	}
	if cfg.ModelGateway.StreamTimeoutSeconds == 0 {
		cfg.ModelGateway.StreamTimeoutSeconds = 300
	}
	if cfg.Chat.MaxContextChunks == 0 {
		cfg.Chat.MaxContextChunks = 5
	}
	if cfg.Chat.FeedbackDownvotePenalty == 0 {
		cfg.Chat.FeedbackDownvotePenalty = 0.12
	}
	if cfg.Chat.FeedbackUpvoteBoost == 0 {
		cfg.Chat.FeedbackUpvoteBoost = 0.08
	}
	if cfg.Chat.FeedbackDeleteBelow == 0 {
		cfg.Chat.FeedbackDeleteBelow = 0.35
	}
	if cfg.SmartImport.MinConfidence == 0 {
		cfg.SmartImport.MinConfidence = 0.55
	}
	if cfg.TaxonomyEvolve.CandidatePromoteDocCount == 0 {
		cfg.TaxonomyEvolve.CandidatePromoteDocCount = 5
	}
	if cfg.TaxonomyEvolve.CompressCandidateThreshold == 0 {
		cfg.TaxonomyEvolve.CompressCandidateThreshold = 20
	}
	if cfg.TaxonomyEvolve.MaxKeywordsPerLabel == 0 {
		cfg.TaxonomyEvolve.MaxKeywordsPerLabel = 10
	}
	if cfg.TaxonomyEvolve.SimilarityMergeMin == 0 {
		cfg.TaxonomyEvolve.SimilarityMergeMin = 0.85
	}
	if cfg.TaxonomyEvolve.SimilarityDistinctMax == 0 {
		cfg.TaxonomyEvolve.SimilarityDistinctMax = 0.5
	}
	if cfg.TaxonomyEvolve.BlueprintMinDocs == 0 {
		cfg.TaxonomyEvolve.BlueprintMinDocs = 5
	}
	if cfg.TaxonomyEvolve.EmbedAlignMin == 0 {
		cfg.TaxonomyEvolve.EmbedAlignMin = 0.85
	}
	if cfg.TaxonomyEvolve.LabelBirthDocCount == 0 {
		cfg.TaxonomyEvolve.LabelBirthDocCount = 8
	}
	if cfg.TaxonomyEvolve.BackfillBatchSize == 0 {
		cfg.TaxonomyEvolve.BackfillBatchSize = 80
	}
	if cfg.Multimodal.VLPlusModel == "" {
		cfg.Multimodal.VLPlusModel = "internvl-latest"
	}
	if cfg.Multimodal.VLMaxModel == "" {
		cfg.Multimodal.VLMaxModel = "internvl-latest"
	}
	if cfg.Multimodal.MaxFileBytes == 0 {
		cfg.Multimodal.MaxFileBytes = 12 << 20
	}
	if cfg.Multimodal.TimeoutSeconds == 0 {
		cfg.Multimodal.TimeoutSeconds = 120
	}
	if cfg.Multimodal.Concurrency <= 0 {
		cfg.Multimodal.Concurrency = 3
	}
	if cfg.Retrieval.StructuredVectorWeight == 0 {
		cfg.Retrieval.StructuredVectorWeight = 0.35
	}
	if cfg.AIWorker.TimeoutSeconds == 0 {
		cfg.AIWorker.TimeoutSeconds = 15
	}
	if cfg.ModelGateway.EmbeddingDim == 0 {
		cfg.ModelGateway.EmbeddingDim = 1024
	}
	if cfg.ModelGateway.EmbeddingModel == "" {
		cfg.ModelGateway.EmbeddingModel = "text-embedding-v4"
	}
	if cfg.ModelGateway.LLMModelFast == "" {
		cfg.ModelGateway.LLMModelFast = "intern-s1-mini"
	}
	if cfg.ModelGateway.LLMModel == "" {
		cfg.ModelGateway.LLMModel = "intern-s2"
	}
	if cfg.ModelGateway.RerankModel == "" {
		cfg.ModelGateway.RerankModel = "qwen3-rerank"
	}
	if cfg.NATS.UGCSubject == "" {
		cfg.NATS.UGCSubject = "graydraft.ugc.ingest"
	}
	if cfg.NATS.StageSubjects.Accepted == "" {
		cfg.NATS.StageSubjects.Accepted = "graydraft.ingest.accepted"
	}
	if cfg.NATS.StageSubjects.Parsed == "" {
		cfg.NATS.StageSubjects.Parsed = "graydraft.ingest.parsed"
	}
	if cfg.NATS.StageSubjects.MediaExtracted == "" {
		cfg.NATS.StageSubjects.MediaExtracted = "graydraft.ingest.media_extracted"
	}
	if cfg.NATS.StageSubjects.Chunked == "" {
		cfg.NATS.StageSubjects.Chunked = "graydraft.ingest.chunked"
	}
	if cfg.NATS.StageSubjects.Embedded == "" {
		cfg.NATS.StageSubjects.Embedded = "graydraft.ingest.embedded"
	}
	if cfg.NATS.StageSubjects.Indexed == "" {
		cfg.NATS.StageSubjects.Indexed = "graydraft.ingest.indexed"
	}
	if cfg.NATS.StageSubjects.QA == "" {
		cfg.NATS.StageSubjects.QA = "graydraft.ingest.qa"
	}
	if cfg.NATS.ConnectTimeoutSeconds == 0 {
		cfg.NATS.ConnectTimeoutSeconds = 5
	}
	if cfg.NATS.JetStream.Stream == "" {
		cfg.NATS.JetStream.Stream = "GRAYDRAFT_UGC"
	}
	if len(cfg.NATS.JetStream.Subjects) == 0 {
		cfg.NATS.JetStream.Subjects = []string{"graydraft.ugc.ingest"}
	}
	if cfg.NATS.JetStream.DLQStream == "" {
		cfg.NATS.JetStream.DLQStream = "GRAYDRAFT_UGC_DLQ"
	}
	if cfg.NATS.JetStream.DLQSubject == "" {
		cfg.NATS.JetStream.DLQSubject = "graydraft.ugc.dlq"
	}
	if cfg.NATS.JetStream.MaxAgeHours == 0 {
		cfg.NATS.JetStream.MaxAgeHours = 168
	}
	if cfg.NATS.JetStream.Replicas == 0 {
		cfg.NATS.JetStream.Replicas = 1
	}
	if cfg.Cache.SimilarityThreshold == 0 {
		cfg.Cache.SimilarityThreshold = 0.92
	}
	if cfg.Cache.TTLSeconds == 0 {
		cfg.Cache.TTLSeconds = 3600
	}
	if cfg.Cache.L1MaxItems == 0 {
		cfg.Cache.L1MaxItems = 10000
	}
	if cfg.Cache.SemanticScanLimit == 0 {
		cfg.Cache.SemanticScanLimit = 32
	}
	if cfg.Cache.SemanticIndexMax == 0 {
		cfg.Cache.SemanticIndexMax = 200
	}
	if cfg.Cache.EntityPoolTTLSeconds == 0 {
		cfg.Cache.EntityPoolTTLSeconds = 3600
	}
	if cfg.Cache.InvalidationSubject == "" {
		cfg.Cache.InvalidationSubject = "graydraft.cache.invalidate"
	}
	if cfg.AnswerCache.TTLSeconds == 0 {
		cfg.AnswerCache.TTLSeconds = 1800
	}
	if cfg.AnswerCache.L1MaxItems == 0 {
		cfg.AnswerCache.L1MaxItems = 5000
	}
	if cfg.Redis.Port == 0 {
		cfg.Redis.Port = 6379
	}
	if cfg.StorageTier.ColdAfterDays == 0 {
		cfg.StorageTier.ColdAfterDays = 90
	}
	if cfg.RAGOps.StrikeDays == 0 {
		cfg.RAGOps.StrikeDays = 3
	}
	if cfg.RAGOps.StrikeThreshold == 0 {
		cfg.RAGOps.StrikeThreshold = 3
	}
	if cfg.RAGOps.QualityBelow == 0 {
		cfg.RAGOps.QualityBelow = 0.45
	}
	if cfg.RAGOps.DefaultAction == "" {
		cfg.RAGOps.DefaultAction = "rechunk"

		// V1-38 / V1-39: dynamic config and rate limit defaults
		if cfg.DynamicConfig.RefreshSeconds == 0 {
			cfg.DynamicConfig.RefreshSeconds = 30
		}
		if cfg.DynamicConfig.Backend == "" {
			cfg.DynamicConfig.Backend = "file_watch"
		}
		if cfg.RateLimit.PerWorkspaceIngestQPS == 0 {
			cfg.RateLimit.PerWorkspaceIngestQPS = 5
		}
		if cfg.RateLimit.PerWorkspaceChatConcurrency == 0 {
			cfg.RateLimit.PerWorkspaceChatConcurrency = 20
		}
		if cfg.RateLimit.MaxTaskChunks == 0 {
			cfg.RateLimit.MaxTaskChunks = 2000
		}
	}
}

func overlayEnv(cfg *Config) {
	if v, ok := os.LookupEnv("OVERVIEW_HEDGE_AFTER_MS"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Chat.OverviewHedgeAfterMS = n
		} else {
			cfg.Chat.OverviewHedgeAfterMS = -1
		}
	}
	if v, ok := os.LookupEnv("OVERVIEW_MODEL"); ok {
		cfg.Chat.OverviewModel = strings.ToLower(strings.TrimSpace(v))
	}

	if v := os.Getenv("HTTP_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.HTTP.Port = p
		}
	}
	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Database.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Database.Port = p
		}
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.Database.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Database.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database.Name = v
	}
	if v := os.Getenv("DB_SSLMODE"); v != "" {
		cfg.Database.SSLMode = v
	}
	if v := os.Getenv("MODEL_GATEWAY_URL"); v != "" {
		cfg.ModelGateway.URL = v
	}
	if v := os.Getenv("EMBEDDING_MODEL"); v != "" {
		cfg.ModelGateway.EmbeddingModel = v
	}
	if v := os.Getenv("LLM_MODEL_FAST"); v != "" {
		cfg.ModelGateway.LLMModelFast = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.ModelGateway.LLMModel = v
	}
	if v := os.Getenv("LLM_MODEL_COMPLEX"); v != "" {
		cfg.ModelGateway.LLMModel = v
	}
	if v := os.Getenv("RERANK_MODEL"); v != "" {
		cfg.ModelGateway.RerankModel = v
	}
	if v := os.Getenv("AI_WORKER_URL"); v != "" {
		cfg.AIWorker.URL = v
	}
	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.NATS.URL = v
	}
	if v := os.Getenv("NATS_ENABLED"); v != "" {
		cfg.NATS.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("NATS_UGC_SUBJECT"); v != "" {
		cfg.NATS.UGCSubject = v
	}
	if v := os.Getenv("NATS_JETSTREAM_ENABLED"); v != "" {
		cfg.NATS.JetStream.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("NATS_STREAM"); v != "" {
		cfg.NATS.JetStream.Stream = v
	}
	if v := os.Getenv("NATS_DLQ_SUBJECT"); v != "" {
		cfg.NATS.JetStream.DLQSubject = v
	}
	if v := os.Getenv("NATS_REPLICAS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.NATS.JetStream.Replicas = n
		}
	}
	if v := os.Getenv("UGC_SYNC_FALLBACK"); v != "" {
		cfg.UGC.SyncFallback = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		cfg.Redis.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if p, err := strconv.Atoi(v); err == nil {
			cfg.Redis.Port = p
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		cfg.Redis.Password = v
	}
	if v := os.Getenv("CACHE_ENABLED"); v != "" {
		cfg.Cache.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("CACHE_SIMILARITY_THRESHOLD"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.Cache.SimilarityThreshold = f
		}
	}
	if v := os.Getenv("ANSWER_CACHE_ENABLED"); v != "" {
		cfg.AnswerCache.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("ANSWER_CACHE_TTL_SECONDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.AnswerCache.TTLSeconds = n
		}
	}
	if v := os.Getenv("DASHSCOPE_API_KEY"); v != "" {
		cfg.Multimodal.APIKey = v
		if !cfg.Multimodal.Enabled {
			cfg.Multimodal.Enabled = true
		}
	}
	if v := os.Getenv("MULTIMODAL_API_KEY"); v != "" {
		cfg.Multimodal.APIKey = v
		if !cfg.Multimodal.Enabled {
			cfg.Multimodal.Enabled = true
		}
	}
	if v := os.Getenv("MULTIMODAL_ENABLED"); v != "" {
		cfg.Multimodal.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("MULTIMODAL_BASE_URL"); v != "" {
		cfg.Multimodal.BaseURL = v
	}
	if cfg.Multimodal.BaseURL == "" {
		if v := os.Getenv("INTERN_BASE_URL"); v != "" {
			cfg.Multimodal.BaseURL = v
		}
	}
	if cfg.Multimodal.APIKey == "" {
		v := strings.TrimSpace(os.Getenv("INTERN_API_KEY"))
		if v == "" {
			v = sealed.Intern()
		}
		if v != "" {
			cfg.Multimodal.APIKey = v
			cfg.Multimodal.Enabled = true
		}
	}
	if v := os.Getenv("MULTIMODAL_VL_MAX_MODEL"); v != "" {
		cfg.Multimodal.VLMaxModel = v
		cfg.Multimodal.VLPlusModel = v
	}
	if v := os.Getenv("MULTIMODAL_FORCE_VLM_ONLY"); v != "" {
		cfg.Multimodal.ForceVLMOnly = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("MULTIMODAL_CHAIN_VLM_ENABLED"); v != "" {
		cfg.Multimodal.ChainVLMEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("METRICS_ENABLED"); v != "" {
		cfg.Observability.MetricsEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("OTEL_ENABLED"); v != "" {
		cfg.Observability.OtelEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); v != "" {
		cfg.Observability.OtelEndpoint = v
	}
	if v := os.Getenv("OTEL_SERVICE_NAME"); v != "" {
		cfg.Observability.OtelServiceName = v
	}
	if v := os.Getenv("CLASSIFIER_LLM_ENABLED"); v != "" {
		cfg.Classifier.LLMEnabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("CHAT_ANSWER_SELF_CHECK"); v != "" {
		cfg.Chat.AnswerSelfCheck = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("BUDGET_ENABLED"); v != "" {
		cfg.Budget.Enabled = v == "1" || v == "true" || v == "yes"
	}
	if v := os.Getenv("BUDGET_DAILY_CHAT_LIMIT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			cfg.Budget.DailyChatLimit = n
		}
	}
	if v := os.Getenv("BUDGET_DAILY_TOKEN_LIMIT"); v != "" {
		if n, err := strconv.ParseInt(v, 10, 64); err == nil && n > 0 {
			cfg.Budget.DailyTokenLimit = n
		}
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("ADMIN_USER"); v != "" {
		cfg.Auth.AdminUser = v
	}
	if v := os.Getenv("ADMIN_PASSWORD"); v != "" {
		cfg.Auth.AdminPassword = v
	}
	if v := os.Getenv("JWT_TOKEN_TTL_HOURS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.Auth.TokenTTLHours = n
		}
	}
	if v := os.Getenv("SMART_IMPORT_MIN_CONFIDENCE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.SmartImport.MinConfidence = f
		}
	}
	if cfg.Database.Port == 0 {
		cfg.Database.Port = 5432
	}
	if cfg.Database.User == "" {
		cfg.Database.User = "graydraft_admin"
	}
	if cfg.Database.SSLMode == "" {
		cfg.Database.SSLMode = "disable"
	}
	if cfg.Auth.AdminUser == "" {
		cfg.Auth.AdminUser = "admin"
	}
	if cfg.Auth.TokenTTLHours == 0 {
		cfg.Auth.TokenTTLHours = 24
	}
}

// DSN returns a PostgreSQL connection string.
func (c *Config) DSN() string {
	return fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Database.Host,
		c.Database.Port,
		c.Database.User,
		c.Database.Password,
		c.Database.Name,
		c.Database.SSLMode,
	)
}
