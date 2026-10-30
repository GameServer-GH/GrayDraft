// Package main is the API Gateway entry for GrayDraft Core.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/config"
	"github.com/graydraft/core/handler"
	"github.com/graydraft/core/middleware"
	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability"
	"github.com/graydraft/core/observability/trace"
	"github.com/graydraft/core/repository/nats"
	"github.com/graydraft/core/repository/postgres"
	redisrepo "github.com/graydraft/core/repository/redis"
	"github.com/graydraft/core/service/agent"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/chat"
	"github.com/graydraft/core/service/classifier"
	"github.com/graydraft/core/service/confidence"
	"github.com/graydraft/core/service/cost"
	"github.com/graydraft/core/service/dynconfig"
	"github.com/graydraft/core/service/embedder"
	"github.com/graydraft/core/service/kbscope"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/multimodal"
	"github.com/graydraft/core/service/ragops"
	"github.com/graydraft/core/service/reranker"
	"github.com/graydraft/core/service/retriever"
	"github.com/graydraft/core/service/smartimport"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/ugc"
	"github.com/graydraft/gocommon/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/config.dev.yaml"
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	logLevel := os.Getenv("LOG_LEVEL")
	if logLevel == "" {
		logLevel = "info"
	}
	log := logger.New(logLevel)

	ctx := context.Background()
	db, err := postgres.New(ctx, cfg.DSN())
	if err != nil {
		log.Error("connect postgres", "error", err)
		os.Exit(1)
	}
	defer db.Close()

	// Run database migrations on startup (all use IF NOT EXISTS, idempotent).
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	if err := postgres.RunMigrations(db, migrationsDir); err != nil {
		log.Error("migration failed", "error", err)
		os.Exit(1)
	}

	chunkRepo := postgres.NewChunkRepo(db)
	chunkRepo.SetSemantic4DStrict(cfg.Retrieval.Semantic4DStrictFilter)
	chunkRepo.SetSemantic4DEntityStrict(cfg.Retrieval.Semantic4DEntityStrict)
	docRepo := postgres.NewDocumentRepo(db)
	scopeRepo := postgres.NewScopeRepo(db, docRepo)
	kbResolver := kbscope.NewResolver(scopeRepo)
	ugcRepo := postgres.NewUGCRepo(db)
	embClient := embedder.NewClient(cfg)
	rrClient := reranker.NewClient(cfg)
	cacheMgr, err := cache.NewManager(cfg)
	if err != nil {
		log.Warn("cache init failed", "error", err)
	}
	answerCache, err := answercache.NewManager(cfg)
	if err != nil {
		log.Warn("answer cache init failed", "error", err)
	}
	llmClient := llmclient.NewClient(cfg)
	clf := classifier.New(cfg).WithLLM(llmClient)
	hybrid := retriever.NewHybrid(chunkRepo, cfg, embClient, rrClient, cacheMgr, clf, llmClient)

	natsPub, err := nats.NewPublisher(cfg)
	if err != nil {
		log.Warn("nats publisher init failed", "error", err)
	}
	if natsPub != nil {
		defer natsPub.Close()
	}
	ugcSvc := ugc.NewService(cfg, ugcRepo, natsPub, ugc.NewWorkerClient(cfg)).WithCaches(cacheMgr, answerCache)
	// V1-07: wire stage pipeline when JetStream stage streams enabled.
	if natsPub != nil && natsPub.StageStreamsEnabled() {
		stagePipeline := ugc.NewStagePipeline(natsPub, ugcRepo)
		ugcSvc.WithStagePipeline(stagePipeline)
		log.Info("stage pipeline enabled")
	}
	mmCache := postgres.NewMultimodalCacheRepo(db)
	mmProc := multimodal.NewProcessor(cfg, mmCache)
	taxEvolveSvc := taxonomyevolve.NewService(docRepo, llmClient, embClient, cfg).WithCaches(cacheMgr, answerCache)
	taxonomyevolve.SetDefault(taxEvolveSvc)
	smartImportSvc := smartimport.NewService(ugcSvc, docRepo, chunkRepo, llmClient, embClient, mmProc, cfg).WithCaches(cacheMgr, answerCache)
	ragopsHealer := ragops.NewHealer(chunkRepo, docRepo, ugcSvc, cfg).WithCaches(cacheMgr, answerCache)
	dynCfg := dynconfig.New(cfg)
	defer dynCfg.Shutdown()

	healthH := handler.NewHealthHandler(db, natsPub, cacheMgr, answerCache)
	chatFeedbackSvc := chat.NewFeedbackService(chunkRepo, cfg, ragopsHealer)

	retrievalH := handler.NewRetrievalHandler(hybrid, kbResolver)
	classifyH := handler.NewClassifyHandler(clf)
	chatFeedbackH := handler.NewChatFeedbackHandler(chatFeedbackSvc)
	ugcH := handler.NewUGCHandler(ugcSvc, kbResolver)
	smartImportH := handler.NewSmartImportHandler(smartImportSvc, kbResolver)
	multimodalH := handler.NewMultimodalHandler(mmProc, mmCache)
	adminH := handler.NewAdminHandler(chunkRepo, ugcRepo)
	adminOpsH := handler.NewAdminOpsHandler(chunkRepo, cfg, ragopsHealer)
	agentRunRepo := postgres.NewAgentRunRepo(db)
	runEventNotifier := nats.NewRunEventNotifier(natsPub)
	agentRunRepo = agentRunRepo.WithEventNotifier(runEventNotifier)
	costRepo := postgres.NewCostRepo(db, cfg.Budget.DailyTokenLimit)
	cost.SetDefault(costRepo)
	cost.TokenUsageHook = observability.RecordModelTokenUsage
	evalRepo := postgres.NewEvalRepo(db)
	adminAgentH := handler.NewAdminAgentHandler(agentRunRepo)
	adminCostH := handler.NewAdminCostHandler(costRepo, cfg.Budget.DailyTokenLimit)
	dlqH := handler.NewDLQHandler(natsPub, ugcRepo)
	authH := handler.NewAuthHandler(cfg)
	kbPanelH := handler.NewKBPanelHandler(docRepo)
	workspaceDataH := handler.NewWorkspaceDataHandler(docRepo, cacheMgr, answerCache)
	workspaceCtxH := handler.NewWorkspaceContextHandler(docRepo, llmClient)
	kbScopeH := handler.NewKBScopeHandler(kbResolver, docRepo)
	toolRegistryRepo := postgres.NewToolRegistryRepo(db)
	toolCache := agent.NewTenantToolCache(toolRegistryRepo, time.Minute)
	globalBudgetRepo := postgres.NewGlobalBudgetRepo(db)
	budgetGuard := agent.GlobalBudgetGuard{
		Policy:    globalBudgetRepo.Policy,
		DailyRuns: globalBudgetRepo.DailyRuns,
		DailyTokens: func(ctx context.Context, tenantID string) (int64, error) {
			return costRepo.DailyTotals(ctx, tenantID)
		},
	}
	toolFactory := agent.ToolFactory{
		Defaults: agent.DefaultEducationRegistry(),
		Cache:    toolCache,
		Build: func(run agent.RunRecord) map[string]agent.ToolFunc {
			allowed := map[string]bool{}
			for _, kb := range run.AuthorizedKBs {
				allowed[kb] = true
			}
			edu := agent.EducationTools{
				Searcher: hybrid, Navigator: chunkRepo, Catalog: docRepo,
				TenantID: run.TenantID, AllowedKB: allowed,
			}
			note := agent.NoteDraftTool{
				TenantID: run.TenantID, AllowedKB: allowed,
				Ingest: func(ctx context.Context, tenantID, kbType, title, body, sourceQuery string, citations []string) (string, error) {
					resp, err := ugcSvc.Notify(ctx, model.UGCNotifyRequest{
						TenantID: tenantID, KBType: kbType, Title: title, Content: body,
						Tags: []string{"edu-note"},
						Extensions: model.Extensions{
							"source_query": sourceQuery,
							"citations":    citations,
							"doc_type":     "note",
						},
					})
					if err != nil {
						return "", err
					}
					if resp == nil {
						return "", nil
					}
					return resp.TaskID, nil
				},
			}
			return map[string]agent.ToolFunc{
				"knowledge.lexical":  edu.Lexical,
				"knowledge.semantic": edu.Semantic,
				"knowledge.read":     edu.Read,
				"knowledge.expand":   edu.Expand,
				"knowledge.catalog":  edu.CatalogDocs,
				"note.draft":         note.Call,
			}
		},
	}
	candidatePolicy := os.Getenv("GRAYDRAFT_AGENT_CANARY_POLICY")
	canaryRate, _ := strconv.ParseFloat(os.Getenv("GRAYDRAFT_AGENT_CANARY_RATE"), 64)
	canary := &agent.CanaryController{SampleRate: canaryRate, MinSamples: 20, MaxFailureRate: .2, Cooldown: 10 * time.Minute}
	var toolSecrets agent.SecretProvider = agent.EnvironmentSecretProvider{}
	if vaultURL, vaultToken := os.Getenv("VAULT_ADDR"), os.Getenv("VAULT_TOKEN"); vaultURL != "" && vaultToken != "" {
		toolSecrets = agent.VaultSecretProvider{BaseURL: vaultURL, Token: vaultToken}
	}
	var sandboxRunner agent.SandboxRunner = agent.DockerSandboxRunner{Image: os.Getenv("GRAYDRAFT_TOOL_RUNNER_IMAGE"), EgressNetwork: os.Getenv("GRAYDRAFT_TOOL_EGRESS_NETWORK")}
	if runnerURL := os.Getenv("GRAYDRAFT_TOOL_RUNNER_URL"); runnerURL != "" {
		sandboxRunner = agent.RemoteSandboxRunner{URL: runnerURL, Token: os.Getenv("GRAYDRAFT_TOOL_RUNNER_TOKEN")}
	}
	var shortMemoryCache agent.MemoryCache = agent.NewInProcessMemoryCache()
	if redisMemory, err := redisrepo.NewMemoryCache(cfg); err == nil {
		shortMemoryCache = redisMemory
		defer redisMemory.Close()
	} else {
		log.Warn("agent memory redis unavailable; using in-process fallback", "error", err)
	}
	agentPlanner := agent.FirstHopPlanner{
		Inner: agent.LLMPlanner{LLM: llmClient},
		Select: func(query string, _ []string) (string, bool) {
			q := agent.CurrentUserQuery(query)
			if agent.PrefersLexical(q) {
				return "knowledge.lexical", true
			}
			plan := clf.ClassifyAndRoute(q)
			return agent.FirstHopFromIntent(string(plan.Intent)), true
		},
	}
	agentLauncher := &agent.Launcher{
		Store: agentRunRepo, Planner: agentPlanner, PerTenant: 4, Budget: budgetGuard,
		Memory: func(run agent.RunRecord) agent.ShortTermMemory {
			base := postgres.NewMemoryRepo(db, run.TenantID, run.ActorID)
			return agent.CachedMemory(base, shortMemoryCache, run.TenantID+":"+run.ConversationID)
		},
		LongMemory:   func(run agent.RunRecord) agent.LongTermMemory { return postgres.NewLongMemoryRepo(db) },
		VectorMemory: func(run agent.RunRecord) agent.VectorLongTermMemory { return postgres.NewLongMemoryRepo(db) },
		Embedder:     func(agent.RunRecord) agent.Embedder { return embClient },
		OnComplete: func(run agent.RunRecord, result agent.Result, err error) {
			if candidatePolicy != "" && run.PolicyVersion == candidatePolicy {
				canary.Observe(err == nil && result.State == agent.StateCompleted)
			}
		},
		Tools: func(run agent.RunRecord) agent.ToolExecutor {
			reg, err := toolFactory.RegistryFor(context.Background(), run)
			if err != nil {
				reg = agent.DefaultEducationRegistry()
			}
			inner := &agent.RegistryExecutor{
				Registry: reg,
				Scopes:   map[string]bool{"knowledge:read": true, "note:write": true},
				Funcs:    toolFactory.Build(run),
			}
			governance := postgres.NewToolGovernanceRepo(db, run.TenantID)
			return agent.GovernedExecutor{
				Registry: inner.Registry, Inner: inner, Scopes: inner.Scopes,
				Approvals: governance, Audit: governance, RunID: run.RunID, MaxOutput: 64 * 1024,
				Runner:  sandboxRunner,
				Secrets: toolSecrets,
			}
		},
	}
	if llmClient != nil {
		agentLauncher.Streamer = llmClient
		agentLauncher.Validator = agent.GroundedAnswerValidator{
			RequiredCitations: 1,
			NLI:               confidence.LLMCoverageNLI{LLM: llmClient},
		}
	}
	policyVersion := os.Getenv("GRAYDRAFT_AGENT_POLICY_VERSION")
	if policyVersion == "" {
		policyVersion = agent.EduPolicyVersion
	}
	evalAuthorizedKBs := strings.Split(os.Getenv("AGENT_EVAL_AUTHORIZED_KBS"), ",")
	if len(evalAuthorizedKBs) == 1 && evalAuthorizedKBs[0] == "" {
		evalAuthorizedKBs = []string{"tutorial"}
	}
	evalSvc := &agent.EvalService{
		Store:       evalRepo,
		DatasetRoot: agent.DefaultDatasetRoot(),
		Executor: agent.NewLauncherReplayExecutor(agent.LauncherReplayConfig{
			Launcher:      agentLauncher,
			PolicyVersion: policyVersion,
			AuthorizedKBs: evalAuthorizedKBs,
			Limits:        agent.Limits{MaxSteps: 8, MaxToolCalls: 12},
		}),
	}
	adminEvalH := handler.NewAdminEvalHandler(evalSvc, evalRepo)
	var agentRunLauncher handler.RunLauncher
	if natsPub == nil || !natsPub.JetStreamEnabled() {
		log.Error("agent runtime requires JetStream; set NATS_URL and enable jetstream")
	} else if _, err := natsPub.StartAgentConsumer(ctx, func(workCtx context.Context, run agent.RunRecord) error {
		_, err := agentLauncher.Run(workCtx, run, agent.Limits{Deadline: run.Deadline})
		return err
	}); err != nil {
		log.Error("agent jetstream consumer failed", "error", err)
	} else {
		agentRunLauncher = nats.QueueLauncher{Publisher: natsPub}
		log.Info("agent jetstream worker enabled")
	}
	agentH := handler.NewAgentHandler(agentRunRepo).
		WithLauncher(agentRunLauncher).
		WithEventWatcher(runEventNotifier).
		WithShadow(&handler.ShadowRunConfig{Controller: canary, CandidatePolicy: candidatePolicy}).
		WithBudget(&budgetGuard)
	adminApprovalsH := handler.NewAdminApprovalsHandler(func(tenant string) handler.ToolApprovalLister {
		return postgres.NewToolGovernanceRepo(db, tenant)
	})
	agentMemoryH := handler.NewAgentMemoryHandler(func(tenantID, actorID string) handler.MemoryStore {
		return postgres.NewMemoryRepo(db, tenantID, actorID)
	})
	longMemoryH := handler.NewLongMemoryHandler(func(tenantID, actorID string) agent.LongMemoryAdmin { return postgres.NewLongMemoryRepo(db) })
	productThreadH := handler.NewProductThreadHandler(postgres.NewProductThreadRepo(db))
	toolRegistryH := handler.NewToolRegistryHandler(toolRegistryRepo)
	toolGovernanceH := handler.NewToolGovernanceHandler(func(tenantID string) handler.ToolGovernanceAdmin {
		return postgres.NewToolGovernanceRepo(db, tenantID)
	})
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		repo := postgres.NewToolGovernanceRepo(db, "")
		if err := repo.ExpireApprovals(context.Background()); err != nil {
			log.Warn("initial tool approval expiry failed", "error", err)
		}
		for range ticker.C {
			if err := repo.ExpireApprovals(context.Background()); err != nil {
				log.Warn("tool approval expiry failed", "error", err)
			}
		}
	}()
	longMemoryRepo := postgres.NewLongMemoryRepo(db)
	go func() {
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			if err := longMemoryRepo.CleanupAllExpired(context.Background()); err != nil {
				log.Warn("long memory cleanup failed", "error", err)
			}
		}
	}()
	if hours, err := strconv.Atoi(os.Getenv("AGENT_EVAL_INTERVAL_HOURS")); err == nil && hours > 0 {
		go runPeriodicAgentEval(ctx, evalSvc, time.Duration(hours)*time.Hour, log)
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	observability.Init(cfg.Observability.MetricsEnabled)
	agent.RecordAnswerMetric = observability.RecordAgentAnswerGroundedness
	agent.RecordToolMetric = observability.RecordAgentToolCall
	agent.RecordRunMetric = observability.RecordAgentRun
	agent.RecordCanaryMetric = observability.RecordAgentCanary

	otelShutdown, otelErr := trace.InitProvider(ctx, trace.ProviderConfig{
		Enabled:     cfg.Observability.OtelEnabled,
		Endpoint:    cfg.Observability.OtelEndpoint,
		ServiceName: cfg.Observability.OtelServiceName,
	})
	if otelErr != nil {
		log.Warn("otel provider init failed", "error", otelErr)
	} else if otelShutdown != nil {
		defer func() { _ = otelShutdown(context.Background()) }()
	}

	r := gin.New()
	r.Use(gin.Recovery(), middleware.RequestID())
	r.Use(middleware.OtelTrace(cfg.Observability.OtelEnabled))
	r.Use(middleware.ChatBudget(cfg))

	// V1-14: per-workspace rate limiter
	rateLimiter := middleware.NewRateLimiter(dynCfg)
	if observability.Enabled() {
		r.Use(middleware.Prometheus())
	}

	r.GET("/api/health", healthH.Health)
	r.GET("/health", healthH.Health)
	if observability.Enabled() {
		r.GET("/metrics", gin.WrapH(promhttp.Handler()))
	}
	if observability.Enabled() && natsPub != nil {
		go observability.RunDLQCollector(ctx, natsPub, 30*time.Second)
	}
	if cfg.GlobalIndex.AutoRefreshHours > 0 {
		go runGlobalIndexRefresh(ctx, chunkRepo, cfg)
	}

	v1 := r.Group("/api/v1")
	{
		v1.POST("/retrieval/search", retrievalH.Search)
		v1.POST("/retrieval/diagnose", retrievalH.Diagnose)
		v1.POST("/classifier/preview", classifyH.Classify)
		v1.POST("/chat/feedback", chatFeedbackH.Submit)
		v1.POST("/ugc/notify", rateLimiter.IngestRateLimit(), ugcH.Notify)
		v1.POST("/ugc/smart-import", rateLimiter.IngestRateLimit(), smartImportH.Import)
		v1.GET("/ugc/tasks/:task_id", ugcH.TaskStatus)
		v1.GET("/ugc/tasks/:task_id/events", ugcH.TaskEvents)
		v1.GET("/multimodal/prompt-defaults", multimodalH.GetPromptDefaults)
		v1.GET("/multimodal/media/:md5", multimodalH.GetMedia)
		v1.POST("/auth/login", authH.Login)

		v1.POST("/tenants/ensure", kbScopeH.EnsureTenant)
		tenant := v1.Group("/tenants/:tenant_id")
		{
			tenant.GET("/kbs", kbScopeH.ListKnowledgeBases)
			tenant.POST("/kbs/ensure", kbScopeH.EnsureKnowledgeBase)
			kbRes := tenant.Group("/kbs/:kb_id")
			{
				kbRes.GET("/documents", kbScopeH.ListDocuments)
				kbRes.GET("/documents/graph", kbScopeH.DocumentGraph)
				kbRes.GET("/documents/:doc_id", kbScopeH.GetDocument)
				kbRes.DELETE("/documents/:doc_id", kbScopeH.DeleteDocument)
			}
		}

		v1.POST("/kb/types/ensure", kbPanelH.EnsureKBType)
		kb := v1.Group("/kb/:kb_type")
		{
			kb.GET("/context", workspaceCtxH.GetContext)
			kb.PUT("/context", workspaceCtxH.PutContext)
			kb.POST("/context/enrich", workspaceCtxH.EnrichContext)
			kb.GET("/documents", kbPanelH.ListDocuments)
			kb.GET("/documents/:doc_id", kbPanelH.GetDocument)
			kb.DELETE("/documents/:doc_id", kbPanelH.DeleteDocument)
			kb.GET("/workspace-data/stats", workspaceDataH.Stats)
			kb.POST("/workspace-data/purge", workspaceDataH.Purge)
		}

		admin := v1.Group("/admin")
		admin.Use(middleware.AdminAuth(cfg.Auth.JWTSecret))
		{
			admin.GET("/overview", adminH.Overview)
			admin.GET("/catalog", adminH.Catalog)
			admin.GET("/documents/stats", adminH.DocumentStats)
			admin.GET("/ugc/tasks", adminH.ListUGCTasks)
			admin.GET("/chunks", adminH.ListChunks)

			admin.POST("/global-index/refresh", adminOpsH.RefreshGlobalIndex)
			admin.GET("/global-index/stats", adminOpsH.GlobalIndexStats)
			admin.POST("/tier/apply", adminOpsH.ApplyColdTier)
			admin.GET("/tier/stats", adminOpsH.TierStats)
			admin.POST("/ragops/scan", adminOpsH.RAGOpsScan)
			admin.GET("/ragops/jobs", adminOpsH.RAGOpsListJobs)
			admin.POST("/ragops/jobs/:id/run", adminOpsH.RAGOpsRunJob)

			admin.GET("/dlq/stats", dlqH.Stats)
			admin.GET("/dlq/messages", dlqH.List)
			admin.POST("/dlq/replay", dlqH.Replay)
			admin.POST("/dlq/discard", dlqH.Discard)

			admin.GET("/agent-runs/:run_id/trace", adminAgentH.RunTrace)
			admin.GET("/cost/summary", adminCostH.Summary)
			admin.GET("/agent-evals", adminEvalH.List)
			admin.GET("/agent-evals/quality", adminEvalH.Quality)
			admin.POST("/agent-evals/run", adminEvalH.Run)
			admin.GET("/tools/approvals", adminApprovalsH.List)
			admin.POST("/tools/approvals/:run_id/:call_id/decide", adminApprovalsH.Decide)
		}

		authProtected := v1.Group("/auth")
		authProtected.Use(middleware.AdminAuth(cfg.Auth.JWTSecret))
		authProtected.GET("/me", authH.Me)
		authProtected.POST("/service-token", authH.IssueServiceToken)
	}
	v2 := r.Group("/api/v2")
	v2.Use(middleware.AgentAuth(cfg.Auth.JWTSecret))
	{
		v2.POST("/agent-runs", agentH.CreateRun)
		v2.GET("/agent-runs/:run_id/events", agentH.Events)
		v2.GET("/agent-runs/:run_id/events/stream", agentH.EventsStream)
		v2.GET("/agent-runs/:run_id/trace", agentH.Trace)
		v2.POST("/agent-runs/:run_id/cancel", agentH.Cancel)
		v2.GET("/conversations/:conversation_id/messages", agentMemoryH.Export)
		v2.DELETE("/conversations/:conversation_id/messages", agentMemoryH.Delete)
		v2.GET("/memories", longMemoryH.List)
		v2.DELETE("/memories/:memory_id", longMemoryH.Revoke)
		v2.POST("/memories/:memory_id/resolve", longMemoryH.Resolve)
		v2.GET("/threads", productThreadH.Get)
		v2.PUT("/threads", productThreadH.Put)
		v2.DELETE("/threads", productThreadH.Delete)
		v2.POST("/tools/publish", toolRegistryH.Publish)
		v2.POST("/tools/bind", toolRegistryH.Bind)
		v2.POST("/tools/:name/disable", toolRegistryH.Disable)
		v2.POST("/tools/approvals", toolGovernanceH.RequestApproval)
		v2.GET("/tools/approvals", toolGovernanceH.ListApprovals)
		v2.POST("/tools/approvals/:run_id/:call_id/decide", toolGovernanceH.Decide)
	}

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Info("graydraft-core listening", "addr", addr, "env", cfg.App.Env)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown", "error", err)
	}
	log.Info("graydraft-core stopped")
}

