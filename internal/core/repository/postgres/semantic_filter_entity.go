package postgres

import (
	"github.com/graydraft/core/model"
)

// semantic4DClauseEntityStrict: when query has entities, documents that declare
// semantic_tags_4d.entities must overlap; legacy docs without 4D still pass.
func semantic4DClauseEntityStrict(startArg int, f *model.QuerySemantic4D) (clause string, args []any, nextArg int) {
	if f == nil || len(f.Entities) == 0 {
		return "", nil, startArg
	}
	baseClause, baseArgs, next := semantic4DClause(startArg, f)
	if baseClause == "" {
		return "", nil, startArg
	}
	entArg := startArg + 3
	clause = baseClause + entityStrictExcludeSQL(entArg)
	return clause, baseArgs, next
}

func semantic4DClauseForMode(startArg int, f *model.QuerySemantic4D, strict bool, entityStrict bool) (clause string, args []any, nextArg int) {
	if strict {
		return semantic4DClauseStrict(startArg, f)
	}
	if entityStrict && f != nil && len(f.Entities) > 0 {
		return semantic4DClauseEntityStrict(startArg, f)
	}
	return semantic4DClause(startArg, f)
}
