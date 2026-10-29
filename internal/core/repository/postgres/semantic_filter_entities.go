package postgres

import "fmt"

// SQL fragments for safe access to extensions.semantic_tags_4d.entities (may be non-array in legacy rows).

const semanticTags4DObject = `jsonb_typeof(d.extensions->'semantic_tags_4d') = 'object'`

const docEntitiesArrayLenExpr = `CASE
  WHEN ` + semanticTags4DObject + `
       AND jsonb_typeof(d.extensions->'semantic_tags_4d'->'entities') = 'array'
  THEN jsonb_array_length(d.extensions->'semantic_tags_4d'->'entities')
  ELSE 0
END`

// entityOverlapExistsSQL: bidirectional ILIKE overlap (doc ent contained in query, or query ent contains doc ent).
// Handles "介绍一下 InternVL" (query) vs stored "莫特斐" via qent ILIKE '%' || ent || '%'.
func entityOverlapExistsSQL(entArg int) string {
	return fmt.Sprintf(`(
				cardinality($%d::text[]) = 0
				OR NOT (`+semanticTags4DObject+`)
				OR jsonb_typeof(d.extensions->'semantic_tags_4d'->'entities') <> 'array'
				OR EXISTS (
				  SELECT 1
				  FROM jsonb_array_elements_text(d.extensions->'semantic_tags_4d'->'entities') ent
				  CROSS JOIN unnest($%d::text[]) qent
				  WHERE ent ILIKE ('%%' || qent || '%%') OR qent ILIKE ('%%' || ent || '%%')
				)
			  )`, entArg, entArg)
}

// entityStrictExcludeSQL filters out docs that have a 4D entity array but no overlap with query entities.
func entityStrictExcludeSQL(entArg int) string {
	// String concat only: entityOverlapExistsSQL contains ILIKE '%' patterns that must not
	// be passed through fmt.Sprintf again (would corrupt SQL with %! placeholders).
	return `
		  AND NOT (
			(d.extensions ? 'semantic_tags_4d')
			AND (` + semanticTags4DObject + `)
			AND (` + docEntitiesArrayLenExpr + `) > 0
			AND NOT (` + entityOverlapExistsSQL(entArg) + `)
		  )`
}
