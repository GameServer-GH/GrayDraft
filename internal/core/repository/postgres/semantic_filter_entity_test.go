package postgres

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

func TestSemantic4DClauseEntityStrict_reusesEntityArg(t *testing.T) {
	f := &model.QuerySemantic4D{
		Domain:   "教程资料",
		Entities: []string{"样例图"},
	}
	clause, args, next := semantic4DClauseEntityStrict(2, f)
	if len(args) != 4 || next != 6 {
		t.Fatalf("args=%d next=%d want 4 args next 6", len(args), next)
	}
	if strings.Count(clause, "$5") < 2 {
		t.Fatalf("entity arg should be reused as $5: %s", clause)
	}
	if strings.Contains(clause, "$6") {
		t.Fatalf("should not introduce $6 duplicate: %s", clause)
	}
	if !strings.Contains(clause, "jsonb_typeof") {
		t.Fatalf("expected safe array typeof guard: %s", clause)
	}
}

func TestSemantic4DClauseEntityStrict_explainSQL(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	f := &model.QuerySemantic4D{Entities: []string{"样例图"}}
	clause, cargs, _ := semantic4DClauseEntityStrict(2, f)
	sql := `SELECT count(*) FROM kb_chunks c JOIN kb_documents d ON d.id = c.doc_id WHERE $1::text IS NOT NULL` + clause
	ctx := context.Background()
	client, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var n int64
	err = client.pool.QueryRow(ctx, sql, append([]any{"_"}, cargs...)...).Scan(&n)
	if err != nil {
		t.Log("SQL:", sql)
		t.Fatal(err)
	}
}
