package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/gateway/config"
	"github.com/graydraft/gateway/handler"
	"github.com/graydraft/gateway/observability"
	"github.com/graydraft/gateway/service/embedder"
	"github.com/graydraft/gateway/service/llm"
	"github.com/graydraft/gateway/service/rerank"
	"github.com/graydraft/gocommon/logger"
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

	log := logger.New(os.Getenv("LOG_LEVEL"))
	embedSvc := embedder.New(cfg)
	embedH := handler.NewEmbeddingsHandler(embedSvc)
	chatSvc := llm.New(cfg)
	chatH := handler.NewChatHandler(chatSvc)
	rrSvc := rerank.New(cfg)
	rrH := handler.NewRerankHandler(rrSvc)
	healthH := handler.NewHealthHandler(cfg)

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Recovery(), observability.Middleware())
	r.GET("/metrics", observability.Handler())
	r.GET("/health", healthH.Health)
	r.GET("/api/health", healthH.Health)
	r.POST("/v1/embeddings", embedH.Embed)
	r.POST("/v1/embeddings/batch", embedH.EmbedBatch)
	r.GET("/v1/embeddings/status", embedH.EmbeddingStatus)
	r.POST("/v1/chat/completions", chatH.Completions)
	r.POST("/v1/rerank", rrH.Rerank)

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Info("graydraft-gateway listening", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Info("graydraft-gateway stopped")
}
