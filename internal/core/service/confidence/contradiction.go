package confidence

import (
	"regexp"
	"sort"
	"strings"

	"github.com/graydraft/core/model"
)

// ContradictionResult captures multi-source conflict detection output.
type ContradictionResult struct {
	HasConflict bool           `json:"has_conflict"`
	Conflicts   []ConflictItem `json:"conflicts,omitempty"`
}

// ConflictItem describes a contradiction between chunks for a given entity attribute.
type ConflictItem struct {
	Entity    string          `json:"entity"`
	Attribute string          `json:"attribute"`
	Values    []ConflictValue `json:"values"`
	Preferred string          `json:"preferred"` // chunk_id of the preferred source
}

// ConflictValue is one chunk's claim about an entity attribute.
type ConflictValue struct {
	ChunkID      string  `json:"chunk_id"`
	Value        string  `json:"value"`
	QualityScore float64 `json:"quality_score"`
}

// entityAttrTriple is an internal extraction result.
type entityAttrTriple struct {
	Entity    string
	Attribute string
	Value     string
	ChunkID   string
	Quality   float64
}

var (
	reEntityAttr  = regexp.MustCompile(`([\p{Han}A-Za-z0-9·]+?)\s*[:：=]\s*(.+?)(?:[,，;；、\n]|$)`)
	reNumericValue = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*(%|[%％]|倍|x|点|级|层|s|秒|m|米|w|万)?`)
)

// DetectContradictions scans retrieved chunks for conflicting claims about the same entity/attribute.
func DetectContradictions(chunks []model.Chunk, queryEntities []string) ContradictionResult {
	if len(chunks) < 2 {
		return ContradictionResult{}
	}

	var triples []entityAttrTriple

	for _, c := range chunks {
		content := c.Content
		matches := reEntityAttr.FindAllStringSubmatch(content, -1)
		for _, m := range matches {
			if len(m) >= 3 {
				entity := strings.TrimSpace(m[1])
				value := strings.TrimSpace(m[2])
				attr := deriveAttribute(entity, content, queryEntities)
				if entity != "" && value != "" {
					triples = append(triples, entityAttrTriple{
						Entity:    entity,
						Attribute: attr,
						Value:     value,
						ChunkID:   c.ID,
						Quality:   c.QualityScore,
					})
				}
			}
		}
	}

	if len(triples) < 2 {
		return ContradictionResult{}
	}

	type key struct {
		entity string
		attr   string
	}
	groups := make(map[key][]entityAttrTriple)
	for _, t := range triples {
		k := key{t.Entity, t.Attribute}
		groups[k] = append(groups[k], t)
	}

	var conflicts []ConflictItem
	for k, group := range groups {
		if len(group) < 2 {
			continue
		}
		values := dedupeValues(group)
		if len(values) < 2 {
			continue
		}
		if allValuesSimilar(values) {
			continue
		}

		sort.Slice(group, func(i, j int) bool {
			return group[i].Quality > group[j].Quality
		})
		preferred := group[0].ChunkID

		var conflictValues []ConflictValue
		for _, t := range group {
			conflictValues = append(conflictValues, ConflictValue{
				ChunkID:      t.ChunkID,
				Value:        t.Value,
				QualityScore: t.Quality,
			})
		}

		conflicts = append(conflicts, ConflictItem{
			Entity:    k.entity,
			Attribute: k.attr,
			Values:    conflictValues,
			Preferred: preferred,
		})
	}

	return ContradictionResult{
		HasConflict: len(conflicts) > 0,
		Conflicts:   conflicts,
	}
}

func deriveAttribute(entity, content string, queryEntities []string) string {
	idx := strings.Index(content, entity)
	if idx < 0 {
		return entity
	}
	start := idx - 10
	if start < 0 {
		start = 0
	}
	prefix := strings.TrimSpace(content[start:idx])
	if prefix != "" {
		words := strings.Fields(prefix)
		if len(words) > 0 {
			return words[len(words)-1] + "_" + entity
		}
	}
	return entity
}

func dedupeValues(triples []entityAttrTriple) []entityAttrTriple {
	seen := make(map[string]entityAttrTriple)
	for _, t := range triples {
		norm := normalizeValue(t.Value)
		if existing, ok := seen[norm]; !ok || t.Quality > existing.Quality {
			seen[norm] = t
		}
	}
	out := make([]entityAttrTriple, 0, len(seen))
	for _, t := range seen {
		out = append(out, t)
	}
	return out
}

func allValuesSimilar(triples []entityAttrTriple) bool {
	var nums []float64
	for _, t := range triples {
		matches := reNumericValue.FindStringSubmatch(t.Value)
		if len(matches) >= 2 {
			n, err := parseFloatStr(matches[1])
			if err == nil {
				nums = append(nums, n)
			}
		}
	}
	if len(nums) < 2 {
		return false
	}
	min, max := nums[0], nums[0]
	for _, n := range nums[1:] {
		if n < min {
			min = n
		}
		if n > max {
			max = n
		}
	}
	if max == 0 {
		return true
	}
	return (max-min)/max <= 0.05
}

func parseFloatStr(s string) (float64, error) {
	var result float64
	var decimal float64 = 1.0
	var isDecimal bool
	var isNegative bool
	var hasDigits bool

	for i, r := range s {
		if i == 0 && r == '-' {
			isNegative = true
			continue
		}
		if r == '.' {
			if isDecimal {
				break
			}
			isDecimal = true
			continue
		}
		if r >= '0' && r <= '9' {
			hasDigits = true
			if isDecimal {
				decimal *= 0.1
				result += float64(r-'0') * decimal
			} else {
				result = result*10 + float64(r-'0')
			}
		} else {
			break
		}
	}
	if !hasDigits {
		return 0, nil
	}
	if isNegative {
		result = -result
	}
	return result, nil
}

func normalizeValue(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimSuffix(v, "%")
	v = strings.TrimSuffix(v, "％")
	v = strings.TrimSuffix(v, "倍")
	v = strings.TrimSuffix(v, "x")
	v = strings.TrimSuffix(v, "X")
	return v
}

// FormatContradictionWarnings converts ContradictionResult into human-readable warnings for SSE done.
func FormatContradictionWarnings(cr ContradictionResult) []string {
	if !cr.HasConflict {
		return nil
	}
	var warnings []string
	for _, c := range cr.Conflicts {
		var vals []string
		for _, v := range c.Values {
			marker := ""
			if v.ChunkID == c.Preferred {
				marker = " *"
			}
			vals = append(vals, v.Value+marker)
		}
		msg := c.Entity + " " + c.Attribute + ": " + strings.Join(vals, " vs ") + " (优先采信*标注来源)"
		warnings = append(warnings, msg)
	}
	return warnings
}
