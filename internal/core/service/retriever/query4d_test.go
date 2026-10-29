package retriever

import (
	"context"
	"strings"
	"testing"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/model"
)

func TestExtractQuery4DRules_spawnQuery(t *testing.T) {
	tax := model.DefaultCatalog().Get(model.UniversalDomainKey)
	got := extractQuery4DRules("InternVL 怎么调用", tax)
	if got.Domain != model.UniversalDomainKey && got.Domain != "" {
		t.Fatalf("domain=%q", got.Domain)
	}
	if !entityMentions(got.Entities, "InternVL") {
		t.Fatalf("entities=%v", got.Entities)
	}
}

func TestExtractQuery4D_disabledLLM(t *testing.T) {
	cfg := &config.Config{}
	cfg.Retrieval.Semantic4DLLMEnabled = false
	got := ExtractQuery4D(context.Background(), "InternVL 接口", cfg, nil, model.DefaultCatalog().Get(model.UniversalDomainKey))
	if got.Method != "rules" && got.Method != "" {
		t.Fatalf("method=%q", got.Method)
	}
}

func TestConstrainQuery4D_dropsLongDomainAndUngroundedEntity(t *testing.T) {
	longDomain := "该知识库聚焦减肥计划的制定与执行，涵盖饮食管理、运动安排"
	got := constrainQuery4D(model.QuerySemantic4D{
		Domain:   longDomain,
		Entities: []string{"番茄有哪"},
		Method:   "llm",
	}, model.QuerySemantic4D{
		Domain:   "减肥饮食管理",
		Entities: []string{"番茄"},
	}, "番茄周几吃，一次吃多少", nil)
	if got.Domain != "减肥饮食管理" {
		t.Fatalf("domain=%q", got.Domain)
	}
	if len(got.Entities) != 1 || got.Entities[0] != "番茄" {
		t.Fatalf("entities=%v", got.Entities)
	}
}

func TestBoostSemantic4DResortsByBoostedScore(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "plain", Score: 0.50, Extensions: map[string]any{}},
		{ID: "match", Score: 0.49, Extensions: map[string]any{
			"semantic_tags_4d": map[string]any{"domain": "game"},
		}},
	}
	BoostSemantic4D(chunks, model.QuerySemantic4D{Domain: "game"})
	if chunks[0].ID != "match" {
		t.Fatalf("boosted chunk must be first, got %s", chunks[0].ID)
	}
}

func entityMentions(ss []string, substr string) bool {
	for _, s := range ss {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}
