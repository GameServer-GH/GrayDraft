package taxonomyevolve

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestValidateBlueprintLabels_ok(t *testing.T) {
	parsed := blueprintLLMResponse{
		Scenes: []model.LabelDefinition{
			{Label: "剧情复盘"},
			{Label: "参数建议"},
		},
		Styles: []model.LabelDefinition{
			{Label: "教程"},
			{Label: "问答"},
		},
	}
	if err := validateBlueprintLabels(parsed); err != nil {
		t.Fatalf("expected ok, got %v", err)
	}
}

func TestValidateBlueprintLabels_duplicateScenes(t *testing.T) {
	parsed := blueprintLLMResponse{
		Scenes: []model.LabelDefinition{
			{Label: "剧情介绍"},
			{Label: "剧情介绍篇"},
		},
	}
	if err := validateBlueprintLabels(parsed); err == nil {
		t.Fatal("expected discrimination error")
	}
}

func TestValidateBlueprintLabels_probeQueries(t *testing.T) {
	parsed := blueprintLLMResponse{
		Domain: "教程资料",
		Scenes: []model.LabelDefinition{
			{Label: "剧情复盘"},
			{Label: "参数建议"},
		},
		Styles: []model.LabelDefinition{
			{Label: "教程"},
			{Label: "问答"},
		},
	}
	if err := validateBlueprintProbeQueries(parsed); err != nil {
		t.Fatalf("expected probes to pass, got %v", err)
	}
}

func TestLabelPairTooSimilar(t *testing.T) {
	if !labelPairTooSimilar("基础介绍", "介绍") {
		t.Fatal("substring pair should be similar")
	}
	if labelPairTooSimilar("剧情复盘", "参数建议") {
		t.Fatal("distinct labels should not match")
	}
}
