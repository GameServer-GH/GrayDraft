//go:build integration

package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/graydraft/core/repository/postgres"
)

func TestChunkRepo_SearchFulltext(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set")
	}
	ctx := context.Background()
	client, err := postgres.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	repo := postgres.NewChunkRepo(client)
	chunks, err := repo.SearchFulltext(ctx, "混合检索", "product", 5, true, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk from seed data")
	}
}
