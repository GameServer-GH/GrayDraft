package taxonomyevolve

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
)

const blueprintMinSceneLabels = 2

// validateBlueprintLabels rejects near-duplicate scene/style labels (low discrimination).
func validateBlueprintLabels(parsed blueprintLLMResponse) error {
	if err := validateLabelSet(parsed.Scenes, "scene"); err != nil {
		return err
	}
	if err := validateLabelSet(parsed.Styles, "style"); err != nil {
		return err
	}
	if err := validateBlueprintProbeQueries(parsed); err != nil {
		return err
	}
	return nil
}

func validateLabelSet(defs []model.LabelDefinition, dim string) error {
	labels := make([]string, 0, len(defs))
	for _, d := range defs {
		if l := strings.TrimSpace(d.Label); l != "" {
			labels = append(labels, l)
		}
	}
	if len(labels) < blueprintMinSceneLabels {
		return nil
	}
	dupPairs := 0
	for i := 0; i < len(labels); i++ {
		for j := i + 1; j < len(labels); j++ {
			if labelPairTooSimilar(labels[i], labels[j]) {
				dupPairs++
			}
		}
	}
	if dupPairs > 0 {
		return fmt.Errorf("%s labels lack discrimination (%d similar pairs among %d)", dim, dupPairs, len(labels))
	}
	return nil
}

func labelPairTooSimilar(a, b string) bool {
	if a == b {
		return true
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return true
	}
	if _, ratio := BestLabelSimilarity(a, []model.LabelDefinition{{Label: b}}); ratio >= 0.82 {
		return true
	}
	return false
}

func validateBlueprintProbeQueries(parsed blueprintLLMResponse) error {
	tax := &model.DomainTaxonomy{
		Domain:        strings.TrimSpace(parsed.Domain),
		Scenes:        parsed.Scenes,
		Styles:        parsed.Styles,
		FromBlueprint: true,
	}
	probes := blueprintProbeQueries(parsed)
	for _, p := range probes {
		var q model.QuerySemantic4D
		tax.ApplyQueryHeuristics(p.Query, &q)
		q = AlignQuery4D(context.Background(), q, tax)
		if p.Scene != "" && q.Scene != p.Scene {
			return fmt.Errorf("probe query scene mismatch: query=%q want=%q got=%q", p.Query, p.Scene, q.Scene)
		}
		if p.Style != "" && q.Style != p.Style {
			return fmt.Errorf("probe query style mismatch: query=%q want=%q got=%q", p.Query, p.Style, q.Style)
		}
	}
	return nil
}

type blueprintProbe struct {
	Query string
	Scene string
	Style string
}

func blueprintProbeQueries(parsed blueprintLLMResponse) []blueprintProbe {
	sceneA := firstBlueprintLabel(parsed.Scenes, 0)
	sceneB := firstBlueprintLabel(parsed.Scenes, 1)
	styleA := firstBlueprintLabel(parsed.Styles, 0)
	styleB := firstBlueprintLabel(parsed.Styles, 1)
	var out []blueprintProbe
	if sceneA != "" {
		out = append(out, blueprintProbe{Query: "请检索" + sceneA + "相关内容", Scene: sceneA})
	}
	if styleA != "" {
		out = append(out, blueprintProbe{Query: "有没有" + styleA + "资料", Style: styleA})
	}
	if sceneB != "" || styleB != "" {
		out = append(out, blueprintProbe{Query: "查一下" + sceneB + styleB, Scene: sceneB, Style: styleB})
	}
	return out
}

func firstBlueprintLabel(defs []model.LabelDefinition, idx int) string {
	seen := 0
	for _, d := range defs {
		if l := strings.TrimSpace(d.Label); l != "" {
			if seen == idx {
				return l
			}
			seen++
		}
	}
	return ""
}
