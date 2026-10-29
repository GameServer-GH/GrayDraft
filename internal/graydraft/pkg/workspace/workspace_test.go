package workspace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenCreatesWorkspace(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, created, err := Open(dir, "my-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if !created {
		t.Fatal("expected created=true on first open")
	}
	if _, err := os.Stat(filepath.Join(root, "notebooks.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SettingsPath(root)); err != nil {
		t.Fatal(err)
	}
	// second open same key
	root2, created2, err := Open(dir, "my-secret-key")
	if err != nil {
		t.Fatal(err)
	}
	if created2 {
		t.Fatal("expected created=false on second open")
	}
	if root2 != root {
		t.Fatalf("root mismatch %q vs %q", root2, root)
	}
}

func TestSnapshotEmpty(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	root, _, err := Open(dir, "k2")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	list, raw, err := Snapshot(ctx, root)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Fatalf("notebooks: %+v", list)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil || len(m) != 0 {
		t.Fatalf("settings: %s err=%v", string(raw), err)
	}
}

func TestNormalizeKeyReject(t *testing.T) {
	t.Parallel()
	if _, err := NormalizeKey(""); err == nil {
		t.Fatal("expected error")
	}
}
