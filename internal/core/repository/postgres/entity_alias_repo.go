package postgres

import (
	"context"
	"encoding/json"

	"github.com/graydraft/core/service/classifier"
)

// LoadEntityAliasIndex builds alias groups for a kb_type partition.
func (r *ChunkRepo) LoadEntityAliasIndex(ctx context.Context, kbType string) (*classifier.AliasIndex, error) {
	rows, err := r.client.pool.Query(ctx, `
		SELECT COALESCE(d.title, ''), COALESCE(d.extensions, '{}'::jsonb),
			COALESCE(array_agg(DISTINCT dt.tag) FILTER (WHERE dt.tag IS NOT NULL), '{}')
		FROM kb_documents d
		LEFT JOIN kb_doc_tags dt ON dt.doc_id = d.id
		WHERE d.index_ready = true
		  AND ($1 = '' OR d.kb_type = $1)
		GROUP BY d.id, d.title, d.extensions
	`, kbType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []classifier.DocAliasSource
	for rows.Next() {
		var title string
		var extRaw []byte
		var tags []string
		if err := rows.Scan(&title, &extRaw, &tags); err != nil {
			return nil, err
		}
		docs = append(docs, classifier.DocAliasSource{
			Title:      title,
			Tags:       tags,
			Extensions: parseExtensionsMap(extRaw),
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return classifier.BuildAliasIndex(docs), nil
}

func parseExtensionsMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil
	}
	return m
}
