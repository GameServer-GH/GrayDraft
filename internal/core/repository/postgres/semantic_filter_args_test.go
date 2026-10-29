package postgres

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/graydraft/core/model"
)

func TestSemantic4DClause_pgxBinding_domainOnly(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	f := &model.QuerySemantic4D{Domain: "教程资料"}
	clause, args, _ := semantic4DClause(2, f)
	t.Log("clause:", clause)
	sql := `SELECT count(*) FROM kb_chunks c JOIN kb_documents d ON d.id = c.doc_id WHERE $1::text IS NOT NULL ` + clause
	all := append([]any{"_"}, args...)

	ctx := context.Background()
	client, err := New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	var n int
	err = client.pool.QueryRow(ctx, sql, all...).Scan(&n)
	if err != nil {
		var pe *pgconn.PgError
		if errors.As(err, &pe) {
			t.Logf("pg err: code=%s msg=%s detail=%s where=%s", pe.Code, pe.Message, pe.Detail, pe.Where)
		}
		t.Fatalf("query failed: %v", err)
	}
	t.Log("count", n)
}
