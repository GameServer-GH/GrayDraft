package postgres

import (
	"context"
	"fmt"

	"github.com/graydraft/core/model"
)

// ListRecentDocSummaries returns up to limit recent documents with summaries for kb_type.
func (r *DocumentRepo) ListRecentDocSummaries(ctx context.Context, kbType string, limit int) ([]model.DocSummarySnippet, error) {
	if kbType == "" {
		return nil, fmt.Errorf("kb_type required")
	}
	if limit < 1 {
		limit = 5
	}
	if limit > 20 {
		limit = 20
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT id::text, COALESCE(title, ''),
			COALESCE(
				NULLIF(extensions->>'global_summary', ''),
				NULLIF(extensions->>'semantic_summary', ''),
				LEFT(COALESCE(content, ''), 400)
			)
		FROM kb_documents
		WHERE kb_type = $1 AND status = 'active'
		ORDER BY updated_at DESC NULLS LAST, created_at DESC
		LIMIT $2
	`, kbType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.DocSummarySnippet
	for rows.Next() {
		var s model.DocSummarySnippet
		if err := rows.Scan(&s.DocID, &s.Title, &s.Summary); err != nil {
			return nil, err
		}
		s.Summary = trimSummary(s.Summary, 500)
		if s.Summary != "" || s.Title != "" {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}

func trimSummary(s string, maxRunes int) string {
	r := []rune(s)
	if len(r) <= maxRunes {
		return s
	}
	return string(r[:maxRunes])
}

// DocTaxonomyRow is a document row for shadow taxonomy backfill.
type DocTaxonomyRow struct {
	DocID      string
	Title      string
	Extensions []byte
}

// ListDocsForTaxonomyBackfill returns documents whose 4D domain may be stale vs blueprint.
func (r *DocumentRepo) ListDocsForTaxonomyBackfill(ctx context.Context, kbType, blueprintDomain string, limit int) ([]DocTaxonomyRow, error) {
	if kbType == "" {
		return nil, fmt.Errorf("kb_type required")
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT id::text, COALESCE(title, ''), COALESCE(extensions::text, '{}')
		FROM kb_documents
		WHERE kb_type = $1 AND status = 'active'
		  AND (
		    NOT (extensions ? 'semantic_tags_4d')
		    OR COALESCE(extensions->'semantic_tags_4d'->>'domain', '') IS DISTINCT FROM $2
		    OR COALESCE(extensions->>'taxonomy_backfill_version', '') IS DISTINCT FROM (
		      SELECT COALESCE(stats->>'blueprint_backfill_version','') FROM workspace_4d_taxonomy WHERE kb_type = $1
		    )
		  )
		ORDER BY updated_at DESC
		LIMIT $3
	`, kbType, blueprintDomain, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []DocTaxonomyRow
	for rows.Next() {
		var row DocTaxonomyRow
		if err := rows.Scan(&row.DocID, &row.Title, &row.Extensions); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}
