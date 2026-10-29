package postgres

import (
	"context"
)

// ListDistinctSemanticEntities returns unique entity strings from indexed docs (cross gd_* partitions).
func (r *ChunkRepo) ListDistinctSemanticEntities(ctx context.Context, kbTypePrefix string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 120
	}
	if kbTypePrefix == "" {
		kbTypePrefix = "gd_"
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT DISTINCT trim(ent) AS entity
		FROM kb_documents d,
		     jsonb_array_elements_text(
		       CASE
		         WHEN jsonb_typeof(d.extensions->'semantic_tags_4d'->'entities') = 'array'
		         THEN d.extensions->'semantic_tags_4d'->'entities'
		         ELSE '[]'::jsonb
		       END
		     ) ent
		WHERE d.status = 'active'
		  AND d.index_ready = true
		  AND d.kb_type LIKE $1 || '%'
		  AND trim(ent) <> ''
		  AND char_length(trim(ent)) BETWEEN 2 AND 24
		ORDER BY char_length(trim(ent))
		LIMIT $2
	`, kbTypePrefix, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		if s != "" {
			out = append(out, s)
		}
	}
	return out, rows.Err()
}
