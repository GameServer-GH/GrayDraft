package agent

import "testing"

func TestBuildTenantContextDoesNotTrustRequestScope(t *testing.T) {
	c, err := BuildTenantContext(Principal{TenantID: "tenant-a", ActorID: "user-a", Scopes: []string{"game:read"}}, "guide", "", "policy-1", []string{"kb-a"})
	if err != nil {
		t.Fatal(err)
	}
	if c.TenantID != "tenant-a" || c.AuthorizedKBs[0] != "kb-a" || !c.Scopes["game:read"] {
		t.Fatalf("unexpected context: %+v", c)
	}
	if _, err := BuildTenantContext(Principal{ActorID: "user-a"}, "guide", "", "policy-1", nil); err == nil {
		t.Fatal("expected missing tenant error")
	}
}
