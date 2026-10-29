package postgres

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"
)

// MultimodalCacheRow is a cached extraction.
type MultimodalCacheRow struct {
	ExtractedText string
	StructuredKV  string
	Route         string
	Metadata      json.RawMessage
	PHash         string
}

// MultimodalCacheRepo reads/writes multimodal_content_cache.
type MultimodalCacheRepo struct {
	client *Client
}

// NewMultimodalCacheRepo creates cache repository.
func NewMultimodalCacheRepo(client *Client) *MultimodalCacheRepo {
	return &MultimodalCacheRepo{client: client}
}

// Get returns cached extraction by MD5.
func (r *MultimodalCacheRepo) Get(ctx context.Context, md5 string) (*MultimodalCacheRow, error) {
	var row MultimodalCacheRow
	var meta []byte
	err := r.client.pool.QueryRow(ctx, `
		SELECT extracted_text, COALESCE(structured_kv, ''), route, metadata, COALESCE(phash, '')
		FROM multimodal_content_cache
		WHERE content_md5 = $1
	`, md5).Scan(&row.ExtractedText, &row.StructuredKV, &row.Route, &meta, &row.PHash)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	row.Metadata = meta
	_, _ = r.client.pool.Exec(ctx, `
		UPDATE multimodal_content_cache SET last_used_at = now() WHERE content_md5 = $1
	`, md5)
	return &row, nil
}

// FindByPHashPrefix returns recent rows with similar phash prefix for hamming compare in app layer.
func (r *MultimodalCacheRepo) FindByPHashCandidates(ctx context.Context, phash string, limit int) ([]MultimodalCacheRow, string, error) {
	if phash == "" {
		return nil, "", nil
	}
	if limit <= 0 {
		limit = 24
	}
	prefix := phash
	if len(prefix) > 8 {
		prefix = prefix[:8]
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT content_md5, extracted_text, COALESCE(structured_kv, ''), route, metadata, phash
		FROM multimodal_content_cache
		WHERE phash IS NOT NULL AND phash <> '' AND left(phash, 8) = $1
		ORDER BY last_used_at DESC
		LIMIT $2
	`, prefix, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()
	var out []MultimodalCacheRow
	var md5 string
	for rows.Next() {
		var row MultimodalCacheRow
		var meta []byte
		var id string
		if err := rows.Scan(&id, &row.ExtractedText, &row.StructuredKV, &row.Route, &meta, &row.PHash); err != nil {
			return nil, "", err
		}
		row.Metadata = meta
		out = append(out, row)
		if md5 == "" {
			md5 = id
		}
	}
	return out, md5, rows.Err()
}

// Put stores extraction result.
func (r *MultimodalCacheRepo) Put(ctx context.Context, md5, mime, phash, route, text, structured string, metadata json.RawMessage) error {
	if metadata == nil {
		metadata = []byte("{}")
	}
	_, err := r.client.pool.Exec(ctx, `
		INSERT INTO multimodal_content_cache (content_md5, mime_type, phash, route, extracted_text, structured_kv, metadata)
		VALUES ($1, $2, NULLIF($3, ''), $4, $5, NULLIF($6, ''), $7::jsonb)
		ON CONFLICT (content_md5) DO UPDATE SET
			mime_type = EXCLUDED.mime_type,
			phash = EXCLUDED.phash,
			route = EXCLUDED.route,
			extracted_text = EXCLUDED.extracted_text,
			structured_kv = EXCLUDED.structured_kv,
			metadata = EXCLUDED.metadata,
			last_used_at = now()
	`, md5, mime, phash, route, text, structured, metadata)
	return err
}

// GetMediaMeta returns cache row for media API.
func (r *MultimodalCacheRepo) GetMediaMeta(ctx context.Context, md5 string) (mime, route, text, structured string, err error) {
	err = r.client.pool.QueryRow(ctx, `
		SELECT mime_type, route, extracted_text, COALESCE(structured_kv, '')
		FROM multimodal_content_cache WHERE content_md5 = $1
	`, md5).Scan(&mime, &route, &text, &structured)
	if err == pgx.ErrNoRows {
		return "", "", "", "", pgx.ErrNoRows
	}
	return mime, route, text, structured, err
}
