package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/console-backend/config"
	"github.com/graydraft/console-backend/handler"
)

func main() {
	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "config/config.dev.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal("load config:", err)
	}

	if cfg.App.Env == "production" {
		gin.SetMode(gin.ReleaseMode)
	}

	proxy := handler.NewCoreProxy(cfg.Core.URL)
	r := gin.New()
	r.Use(gin.Recovery())

	// Proxy all Core API calls (admin, health, RAG, UGC).
	r.Any("/api/*path", proxy.Forward)
	r.Any("/health", proxy.Forward)

	staticDir := cfg.Static.Dir
	if _, err := os.Stat(staticDir); err == nil {
		r.Static("/assets", filepath.Join(staticDir, "assets"))
		r.GET("/", func(c *gin.Context) {
			c.File(filepath.Join(staticDir, "index.html"))
		})
		r.NoRoute(func(c *gin.Context) {
			if c.Request.Method != http.MethodGet {
				c.Status(http.StatusNotFound)
				return
			}
			c.File(filepath.Join(staticDir, "index.html"))
		})
	} else {
		r.GET("/", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"service": "graydraft-console",
				"message": "frontend not built; run npm run build in internal/console/frontend",
				"core_url": cfg.Core.URL,
			})
		})
	}

	addr := fmt.Sprintf(":%d", cfg.HTTP.Port)
	log.Printf("graydraft-console listening on %s (core=%s)", addr, cfg.Core.URL)
	if err := r.Run(addr); err != nil {
		log.Fatal(err)
	}
}
