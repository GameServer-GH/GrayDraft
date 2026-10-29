package postgres

import (
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

func TestSemantic4DClauseStrict_requiresTag(t *testing.T) {
	f := &model.QuerySemantic4D{Domain: "教程资料", Scene: "版本发布"}
	clause, _, _ := semantic4DClauseStrict(2, f)
	if !strings.Contains(clause, "semantic_tags_4d") {
		t.Fatal("expected strict clause")
	}
	if strings.Contains(clause, "NOT (d.extensions") {
		t.Fatal("strict must not include legacy pass-through")
	}
}
