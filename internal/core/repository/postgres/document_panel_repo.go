package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"
)

// PanelDocumentListItem is a row for GET /api/v1/kb/{kb_type}/documents.
type PanelDocumentListItem struct {
	ID           string          `json:"id"`
	Title        string          `json:"title"`
	Preview      string          `json:"preview"`
	IndexReady   bool            `json:"index_ready"`
	QualityScore float64         `json:"quality_score"`
	ChunkCount   int             `json:"chunk_count"`
	Extensions   json.RawMessage `json:"extensions"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// PanelChunkRow is a chunk in document detail.
type PanelChunkRow struct {
	ID           string  `json:"id"`
	ChunkIndex   int     `json:"chunk_index"`
	Content      string  `json:"content"`
	IndexReady   bool    `json:"index_ready"`
	QualityScore float64 `json:"quality_score"`
}

// PanelQARow is a QA pair in document detail.
type PanelQARow struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

// PanelDocumentDetail is GET /api/v1/kb/{kb_type}/documents/{doc_id}.
type PanelDocumentDetail struct {
	ID           string          `json:"id"`
	KBType       string          `json:"kb_type"`
	Title        string          `json:"title"`
	Content      string          `json:"content"`
	SourceURL    string          `json:"source_url,omitempty"`
	IndexReady   bool            `json:"index_ready"`
	QualityScore float64         `json:"quality_score"`
	KBVersion    int64           `json:"kb_version"`
	Extensions   json.RawMessage `json:"extensions"`
	Chunks       []PanelChunkRow `json:"chunks"`
	QAPairs      []PanelQARow    `json:"qa_pairs"`
	UpdatedAt    time.Time       `json:"updated_at"`
}

// EnsureKBType registers a kb_type if missing (idempotent).
func (r *DocumentRepo) EnsureKBType(ctx context.Context, name, description string) error {
	if name == "" {
		return fmt.Errorf("kb_type name required")
	}
	_, err := r.client.pool.Exec(ctx, `
		INSERT INTO kb_types (name, description)
		VALUES ($1, $2)
		ON CONFLICT (name) DO NOTHING
	`, name, description)
	return err
}

// ListPanelDocuments returns paginated documents for a kb partition.
func (r *DocumentRepo) ListPanelDocuments(ctx context.Context, kbType string, limit, offset int, indexReady *bool) ([]PanelDocumentListItem, int64, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	countSQL := `SELECT COUNT(*) FROM kb_documents WHERE kb_type = $1`
	countArgs := []any{kbType}
	if indexReady != nil {
		countSQL += ` AND index_ready = $2`
		countArgs = append(countArgs, *indexReady)
	}
	var total int64
	if err := r.client.pool.QueryRow(ctx, countSQL, countArgs...).Scan(&total); err != nil {
		return nil, 0, err
	}

	listSQL := `
		SELECT d.id::text,
		       COALESCE(d.title, ''),
		       d.index_ready,
		       d.quality_score,
		       d.updated_at,
		       COUNT(c.id)::int AS chunk_count,
		       COALESCE((
		         SELECT left(c2.content, 120)
		         FROM kb_chunks c2
		         WHERE c2.doc_id = d.id AND c2.kb_type = d.kb_type
		         ORDER BY c2.chunk_index
		         LIMIT 1
		       ), '') AS preview,
		       COALESCE(d.extensions, '{}'::jsonb)
		FROM kb_documents d
		LEFT JOIN kb_chunks c ON c.doc_id = d.id AND c.kb_type = d.kb_type
		WHERE d.kb_type = $1
	`
	listArgs := []any{kbType}
	argN := 2
	if indexReady != nil {
		listSQL += fmt.Sprintf(` AND d.index_ready = $%d`, argN)
		listArgs = append(listArgs, *indexReady)
		argN++
	}
	listSQL += fmt.Sprintf(`
		GROUP BY d.id, d.title, d.index_ready, d.quality_score, d.updated_at, d.extensions
		ORDER BY d.updated_at DESC
		LIMIT $%d OFFSET $%d`, argN, argN+1)
	listArgs = append(listArgs, limit, offset)

	rows, err := r.client.pool.Query(ctx, listSQL, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []PanelDocumentListItem
	for rows.Next() {
		var it PanelDocumentListItem
		var ext []byte
		if err := rows.Scan(&it.ID, &it.Title, &it.IndexReady, &it.QualityScore, &it.UpdatedAt, &it.ChunkCount, &it.Preview, &ext); err != nil {
			return nil, 0, err
		}
		it.Extensions = json.RawMessage(ext)
		items = append(items, it)
	}
	if items == nil {
		items = []PanelDocumentListItem{}
	}
	return items, total, rows.Err()
}

// GetPanelDocument loads one document with chunks and QA pairs.
func (r *DocumentRepo) GetPanelDocument(ctx context.Context, kbType, docID string) (*PanelDocumentDetail, error) {
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		return nil, fmt.Errorf("invalid doc_id: %w", err)
	}

	row := r.client.pool.QueryRow(ctx, `
		SELECT id::text, kb_type, COALESCE(title, ''), COALESCE(source_url, ''),
		       index_ready, quality_score, kb_version,
		       COALESCE(extensions, '{}'::jsonb), updated_at
		FROM kb_documents
		WHERE id = $1 AND kb_type = $2
	`, docUUID, kbType)

	var d PanelDocumentDetail
	var ext []byte
	if err := row.Scan(&d.ID, &d.KBType, &d.Title, &d.SourceURL, &d.IndexReady, &d.QualityScore,
		&d.KBVersion, &ext, &d.UpdatedAt); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	d.Extensions = json.RawMessage(ext)

	chunkRows, err := r.client.pool.Query(ctx, `
		SELECT id::text, chunk_index, content, index_ready, quality_score
		FROM kb_chunks
		WHERE doc_id = $1 AND kb_type = $2
		ORDER BY chunk_index
	`, docUUID, kbType)
	if err != nil {
		return nil, err
	}
	defer chunkRows.Close()

	var parts []string
	for chunkRows.Next() {
		var c PanelChunkRow
		if err := chunkRows.Scan(&c.ID, &c.ChunkIndex, &c.Content, &c.IndexReady, &c.QualityScore); err != nil {
			return nil, err
		}
		d.Chunks = append(d.Chunks, c)
		parts = append(parts, c.Content)
	}
	if err := chunkRows.Err(); err != nil {
		return nil, err
	}
	if d.Chunks == nil {
		d.Chunks = []PanelChunkRow{}
	}
	d.Content = joinChunkParts(parts)

	qaRows, err := r.client.pool.Query(ctx, `
		SELECT question, answer
		FROM kb_qa_pairs
		WHERE doc_id = $1 AND kb_type = $2
		ORDER BY created_at
	`, docUUID, kbType)
	if err != nil {
		return nil, err
	}
	defer qaRows.Close()

	for qaRows.Next() {
		var q PanelQARow
		if err := qaRows.Scan(&q.Question, &q.Answer); err != nil {
			return nil, err
		}
		d.QAPairs = append(d.QAPairs, q)
	}
	if d.QAPairs == nil {
		d.QAPairs = []PanelQARow{}
	}
	if err := qaRows.Err(); err != nil {
		return nil, err
	}
	return &d, nil
}

// CountPanelDocuments returns document count for a kb_type partition.
func (r *DocumentRepo) CountPanelDocuments(ctx context.Context, kbType string) (int64, error) {
	var n int64
	err := r.client.pool.QueryRow(ctx, `
		SELECT COUNT(*)::bigint FROM kb_documents WHERE kb_type = $1
	`, kbType).Scan(&n)
	return n, err
}

// PurgePanelDocuments deletes all documents for kb_type (cascades chunks/QA) and bumps catalog.
func (r *DocumentRepo) PurgePanelDocuments(ctx context.Context, kbType string) (int64, error) {
	tx, err := r.client.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `DELETE FROM kb_documents WHERE kb_type = $1`, kbType)
	if err != nil {
		return 0, err
	}
	bumpCatalogVersionTx(ctx, tx, kbType)
	if err := tx.Commit(ctx); err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}

// DeletePanelDocument removes a document (cascades chunks/QA) and bumps catalog version.
func (r *DocumentRepo) DeletePanelDocument(ctx context.Context, kbType, docID string) (bool, error) {
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		return false, fmt.Errorf("invalid doc_id: %w", err)
	}

	tx, err := r.client.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tag, err := tx.Exec(ctx, `
		DELETE FROM kb_documents WHERE id = $1 AND kb_type = $2
	`, docUUID, kbType)
	if err != nil {
		return false, err
	}
	if tag.RowsAffected() == 0 {
		return false, nil
	}
	bumpCatalogVersionTx(ctx, tx, kbType)
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

// PanelGraphNode is one document with a representative chunk embedding for galaxy layout.
type PanelGraphNode struct {
	ID         string          `json:"id"`
	Title      string          `json:"title"`
	Extensions json.RawMessage `json:"extensions"`
	Embedding  []float32       `json:"embedding,omitempty"`
}

// ListPanelDocumentGraph returns indexed documents with first-chunk embeddings for semantic galaxy UI.
func (r *DocumentRepo) ListPanelDocumentGraph(ctx context.Context, kbType string, limit int) ([]PanelGraphNode, error) {
	if limit <= 0 || limit > 200 {
		limit = 120
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT d.id::text,
		       COALESCE(d.title, ''),
		       COALESCE(d.extensions, '{}'::jsonb),
		       sub.embedding
		FROM kb_documents d
		INNER JOIN LATERAL (
			SELECT c.embedding
			FROM kb_chunks c
			WHERE c.doc_id = d.id AND c.kb_type = d.kb_type
			  AND c.index_ready = true AND c.embedding IS NOT NULL
			ORDER BY c.chunk_index ASC
			LIMIT 1
		) sub ON true
		WHERE d.kb_type = $1 AND d.index_ready = true
		ORDER BY d.updated_at DESC
		LIMIT $2
	`, kbType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []PanelGraphNode
	for rows.Next() {
		var it PanelGraphNode
		var ext []byte
		var emb pgvector.Vector
		if err := rows.Scan(&it.ID, &it.Title, &ext, &emb); err != nil {
			return nil, err
		}
		it.Extensions = json.RawMessage(ext)
		if s := emb.Slice(); len(s) > 0 {
			it.Embedding = s
		}
		items = append(items, it)
	}
	if items == nil {
		items = []PanelGraphNode{}
	}
	return items, rows.Err()
}

func joinChunkParts(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	if len(parts) == 1 {
		return parts[0]
	}
	out := parts[0]
	for _, p := range parts[1:] {
		out += "\n\n" + p
	}
	return out
}
