package agent

import "testing"

func TestVersionConflict(t *testing.T) {
	if !VersionConflict("版本 1.1.0", "1.2.0", nil) {
		t.Fatal("expected conflict")
	}
	if VersionConflict("版本 1.2.0", "1.2.0", nil) {
		t.Fatal("unexpected conflict")
	}
}
