package taxonomyevolve

import (
	"context"
	"testing"

	"github.com/graydraft/core/model"
)

func TestAlignEntityToCorpus_substring(t *testing.T) {
	s := &Service{}
	corpus := []string{"莫特斐", "参数说明"}
	got := s.alignEntityToCorpus(context.Background(), "介绍一下莫特斐", corpus)
	if got != "莫特斐" {
		t.Fatalf("got %q want 莫特斐", got)
	}
}

func TestEntityCorpusFromTaxonomy(t *testing.T) {
	tax := &model.DomainTaxonomy{
		Domain:         "教程资料",
		DetectKeywords: []string{"莫特斐"},
		Scenes:         []model.LabelDefinition{{Label: "参数建议", Keywords: []string{"参数"}}},
	}
	c := entityCorpusFromTaxonomy(tax)
	if len(c) < 3 {
		t.Fatalf("corpus too small: %v", c)
	}
}
