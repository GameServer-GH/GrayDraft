package index

import (
	"context"
	"testing"
	"time"
)

func TestDeleteNotebook(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewService(dir)
	ctx := context.Background()

	meta, err := s.CreateNotebook(ctx, "keep-me")
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.CreateNotebook(ctx, "remove-me")
	if err != nil {
		t.Fatal(err)
	}
	list, err := s.ListNotebooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var otherID string
	for _, n := range list {
		if n.ID != meta.ID {
			otherID = n.ID
			break
		}
	}
	if otherID == "" {
		t.Fatal("expected second notebook")
	}
	if err := s.DeleteNotebook(ctx, otherID); err != nil {
		t.Fatal(err)
	}
	after, err := s.ListNotebooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 1 {
		t.Fatalf("len got %d want 1: %+v", len(after), after)
	}
	for _, n := range after {
		if n.ID == otherID {
			t.Fatal("removed id still present")
		}
	}
	found := false
	for _, n := range after {
		if n.ID == meta.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected keep-me notebook to remain")
	}
	if err := s.DeleteNotebook(ctx, otherID); err == nil {
		t.Fatal("expected error deleting missing id")
	}
}

func TestRenameNotebook(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	s := NewService(dir)
	ctx := context.Background()

	meta, err := s.CreateNotebook(ctx, "old-name")
	if err != nil {
		t.Fatal(err)
	}
	updated, err := s.RenameNotebook(ctx, meta.ID, "  new title  ")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "new title" {
		t.Fatalf("title got %q want %q", updated.Title, "new title")
	}
	if updated.ID != meta.ID {
		t.Fatalf("id changed: %q vs %q", updated.ID, meta.ID)
	}
	if _, err := time.Parse(time.RFC3339, updated.UpdatedAt); err != nil {
		t.Fatalf("UpdatedAt: %v", err)
	}
	list, err := s.ListNotebooks(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Title != "new title" {
		t.Fatalf("list: %+v", list)
	}
	if _, err := s.RenameNotebook(ctx, "missing-id", "x"); err == nil {
		t.Fatal("expected error renaming missing id")
	}
}
