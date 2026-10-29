package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// DocumentRow is a kb_documents row used for dedup / smart import.
type DocumentRow struct {
	ID           string
	KBType       string
	Title        string
	ContentHash  string
	KBVersion    int64
	IndexReady   bool
}

// DocumentRepo reads document metadata for import dedup.
type DocumentRepo struct {
	client *Client
}

// NewDocumentRepo creates a document metadata repository.
func NewDocumentRepo(client *Client) *DocumentRepo {
	return &DocumentRepo{client: client}
}

// FindByContentHash returns the latest document with the same content hash in a kb partition.
func (r *DocumentRepo) FindByContentHash(ctx context.Context, kbType, contentHash string) (*DocumentRow, error) {
	if contentHash == "" {
		return nil, nil
	}
	row := r.client.pool.QueryRow(ctx, `
		SELECT id::text, kb_type, COALESCE(title, ''), COALESCE(content_hash, ''), kb_version, index_ready
		FROM kb_documents
		WHERE kb_type = $1 AND content_hash = $2
		ORDER BY updated_at DESC
		LIMIT 1
	`, kbType, contentHash)

	var doc DocumentRow
	if err := row.Scan(&doc.ID, &doc.KBType, &doc.Title, &doc.ContentHash, &doc.KBVersion, &doc.IndexReady); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find by content_hash: %w", err)
	}
	return &doc, nil
}

// FindByTitle returns a document with the same normalized title in a kb partition.
func (r *DocumentRepo) FindByTitle(ctx context.Context, kbType, title string) (*DocumentRow, error) {
	title = normalizeDocTitle(title)
	if title == "" {
		return nil, nil
	}
	row := r.client.pool.QueryRow(ctx, `
		SELECT id::text, kb_type, COALESCE(title, ''), COALESCE(content_hash, ''), kb_version, index_ready
		FROM kb_documents
		WHERE kb_type = $1 AND lower(trim(title)) = lower($2)
		ORDER BY updated_at DESC
		LIMIT 1
	`, kbType, title)

	var doc DocumentRow
	if err := row.Scan(&doc.ID, &doc.KBType, &doc.Title, &doc.ContentHash, &doc.KBVersion, &doc.IndexReady); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("find by title: %w", err)
	}
	return &doc, nil
}

func normalizeDocTitle(title string) string {
	r := []rune(title)
	if len(r) > 200 {
		return string(r[:200])
	}
	return string(r)
}
