// Command backfill-semantic4d fills semantic_tags_4d + global_summary on documents missing them.
//
// Usage:
//
//	CONFIG_PATH=internal/core/config/config.dev.yaml go run ./internal/core/cmd/backfill-semantic4d [-kb_type=tutorial] [-limit=50] [-purge]
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/repository/postgres"
	"github.com/graydraft/core/service/answercache"
	"github.com/graydraft/core/service/cache"
	"github.com/graydraft/core/service/llmclient"
	"github.com/graydraft/core/service/smartimport"
)

var purge = flag.Bool("purge", false, "purge retrieval and answer caches after backfill")

func main() {
	kbType := flag.String("kb_type", "", "optional kb_type filter")
	limit := flag.Int("limit", 50, "max documents to process")
	flag.Parse()

	cfgPath := os.Getenv("CONFIG_PATH")
	if cfgPath == "" {
		cfgPath = "internal/core/config/config.dev.yaml"
	}
	cfg, err := config.Load(cfgPath)
	if err != nil {
		log.Fatal(err)
	}
	ctx := context.Background()
	db, err := postgres.New(ctx, cfg.DSN())
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	llm := llmclient.NewClient(cfg)
	complete := func(prompt string) (string, error) { return llm.Complete(ctx, prompt) }

	rows, err := db.Pool().Query(ctx, `
		SELECT d.id::text, d.kb_type, COALESCE(d.title,''), COALESCE(d.extensions,'{}'::jsonb)
		FROM kb_documents d
		WHERE ($1 = '' OR d.kb_type = $1)
		  AND (
		    NOT (d.extensions ? 'semantic_tags_4d')
		    OR COALESCE(d.extensions->'semantic_tags_4d', '{}'::jsonb) = '{}'::jsonb
		  )
		ORDER BY d.updated_at DESC
		LIMIT $2
	`, *kbType, *limit)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var ok, fail int
	for rows.Next() {
		var id, kb, title string
		var extJSON []byte
		if err := rows.Scan(&id, &kb, &title, &extJSON); err != nil {
			log.Fatal(err)
		}
		body, err := loadDocBody(ctx, db, id, kb)
		if err != nil || strings.TrimSpace(body) == "" {
			log.Printf("skip %s: no body", id)
			fail++
			continue
		}
		pivot, err := smartimport.RunSemanticPivot(body, "", complete)
		if err != nil {
			log.Printf("fail %s: %v", id, err)
			fail++
			continue
		}
		var ext map[string]any
		_ = json.Unmarshal(extJSON, &ext)
		if ext == nil {
			ext = map[string]any{}
		}
		if pivot.Tags4D.Domain != "" || len(pivot.Tags4D.Entities) > 0 || pivot.Tags4D.Scene != "" || pivot.Tags4D.Style != "" {
			ext["semantic_tags_4d"] = map[string]any{
				"domain": pivot.Tags4D.Domain, "entities": pivot.Tags4D.Entities,
				"scene": pivot.Tags4D.Scene, "style": pivot.Tags4D.Style,
			}
		}
		if pivot.GlobalSummary != "" {
			ext["global_summary"] = pivot.GlobalSummary
		}
		ext["semantic_backfill"] = "pivot_llm"
		b, _ := json.Marshal(ext)
		_, err = db.Pool().Exec(ctx, `
			UPDATE kb_documents SET extensions = $1::jsonb, updated_at = now() WHERE id = $2::uuid AND kb_type = $3
		`, b, id, kb)
		if err != nil {
			log.Printf("update %s: %v", id, err)
			fail++
			continue
		}
		ok++
		log.Printf("ok %s %q", id, title)
	}
	fmt.Printf("done ok=%d fail=%d\n", ok, fail)

	if *purge && ok > 0 {
		purgeCaches(ctx, cfg, *kbType)
	}
}

func purgeCaches(ctx context.Context, cfg *config.Config, kbType string) {
	if kbType == "" {
		fmt.Println("skip purge: no kb_type specified")
		return
	}
	cacheMgr, err := cache.NewManager(cfg)
	if err != nil || cacheMgr == nil {
		fmt.Printf("skip retrieval cache purge: %v\n", err)
	} else {
		n, err := cacheMgr.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			fmt.Printf("retrieval cache purge failed: %v\n", err)
		} else {
			fmt.Printf("retrieval cache purged kb_type=%s removed=%d\n", kbType, n)
		}
	}
	answerCache, err := answercache.NewManager(cfg)
	if err != nil || answerCache == nil {
		fmt.Printf("skip answer cache purge: %v\n", err)
	} else {
		n, err := answerCache.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			fmt.Printf("answer cache purge failed: %v\n", err)
		} else {
			fmt.Printf("answer cache purged kb_type=%s removed=%d\n", kbType, n)
		}
	}
}

func loadDocBody(ctx context.Context, db *postgres.Client, docID, kbType string) (string, error) {
	var parts []string
	rows, err := db.Pool().Query(ctx, `
		SELECT content FROM kb_chunks
		WHERE doc_id = $1::uuid AND kb_type = $2
		ORDER BY chunk_index
	`, docID, kbType)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return "", err
		}
		parts = append(parts, c)
	}
	return strings.Join(parts, "\n\n"), rows.Err()
}
