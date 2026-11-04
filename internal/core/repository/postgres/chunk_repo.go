package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// ChunkRepo performs chunk retrieval against PostgreSQL.
type ChunkRepo struct {
	client                 *Client
	semantic4DStrict       bool
	semantic4DEntityStrict bool
}

// NewChunkRepo returns a chunk repository.
func NewChunkRepo(client *Client) *ChunkRepo {
	return &ChunkRepo{client: client}
}

// SetSemantic4DStrict enables strict 4D pre-filter (exclude docs without semantic_tags_4d).
func (r *ChunkRepo) SetSemantic4DStrict(strict bool) {
	if r != nil {
		r.semantic4DStrict = strict
	}
}

// SetSemantic4DEntityStrict requires docs with 4D entities to match query entities (legacy docs still pass).
func (r *ChunkRepo) SetSemantic4DEntityStrict(strict bool) {
	if r != nil {
		r.semantic4DEntityStrict = strict
	}
}

// CatalogVersion returns the kb catalog version for cache keys.
func (r *ChunkRepo) CatalogVersion(ctx context.Context, kbType string) (int64, error) {
	var ver int64
	err := r.client.pool.QueryRow(ctx, `
		SELECT version FROM kb_catalog_version WHERE kb_type = $1
	`, kbType).Scan(&ver)
	if err == pgx.ErrNoRows {
		return 0, nil
	}
	return ver, err
}

// SearchFulltext runs ts_rank + trigram fallback fulltext retrieval on search_text (3.5).
func (r *ChunkRepo) SearchFulltext(ctx context.Context, query, kbType string, limit int, includeCold bool, expandedTerms []string, sem4d *model.QuerySemantic4D) ([]model.Chunk, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}

	terms := extractSearchTerms(q)
	seen := make(map[string]struct{}, len(terms))
	for _, t := range terms {
		seen[strings.ToLower(t)] = struct{}{}
	}
	for _, t := range expandedTerms {
		t = strings.TrimSpace(t)
		if t == "" {
			continue
		}
		key := strings.ToLower(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		terms = append(terms, t)
	}

	st := chunkSearchTextExpr
	keywordClause := ""
	args := []any{q}
	argN := 2
	if len(terms) > 0 {
		keywordClause = fmt.Sprintf(`
			OR EXISTS (
				SELECT 1 FROM unnest($%d::text[]) AS kw
				WHERE length(kw) >= 2 AND %s ILIKE '%%' || kw || '%%'
			)`, argN, st)
		args = append(args, terms)
		argN++
	}

	sql := fmt.Sprintf(`
		SELECT
			c.id::text,
			c.doc_id::text,
			c.kb_type,
			c.content,
			c.chunk_index,
			c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			GREATEST(
				ts_rank(to_tsvector('simple', %s), plainto_tsquery('simple', $1)),
				similarity(%s, $1),
				CASE WHEN %s ILIKE '%%' || $1 || '%%' THEN 0.8 ELSE 0 END
			) AS score
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.index_ready = true
		  AND (
			to_tsvector('simple', %s) @@ plainto_tsquery('simple', $1)
			OR %s %% $1
			OR %s ILIKE '%%' || $1 || '%%'
			%s
		  )%s
	`, st, st, st, st, st, st, keywordClause, tierFilter(includeCold))

	if clause, cargs, next := semantic4DClauseForMode(argN, sem4d, r.semantic4DStrict, r.semantic4DEntityStrict); clause != "" {
		sql += clause
		args = append(args, cargs...)
		argN = next
	}
	if kbType != "" {
		sql += fmt.Sprintf(` AND c.kb_type = $%d`, argN)
		args = append(args, kbType)
		argN++
	}
	sql += fmt.Sprintf(` ORDER BY score DESC LIMIT $%d`, argN)
	args = append(args, limit)

	rows, err := r.client.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "fulltext")
}

