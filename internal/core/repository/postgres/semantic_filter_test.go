package postgres

import (
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

func TestSemantic4DClause_includesLegacyPassThrough(t *testing.T) {
	f := &model.QuerySemantic4D{
		Domain:   "教程资料",
		Scene:    "版本发布",
		Entities: []string{"小夜"},
	}
	clause, args, next := semantic4DClause(2, f)
	if clause == "" || len(args) != 4 || next != 6 {
		t.Fatalf("clause=%q args=%d next=%d", clause, len(args), next)
	}
	if !strings.Contains(clause, "semantic_tags_4d") {
		t.Fatalf("missing tag path: %s", clause)
	}
}

func TestSemantic4DClause_inactive(t *testing.T) {
	clause, args, next := semantic4DClause(2, nil)
	if clause != "" || len(args) != 0 || next != 2 {
		t.Fatalf("clause=%q args=%v next=%d", clause, args, next)
	}
}
