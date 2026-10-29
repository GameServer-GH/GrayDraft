package postgres

import (
	"context"
	"fmt"
	"time"
)

// CatalogRow is a kb catalog version row for admin APIs.
type CatalogRow struct {
	KBType             string    `json:"kb_type"`
	Version            int64     `json:"version"`
	LatestDocUpdatedAt time.Time `json:"latest_doc_updated_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// DocumentStat is per-kb_type document counts.
type DocumentStat struct {
	KBType      string `json:"kb_type"`
	TotalDocs   int64  `json:"total_docs"`
	ReadyDocs   int64  `json:"index_ready_docs"`
	TotalChunks int64  `json:"total_chunks"`
}

// ListCatalogVersions returns all kb catalog versions.
func (r *ChunkRepo) ListCatalogVersions(ctx context.Context) ([]CatalogRow, error) {
	rows, err := r.client.pool.Query(ctx, `
		SELECT kb_type, version, latest_doc_updated_at, updated_at
		FROM kb_catalog_version
		ORDER BY kb_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []CatalogRow
	for rows.Next() {
		var row CatalogRow
		if err := rows.Scan(&row.KBType, &row.Version, &row.LatestDocUpdatedAt, &row.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// DocumentStats returns document and chunk counts per kb_type.
func (r *ChunkRepo) DocumentStats(ctx context.Context) ([]DocumentStat, error) {
	rows, err := r.client.pool.Query(ctx, `
		SELECT d.kb_type,
		       COUNT(DISTINCT d.id) AS total_docs,
		       COUNT(DISTINCT d.id) FILTER (WHERE d.index_ready) AS ready_docs,
		       COUNT(c.id) AS total_chunks
		FROM kb_documents d
		LEFT JOIN kb_chunks c ON c.doc_id = d.id AND c.kb_type = d.kb_type
		GROUP BY d.kb_type
		ORDER BY d.kb_type
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []DocumentStat
	for rows.Next() {
		var s DocumentStat
		if err := rows.Scan(&s.KBType, &s.TotalDocs, &s.ReadyDocs, &s.TotalChunks); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}

// ListUGCTasks returns recent ingest tasks.
func (r *UGCRepo) ListTasks(ctx context.Context, limit int, status string) ([]map[string]any, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	q := `
		SELECT id::text, COALESCE(doc_id::text, ''), kb_type, COALESCE(title, ''), status,
		       COALESCE(error_message, ''), chunk_count, created_at, updated_at
		FROM ugc_ingest_tasks
	`
	var args []any
	if status != "" {
		q += ` WHERE status = $1 ORDER BY created_at DESC LIMIT $2`
		args = []any{status, limit}
	} else {
		q += ` ORDER BY created_at DESC LIMIT $1`
		args = []any{limit}
	}

	rows, err := r.client.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []map[string]any
	for rows.Next() {
		var id, docID, kbType, title, st, errMsg string
		var chunkCount int
		var createdAt, updatedAt time.Time
		if err := rows.Scan(&id, &docID, &kbType, &title, &st, &errMsg, &chunkCount, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		out = append(out, map[string]any{
			"task_id": id, "doc_id": docID, "kb_type": kbType, "title": title,
			"status": st, "error_message": errMsg, "chunk_count": chunkCount,
			"created_at": createdAt, "updated_at": updatedAt,
		})
	}
	if out == nil {
		out = []map[string]any{}
	}
	return out, rows.Err()
}

// UGCStatusCounts returns task counts grouped by status.
func (r *UGCRepo) UGCStatusCounts(ctx context.Context) (map[string]int64, error) {
	rows, err := r.client.pool.Query(ctx, `
		SELECT status, COUNT(*) FROM ugc_ingest_tasks GROUP BY status
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]int64)
	for rows.Next() {
		var status string
		var n int64
		if err := rows.Scan(&status, &n); err != nil {
			return nil, err
		}
		out[status] = n
	}
	return out, rows.Err()
}

// MarkTaskProcessing sets status to processing (used by worker; exported for consistency).
func (r *UGCRepo) MarkTaskProcessing(ctx context.Context, taskID string) error {
	tag, err := r.client.pool.Exec(ctx, `
		UPDATE ugc_ingest_tasks SET status = 'processing', updated_at = now() WHERE id = $1::uuid
	`, taskID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("task not found")
	}
	return nil
}
