package postgres

import "testing"

func TestDefaultStoragePartition(t *testing.T) {
	p := DefaultStoragePartition("tn_abc", "default")
	if p == "" || p == "t__" {
		t.Fatalf("unexpected partition: %q", p)
	}
}

func TestValidateScopeID(t *testing.T) {
	if err := ValidateScopeID("valid-id_1", "tenant_id"); err != nil {
		t.Fatal(err)
	}
	if err := ValidateScopeID("", "tenant_id"); err == nil {
		t.Fatal("expected error for empty id")
	}
}