// SearchVector runs cosine-distance vector retrieval (requires query embedding).
func (r *ChunkRepo) SearchVector(ctx context.Context, embedding []float32, kbType string, limit int, includeCold bool, sem4d *model.QuerySemantic4D) ([]model.Chunk, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	vec := pgvector.NewVector(embedding)

	sql := `
		SELECT
			c.id::text,
			c.doc_id::text,
			c.kb_type,
			c.content,
			c.chunk_index,
			c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			` + chunkExtensionsSelect + `,
			` + docExtensionsSelect + `,
			1 - (c.embedding <=> $1) AS score
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.index_ready = true
		  AND c.embedding IS NOT NULL
	` + tierFilter(includeCold)
	args := []any{vec}
	argN := 2
	if clause, cargs, next := semantic4DClauseForMode(argN, sem4d, r.semantic4DStrict, r.semantic4DEntityStrict); clause != "" {
		sql += clause
		args = append(args, cargs...)
		argN = next
	}
	if kbType != "" {
		sql += fmt.Sprintf(` AND c.kb_type = $%d`, argN)
		args = append(args, kbType)
		argN++
	}
	sql += fmt.Sprintf(` ORDER BY c.embedding <=> $1 LIMIT $%d`, argN)
	args = append(args, limit)

	rows, err := r.client.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "vector")
}

