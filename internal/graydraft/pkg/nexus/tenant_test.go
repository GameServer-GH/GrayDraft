package nexus

import "testing"

func TestTenantIDFromWorkspaceKey_stable(t *testing.T) {
	a := TenantIDFromWorkspaceKey("my-secret-key")
	b := TenantIDFromWorkspaceKey("my-secret-key")
	if a != b || a == "" {
		t.Fatalf("expected stable tenant id, got %q / %q", a, b)
	}
}

func TestScopeFromWorkspace(t *testing.T) {
	s := ScopeFromWorkspace("key", "gd_")
	if s.TenantID == "" || s.KBID != DefaultKBID {
		t.Fatalf("unexpected scope: %+v", s)
	}
	if s.StoragePartition != KbTypeFromWorkspaceKey("key", "gd_") {
		t.Fatalf("storage partition mismatch")
	}
}

func TestWorkspaceStoragePartition_ignoresLegacyProduct(t *testing.T) {
	scope := ScopeFromWorkspace("key", "gd_")
	scope.StoragePartition = "product"
	got := WorkspaceStoragePartition(scope, "key", "gd_")
	want := KbTypeFromWorkspaceKey("key", "gd_")
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}
