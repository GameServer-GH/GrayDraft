package auth

import (
	"testing"
	"time"
)

func TestIssueServiceAndValidateRoles(t *testing.T) {
	secret := "test-secret"
	tok, _, err := IssueService(secret, "ghost-bff", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	claims, err := ValidateRoles(secret, tok, RoleAdmin, RoleAgent)
	if err != nil || claims.Subject != "ghost-bff" || claims.Role != RoleAgent {
		t.Fatalf("claims=%+v err=%v", claims, err)
	}
	if _, err := Validate(secret, tok); err == nil {
		t.Fatal("admin Validate should reject agent_service token")
	}
}

func TestAdminTokenStillValidates(t *testing.T) {
	secret := "test-secret"
	tok, _, err := Issue(secret, "admin", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Validate(secret, tok); err != nil {
		t.Fatal(err)
	}
	if _, err := ValidateRoles(secret, tok, RoleAdmin, RoleAgent); err != nil {
		t.Fatal(err)
	}
}
