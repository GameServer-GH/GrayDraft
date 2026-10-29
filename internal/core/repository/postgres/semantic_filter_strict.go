package postgres

import (
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
)

// semantic4DClauseStrict requires documents to have semantic_tags_4d matching the query (no legacy pass-through).
func semantic4DClauseStrict(startArg int, f *model.QuerySemantic4D) (clause string, args []any, nextArg int) {
	if f == nil || !f.Active() {
		return "", nil, startArg
	}
	domain := strings.TrimSpace(f.Domain)
	scene := strings.TrimSpace(f.Scene)
	style := strings.TrimSpace(f.Style)
	entities := f.Entities
	if domain == "" && scene == "" && style == "" && len(entities) == 0 {
		return "", nil, startArg
	}

	d := startArg
	s := startArg + 1
	st := startArg + 2
	e := startArg + 3
	nextArg = startArg + 4

	clause = fmt.Sprintf(`
		  AND (d.extensions ? 'semantic_tags_4d')
		  AND COALESCE(d.extensions->'semantic_tags_4d', '{}'::jsonb) != '{}'::jsonb
		  AND (
			  ($%d::text = '' OR d.extensions->'semantic_tags_4d'->>'domain' ILIKE ('%%' || $%d || '%%')
			   OR $%d ILIKE ('%%' || (d.extensions->'semantic_tags_4d'->>'domain') || '%%'))
			  AND ($%d::text = '' OR d.extensions->'semantic_tags_4d'->>'scene' ILIKE ('%%' || $%d || '%%')
			   OR $%d ILIKE ('%%' || (d.extensions->'semantic_tags_4d'->>'scene') || '%%'))
			  AND ($%d::text = '' OR d.extensions->'semantic_tags_4d'->>'style' ILIKE ('%%' || $%d || '%%')
			   OR $%d ILIKE ('%%' || (d.extensions->'semantic_tags_4d'->>'style') || '%%'))
			  AND %s
		  )`, d, d, d, s, s, s, st, st, st, entityOverlapExistsSQL(e))

	if entities == nil {
		entities = []string{}
	}
	args = []any{domain, scene, style, entities}
	return clause, args, nextArg
}
