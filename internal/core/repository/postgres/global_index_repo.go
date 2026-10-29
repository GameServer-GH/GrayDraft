package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/graydraft/core/model"
	"github.com/pgvector/pgvector-go"
)

// GlobalIndexStats summarizes the materialized view.
type GlobalIndexStats struct {
	RowCount    int64     `json:"row_count"`
	WithVector  int64     `json:"with_vector"`
	LastRefresh time.Time `json:"last_refresh,omitempty"`
}

// RefreshGlobalIndex rebuilds kb_global_index (O-17).
func (r *ChunkRepo) RefreshGlobalIndex(ctx context.Context, concurrent bool) error {
	sql := `REFRESH MATERIALIZED VIEW kb_global_index`
	if concurrent {
		sql = `REFRESH MATERIALIZED VIEW CONCURRENTLY kb_global_index`
	}
	_, err := r.client.pool.Exec(ctx, sql)
	return err
}

// GlobalIndexStats returns row counts for the materialized view.
func (r *ChunkRepo) GlobalIndexStats(ctx context.Context) (GlobalIndexStats, error) {
	var s GlobalIndexStats
	err := r.client.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint,
		       COUNT(*) FILTER (WHERE embedding IS NOT NULL)::bigint
		FROM kb_global_index
	`).Scan(&s.RowCount, &s.WithVector)
	if err != nil {
		return s, err
	}
	_ = r.client.pool.QueryRow(ctx, `
		SELECT COALESCE(MAX(c.updated_at), now())
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.index_ready = true AND d.index_ready = true AND d.storage_tier = 'hot'
	`).Scan(&s.LastRefresh)
	return s, nil
}

// SearchGlobalVector queries kb_global_index by embedding (O-17).
func (r *ChunkRepo) SearchGlobalVector(ctx context.Context, embedding []float32, limit int, sem4d *model.QuerySemantic4D) ([]model.Chunk, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	vec := pgvector.NewVector(embedding)
	args := []any{vec, limit}
	argN := 3
	semClause := ""
	if clause, cargs, next := semantic4DClauseForMode(argN, sem4d, r.semantic4DStrict, r.semantic4DEntityStrict); clause != "" {
		semClause = clause
		args = append(args, cargs...)
		argN = next
	}
	_ = argN
	rows, err := r.client.pool.Query(ctx, fmt.Sprintf(`
		SELECT
			g.id::text,
			g.doc_id::text,
			g.kb_type,
			g.snippet AS content,
			0 AS chunk_index,
			g.quality_score,
			COALESCE(g.doc_title, '') AS doc_title,
			` + chunkExtensionsSelect + `,
			1 - (g.embedding <=> $1) AS score
		FROM kb_global_index g
		JOIN kb_documents d ON d.id = g.doc_id
		JOIN kb_chunks c ON c.id = g.id AND c.kb_type = g.kb_type
		WHERE g.embedding IS NOT NULL%s
		ORDER BY g.embedding <=> $1
		LIMIT $2
	`, semClause), args...)
	if err != nil {
		return nil, fmt.Errorf("global vector search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "global_vector")
}

// SearchGlobalHybrid fuses global fulltext + vector (O-17).
func (r *ChunkRepo) SearchGlobalHybrid(
	ctx context.Context,
	query string,
	embedding []float32,
	limit int,
	sem4d *model.QuerySemantic4D,
) ([]model.Chunk, error) {
	ft, err := r.SearchGlobalIndex(ctx, query, limit, sem4d)
	if err != nil {
		return nil, err
	}
	if len(embedding) == 0 {
		return ft, nil
	}
	vec, err := r.SearchGlobalVector(ctx, embedding, limit, sem4d)
	if err != nil {
		return ft, nil
	}
	return fuseGlobalRRF(60, ft, vec), nil
}

func fuseGlobalRRF(constK int, lists ...[]model.Chunk) []model.Chunk {
	scores := make(map[string]float64)
	chunks := make(map[string]model.Chunk)
	for li, list := range lists {
		w := 1.0
		if li == 1 {
			w = 1.2
		}
		for i, c := range list {
			key := c.ID + ":" + c.KBType
			scores[key] += w / float64(constK+i+1)
			if _, ok := chunks[key]; !ok {
				chunks[key] = c
			}
		}
	}
	type ranked struct {
		key   string
		score float64
	}
	var order []ranked
	for k, s := range scores {
		order = append(order, ranked{k, s})
	}
	for i := 0; i < len(order); i++ {
		for j := i + 1; j < len(order); j++ {
			if order[j].score > order[i].score {
				order[i], order[j] = order[j], order[i]
			}
		}
	}
	out := make([]model.Chunk, 0, len(order))
	for _, r := range order {
		c := chunks[r.key]
		c.Score = r.score
		c.Source = "global_hybrid"
		out = append(out, c)
	}
	return out
}

// TierStats is hot/cold document counts (O-18).
type TierStats struct {
	HotDocs  int64 `json:"hot_docs"`
	ColdDocs int64 `json:"cold_docs"`
}

// TierStats returns document counts by storage tier.
func (r *ChunkRepo) TierStats(ctx context.Context) (TierStats, error) {
	var s TierStats
	err := r.client.pool.QueryRow(ctx, `
		SELECT
			COUNT(*) FILTER (WHERE storage_tier = 'hot')::bigint,
			COUNT(*) FILTER (WHERE storage_tier = 'cold')::bigint
		FROM kb_documents
	`).Scan(&s.HotDocs, &s.ColdDocs)
	return s, err
}

// ApplyColdTier marks stale hot documents as cold (O-18).
func (r *ChunkRepo) ApplyColdTier(ctx context.Context, coldAfterDays int) (int64, error) {
	if coldAfterDays <= 0 {
		coldAfterDays = 90
	}
	tag, err := r.client.pool.Exec(ctx, `
		UPDATE kb_documents
		SET storage_tier = 'cold', updated_at = now()
		WHERE storage_tier = 'hot'
		  AND (
		    last_accessed_at IS NULL
		    OR last_accessed_at < now() - ($1 || ' days')::interval
		  )
	`, fmt.Sprint(coldAfterDays))
	if err != nil {
		return 0, err
	}
	if tag.RowsAffected() > 0 {
		_ = r.RefreshGlobalIndex(ctx, true)
	}
	return tag.RowsAffected(), nil
}

// WarmDocuments sets storage_tier=hot and bumps last_accessed_at (O-18).
func (r *ChunkRepo) WarmDocuments(ctx context.Context, docIDs []uuid.UUID) error {
	if len(docIDs) == 0 {
		return nil
	}
	_, err := r.client.pool.Exec(ctx, `
		UPDATE kb_documents
		SET storage_tier = 'hot', last_accessed_at = now(), updated_at = now()
		WHERE id = ANY($1::uuid[])
	`, docIDs)
	return err
}

// TouchDocumentAccess updates last_accessed_at for retrieved docs (O-18).
func (r *ChunkRepo) TouchDocumentAccess(ctx context.Context, docIDs []string) {
	ids, err := parseUUIDs(docIDs)
	if err != nil || len(ids) == 0 {
		return
	}
	_, _ = r.client.pool.Exec(ctx, `
		UPDATE kb_documents
		SET last_accessed_at = now()
		WHERE id = ANY($1::uuid[])
	`, ids)
}

func tierFilter(includeCold bool) string {
	if includeCold {
		return ""
	}
	return ` AND d.storage_tier = 'hot'`
}
