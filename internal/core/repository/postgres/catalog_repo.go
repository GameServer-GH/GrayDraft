package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

type catalogExec interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
}

// BumpCatalogVersion increments kb_catalog_version for a partition (cache invalidation).
func BumpCatalogVersion(ctx context.Context, exec catalogExec, kbType string) error {
	_, err := exec.Exec(ctx, `
		INSERT INTO kb_catalog_version (kb_type, version, latest_doc_updated_at, updated_at)
		VALUES ($1, 1, now(), now())
		ON CONFLICT (kb_type) DO UPDATE SET
			version = kb_catalog_version.version + 1,
			latest_doc_updated_at = now(),
			updated_at = now()
	`, kbType)
	return err
}

func bumpCatalogVersionTx(ctx context.Context, tx pgx.Tx, kbType string) {
	_ = BumpCatalogVersion(ctx, tx, kbType)
}