// SearchVectorStructured runs cosine retrieval on embedding_structured (4.0).
func (r *ChunkRepo) SearchVectorStructured(ctx context.Context, embedding []float32, kbType string, limit int, includeCold bool, sem4d *model.QuerySemantic4D) ([]model.Chunk, error) {
	if len(embedding) == 0 {
		return nil, nil
	}
	vec := pgvector.NewVector(embedding)

	sql := `
		SELECT
			c.id::text,
			c.doc_id::text,
			c.kb_type,
			c.content,
			c.chunk_index,
			c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			` + chunkExtensionsSelect + `,
			` + docExtensionsSelect + `,
			1 - (c.embedding_structured <=> $1) AS score
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.index_ready = true
		  AND c.embedding_structured IS NOT NULL
	` + tierFilter(includeCold)
	args := []any{vec}
	argN := 2
	if clause, cargs, next := semantic4DClauseForMode(argN, sem4d, r.semantic4DStrict, r.semantic4DEntityStrict); clause != "" {
		sql += clause
		args = append(args, cargs...)
		argN = next
	}
	if kbType != "" {
		sql += fmt.Sprintf(` AND c.kb_type = $%d`, argN)
		args = append(args, kbType)
		argN++
	}
	sql += fmt.Sprintf(` ORDER BY c.embedding_structured <=> $1 LIMIT $%d`, argN)
	args = append(args, limit)

	rows, err := r.client.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, fmt.Errorf("structured vector search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "structured")
}

// SearchGlobalIndex queries the cross-partition materialized view.
func (r *ChunkRepo) SearchGlobalIndex(ctx context.Context, query string, limit int, sem4d *model.QuerySemantic4D) ([]model.Chunk, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	args := []any{q, limit}
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
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			ts_rank(to_tsvector('simple', g.snippet), plainto_tsquery('simple', $1)) AS score
		FROM kb_global_index g
		JOIN kb_documents d ON d.id = g.doc_id
		JOIN kb_chunks c ON c.id = g.id AND c.kb_type = g.kb_type
		WHERE to_tsvector('simple', g.snippet) @@ plainto_tsquery('simple', $1)%s
		ORDER BY score DESC
		LIMIT $2
	`, semClause), args...)
	if err != nil {
		return nil, fmt.Errorf("global index search: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "global")
}

// AdminChunkRow is one row for Console chunk list (O-08).
type AdminChunkRow struct {
	ID           string  `json:"id"`
	DocID        string  `json:"doc_id"`
	KBType       string  `json:"kb_type"`
	DocTitle     string  `json:"doc_title"`
	ChunkIndex   int     `json:"chunk_index"`
	Preview      string  `json:"preview"`
	QualityScore float64 `json:"quality_score"`
	IndexReady   bool    `json:"index_ready"`
}

// ListAdminChunks returns paginated chunks for ops (O-08).
func (r *ChunkRepo) ListAdminChunks(ctx context.Context, kbType string, limit, offset int) ([]AdminChunkRow, int64, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	kbType = strings.TrimSpace(kbType)
	var total int64
	countQ := `SELECT COUNT(*) FROM kb_chunks c WHERE ($1 = '' OR c.kb_type = $1)`
	if err := r.client.pool.QueryRow(ctx, countQ, kbType).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT c.id::text, c.doc_id::text, c.kb_type, COALESCE(d.title, ''),
			c.chunk_index, LEFT(c.content, 200), c.quality_score, COALESCE(c.index_ready, false)
		FROM kb_chunks c
		LEFT JOIN kb_documents d ON d.id = c.doc_id
		WHERE ($1 = '' OR c.kb_type = $1)
		ORDER BY c.updated_at DESC NULLS LAST, c.id DESC
		LIMIT $2 OFFSET $3
	`, kbType, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var out []AdminChunkRow
	for rows.Next() {
		var row AdminChunkRow
		if err := rows.Scan(
			&row.ID, &row.DocID, &row.KBType, &row.DocTitle,
			&row.ChunkIndex, &row.Preview, &row.QualityScore, &row.IndexReady,
		); err != nil {
			return nil, 0, err
		}
		out = append(out, row)
	}
	return out, total, rows.Err()
}

// SearchSimilarDocuments finds docs by chunk embedding similarity (O-05).
func (r *ChunkRepo) SearchSimilarDocuments(ctx context.Context, kbType string, embedding []float32, limit int) ([]model.SimilarDocument, error) {
	if len(embedding) == 0 || strings.TrimSpace(kbType) == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	vec := pgvector.NewVector(embedding)
	rows, err := r.client.pool.Query(ctx, `
		SELECT d.id::text, COALESCE(d.title, ''), d.kb_type,
			MAX(1 - (c.embedding <=> $1)) AS sim
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.kb_type = $2 AND c.embedding IS NOT NULL
		GROUP BY d.id, d.title, d.kb_type
		ORDER BY sim DESC
		LIMIT $3
	`, vec, kbType, limit)
	if err != nil {
		return nil, fmt.Errorf("similar docs: %w", err)
	}
	defer rows.Close()
	var out []model.SimilarDocument
	for rows.Next() {
		var d model.SimilarDocument
		if err := rows.Scan(&d.DocID, &d.Title, &d.KBType, &d.Similarity); err != nil {
			return nil, err
		}
		if d.Similarity >= 0.72 {
			out = append(out, d)
		}
	}
	return out, rows.Err()
}

// Window returns the chunk, its section siblings, and explicit prev/next/parent links.
// Index neighbors are used only when the chunk has no section path.
func (r *ChunkRepo) Window(ctx context.Context, kbType, chunkID string) ([]model.Chunk, error) {
	rows, err := r.client.pool.Query(ctx, `
		WITH cur AS (
			SELECT id, doc_id, kb_type, chunk_index,
				COALESCE(metadata->>'section_path', '') AS section_path,
				NULLIF(metadata->>'prev_chunk_id', '') AS prev_id,
				NULLIF(metadata->>'next_chunk_id', '') AS next_id,
				NULLIF(metadata->>'parent_chunk_id', '') AS parent_id
			FROM kb_chunks
			WHERE id = $1::uuid AND ($2 = '' OR kb_type = $2)
		)
		SELECT
			c.id::text, c.doc_id::text, c.kb_type, c.content, c.chunk_index, c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			1.0 AS score
		FROM kb_chunks c
		JOIN cur ON c.doc_id = cur.doc_id AND c.kb_type = cur.kb_type
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.id = cur.id
			OR c.id::text = cur.prev_id
			OR c.id::text = cur.next_id
			OR c.id::text = cur.parent_id
			OR NULLIF(c.metadata->>'parent_chunk_id', '') = cur.id::text
			OR (
				cur.section_path <> ''
				AND COALESCE(c.metadata->>'section_path', '') = cur.section_path
			)
			OR (
				cur.section_path = ''
				AND c.chunk_index BETWEEN cur.chunk_index - 1 AND cur.chunk_index + 1
			)
		ORDER BY c.chunk_index
	`, chunkID, kbType)
	if err != nil {
		return nil, fmt.Errorf("chunk window: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "section")
}

// Expand returns chunks linked to an entity, then falls back to text match.
func (r *ChunkRepo) Expand(ctx context.Context, kbType, entity string, limit int) ([]model.Chunk, error) {
	entity = strings.TrimSpace(entity)
	if entity == "" {
		return nil, nil
	}
	if limit <= 0 {
		limit = 5
	}
	rows, err := r.client.pool.Query(ctx, `
		WITH hop1 AS (
			SELECT dst_chunk_id
			FROM edu_edges
			WHERE ($1 = '' OR kb_type = $1)
			  AND src_key ILIKE $2
		),
		via AS (
			SELECT DISTINCT e.src_key
			FROM edu_edges e
			WHERE e.dst_chunk_id IN (SELECT dst_chunk_id FROM hop1)
			  AND ($1 = '' OR e.kb_type = $1)
			  AND e.src_key NOT ILIKE $2
		),
		hop2 AS (
			SELECT e.dst_chunk_id
			FROM edu_edges e
			JOIN via ON e.src_key = via.src_key
			WHERE ($1 = '' OR e.kb_type = $1)
		),
		hits AS (
			SELECT dst_chunk_id FROM hop1
			UNION
			SELECT dst_chunk_id FROM hop2
		)
		SELECT
			c.id::text, c.doc_id::text, c.kb_type, c.content, c.chunk_index, c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			1.0 AS score
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE c.index_ready = true
		  AND ($1 = '' OR c.kb_type = $1)
		  AND c.id IN (SELECT dst_chunk_id FROM hits)
		ORDER BY c.quality_score DESC, c.chunk_index
		LIMIT $3
	`, kbType, entity, limit)
	if err != nil {
		return nil, fmt.Errorf("entity expand: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "expand")
}

// GroupSpan returns chunks from the anchor document and the next two slices that
// share its title group. Slice markers such as （3/14） are not a separate work.
// Query tokens that already appear in the group title are ignored so the paper
// name does not crowd out the later section that holds the figure.
func (r *ChunkRepo) GroupSpan(ctx context.Context, kbType, chunkID, query string, limit int) ([]model.Chunk, error) {
	if limit <= 0 || limit > 12 {
		limit = 8
	}
	rows, err := r.client.pool.Query(ctx, `
		WITH cur AS (
			SELECT c.id, c.doc_id, c.kb_type, d.title AS doc_title, d.extensions->>'source_revision_sha256' AS source_revision,
				btrim(regexp_replace(regexp_replace(d.title, '^论文[[:space:]]*', ''), '[（(][0-9]+/[0-9]+[）)]', '', 'g')) AS g
			FROM kb_chunks c
			JOIN kb_documents d ON d.id = c.doc_id
			WHERE c.id = $1::uuid AND ($2 = '' OR c.kb_type = $2)
		)
		SELECT
			c.id::text, c.doc_id::text, c.kb_type, c.content, c.chunk_index, c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			1.0 AS score,
			cur.doc_title AS anchor_title,
			cur.id::text AS anchor_id
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		JOIN cur ON c.kb_type = cur.kb_type
			AND ((COALESCE(cur.source_revision, '') <> '' AND d.extensions->>'source_revision_sha256' = cur.source_revision)
                OR (COALESCE(cur.source_revision, '') = '' AND btrim(regexp_replace(regexp_replace(d.title, '^论文[[:space:]]*', ''), '[（(][0-9]+/[0-9]+[）)]', '', 'g')) = cur.g))
		ORDER BY d.title, d.id, c.chunk_index
	`, chunkID, kbType)
	if err != nil {
		return nil, fmt.Errorf("group span: %w", err)
	}
	defer rows.Close()
	var all []model.Chunk
	var anchorTitle, anchorID string
	for rows.Next() {
		var c model.Chunk
		var extRaw, docExtRaw []byte
		var aTitle, aID string
		if err := rows.Scan(
			&c.ID, &c.DocID, &c.KBType, &c.Content,
			&c.ChunkIndex, &c.QualityScore, &c.DocTitle, &extRaw, &docExtRaw, &c.Score,
			&aTitle, &aID,
		); err != nil {
			return nil, fmt.Errorf("group span scan: %w", err)
		}
		c.Extensions = parseExtensions(extRaw)
		c.DocExtensions = parseExtensions(docExtRaw)
		c.Source = "group"
		if anchorID == "" {
			anchorTitle, anchorID = aTitle, aID
		}
		all = append(all, c)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return pickGroupSpan(all, anchorTitle, anchorID, query, limit), nil
}

// GroupOpenings returns the first body chunk of the first slice in each title group.
func (r *ChunkRepo) GroupOpenings(ctx context.Context, kbType string, groups []string) ([]model.Chunk, error) {
	if len(groups) == 0 {
		return nil, nil
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT
			c.id::text, c.doc_id::text, c.kb_type, c.content, c.chunk_index, c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			1.0 AS score
		FROM kb_documents d
        JOIN LATERAL (
            SELECT x.* FROM kb_chunks x
            WHERE x.doc_id=d.id AND x.kb_type=d.kb_type AND x.chunk_index>=0
            ORDER BY x.chunk_index,x.id LIMIT 4
        ) c ON TRUE
        WHERE ($1 = '' OR d.kb_type = $1)
		ORDER BY d.title, c.chunk_index
	`, kbType)
	if err != nil {
		return nil, fmt.Errorf("group openings: %w", err)
	}
	defer rows.Close()
	all, err := scanChunks(rows, "opening")
	if err != nil {
		return nil, err
	}
	return pickGroupOpenings(all, groups), nil
}

// Openings returns the first body chunk of each document (skipping the slice banner).
func (r *ChunkRepo) Openings(ctx context.Context, kbType string, docIDs []string) ([]model.Chunk, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT DISTINCT ON (d.id)
			c.id::text, c.doc_id::text, c.kb_type, c.content, c.chunk_index, c.quality_score,
			COALESCE(d.title, '') AS doc_title,
			`+chunkExtensionsSelect+`,
			`+docExtensionsSelect+`,
			1.0 AS score
		FROM kb_chunks c
		JOIN kb_documents d ON d.id = c.doc_id
		WHERE ($1 = '' OR c.kb_type = $1)
		  AND d.id::text = ANY($2)
		  AND c.chunk_index >= 0
		ORDER BY d.id, c.chunk_index
	`, kbType, docIDs)
	if err != nil {
		return nil, fmt.Errorf("openings: %w", err)
	}
	defer rows.Close()
	return scanChunks(rows, "opening")
}

func scanChunks(rows pgx.Rows, source string) ([]model.Chunk, error) {
	var out []model.Chunk
	for rows.Next() {
		var c model.Chunk
		var extRaw, docExtRaw []byte
		if err := rows.Scan(
			&c.ID, &c.DocID, &c.KBType, &c.Content,
			&c.ChunkIndex, &c.QualityScore, &c.DocTitle, &extRaw, &docExtRaw, &c.Score,
		); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		c.Extensions = parseExtensions(extRaw)
		c.DocExtensions = parseExtensions(docExtRaw)
		c.Source = source
		out = append(out, c)
	}
	return out, rows.Err()
}
