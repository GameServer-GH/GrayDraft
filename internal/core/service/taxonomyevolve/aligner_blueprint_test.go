package taxonomyevolve

import (
	"context"
	"testing"

	"github.com/graydraft/core/model"
)

func TestAlignScene_blueprintSkipsKeywordNormalize(t *testing.T) {
	tax := &model.DomainTaxonomy{
		Domain:        "学科资料",
		FromBlueprint: true,
		Scenes: []model.LabelDefinition{
			{Label: "行程规划", Keywords: []string{"路线"}},
		},
	}
	// "路线" would map via GenericNormalize to 行程规划 on YAML path; blueprint path keeps raw without embedder service.
	got := AlignScene(context.Background(), "路线规划草案", tax)
	if got == "行程规划" {
		t.Fatalf("blueprint align should not use keyword Contains alone, got %q", got)
	}
}
