package classifier

import (
	"testing"

	"github.com/graydraft/core/config"
)

func testCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Classifier.Enabled = true
	cfg.Retrieval.FulltextWeight = 0.5
	cfg.Retrieval.VectorWeight = 0.5
	return cfg
}

func TestClassify_properNoun(t *testing.T) {
	c := New(testCfg())
	intent, _, _ := c.Classify("GrayDraft v3.0 index_ready")
	if intent != IntentProperNoun {
		t.Fatalf("expected proper_noun, got %s", intent)
	}
}

func TestClassify_concept(t *testing.T) {
	c := New(testCfg())
	intent, _, _ := c.Classify("如何理解自适应混合检索机制")
	if intent != IntentConcept {
		t.Fatalf("expected concept, got %s", intent)
	}
}

func TestClassify_hybrid(t *testing.T) {
	c := New(testCfg())
	intent, _, _ := c.Classify("对比社区与产品文档的区别")
	if intent != IntentHybrid {
		t.Fatalf("expected hybrid, got %s", intent)
	}
}

func TestRoute_properNoun_weights(t *testing.T) {
	c := New(testCfg())
	plan := c.Route(IntentProperNoun)
	if plan.FulltextWeight <= plan.VectorWeight {
		t.Fatalf("proper noun should favor fulltext: ft=%f vec=%f", plan.FulltextWeight, plan.VectorWeight)
	}
}
