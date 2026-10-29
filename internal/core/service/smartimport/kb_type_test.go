package smartimport

import "testing"

func TestResolveKBType_valid_workspace(t *testing.T) {
	kb, conf, reason := ResolveKBType("gd_abc123def")
	if kb != "gd_abc123def" {
		t.Fatalf("kb=%q", kb)
	}
	if conf != 0.95 {
		t.Fatalf("conf=%f", conf)
	}
	if reason == "" {
		t.Fatal("reason empty")
	}
}

func TestResolveKBType_empty(t *testing.T) {
	kb, conf, reason := ResolveKBType("")
	if kb != "" || conf != 0 {
		t.Fatalf("kb=%q conf=%f", kb, conf)
	}
	if reason != "workspace partition required" {
		t.Fatalf("reason=%q", reason)
	}
}

func TestResolveKBType_whitespace(t *testing.T) {
	kb, _, _ := ResolveKBType("  gd_test123  ")
	if kb != "gd_test123" {
		t.Fatalf("kb=%q", kb)
	}
}

func TestResolveKBType_arbitrary_string(t *testing.T) {
	kb, _, _ := ResolveKBType("random_string")
	if kb != "" {
		t.Fatalf("arbitrary string should not resolve, got kb=%q", kb)
	}
}

func TestIsWorkspacePartition_valid(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"gd_abc123", true},
		{"gd_", false},
		{"gd", false},
		{"tutorial", false},
		{"GD_abc", false},
		{" gd_xyz ", true},
		{"", false},
	}
	for _, tt := range tests {
		if got := IsWorkspacePartition(tt.input); got != tt.expected {
			t.Errorf("IsWorkspacePartition(%q)=%v, want %v", tt.input, got, tt.expected)
		}
	}
}

func TestValidKBType(t *testing.T) {
	if !ValidKBType("gd_abc") {
		t.Fatal("gd_abc should be valid")
	}
	if ValidKBType("tutorial") {
		t.Fatal("storage corpus names are not workspace partitions")
	}
}
