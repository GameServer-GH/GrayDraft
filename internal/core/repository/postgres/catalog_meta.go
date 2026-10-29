package postgres

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
)

// CatalogMeta is kb_catalog_version metadata for cache keys.
type CatalogMeta struct {
	Version            int64
	LatestDocUpdatedAt time.Time
}

// GetCatalogMeta returns catalog version info for a kb_type partition.
func (r *ChunkRepo) GetCatalogMeta(ctx context.Context, kbType string) (CatalogMeta, error) {
	if kbType == "" {
		return CatalogMeta{}, nil
	}
	var meta CatalogMeta
	err := r.client.pool.QueryRow(ctx, `
		SELECT version, latest_doc_updated_at
		FROM kb_catalog_version
		WHERE kb_type = $1
	`, kbType).Scan(&meta.Version, &meta.LatestDocUpdatedAt)
	if err == pgx.ErrNoRows {
		return CatalogMeta{}, nil
	}
	return meta, err
}
