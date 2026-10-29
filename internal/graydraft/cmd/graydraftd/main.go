package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/graydraft/graydraft/internal/server"
)

func main() {
	addr := flag.String("addr", ":8787", "HTTP listen address")
	data := flag.String("data", "", "workspace data directory")
	flag.Parse()

	dataDir := *data
	if dataDir == "" {
		dataDir = os.Getenv("GRAYDRAFT_DATA_DIR")
	}
	if dataDir == "" {
		dataDir = "data"
	}
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("create data dir", "error", err, "path", dataDir)
		os.Exit(1)
	}

	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	app := server.NewApp(log, dataDir)
	srv := &http.Server{Addr: *addr, Handler: app.Handler()}

	go func() {
		log.Info("graydraftd listening", "addr", *addr, "data", dataDir)
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
	log.Info("graydraftd stopped")
}