func runPeriodicAgentEval(ctx context.Context, svc *agent.EvalService, interval time.Duration, log *slog.Logger) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	run := func() {
		rec, err := svc.RunGolden(ctx, agent.RunGoldenRequest{
			PolicyVersion: os.Getenv("GRAYDRAFT_AGENT_CANARY_POLICY"),
			UseOracle:     os.Getenv("AGENT_EVAL_USE_ORACLE") == "1",
		})
		if err != nil {
			log.Warn("scheduled agent eval failed", "error", err)
			return
		}
		log.Info("scheduled agent eval completed", "eval_id", rec.EvalID, "pass_rate", rec.PassRate)
	}
	run()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}

func runGlobalIndexRefresh(ctx context.Context, repo *postgres.ChunkRepo, cfg *config.Config) {
	interval := time.Duration(cfg.GlobalIndex.AutoRefreshHours) * time.Hour
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			start := time.Now()
			if err := repo.RefreshGlobalIndex(ctx, cfg.GlobalIndex.RefreshConcurrent); err != nil {
				observability.RecordBackfill("global_index_refresh", "fail", time.Since(start))
				slog.Warn("global index auto refresh failed", "error", err)
			} else {
				observability.RecordBackfill("global_index_refresh", "ok", time.Since(start))
				slog.Info("global index refreshed", "interval_hours", cfg.GlobalIndex.AutoRefreshHours)
			}
		}
	}
}
