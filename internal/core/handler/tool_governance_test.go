package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type governanceStub struct{ actor, status string }

func (s *governanceStub) CreateApproval(_ context.Context, actor, _, _, _, _ string, _ time.Time) error {
	s.actor = actor
	return nil
}
func (s *governanceStub) Decide(_ context.Context, _, _, actor, status string) error {
	s.actor, s.status = actor, status
	return nil
}

func TestApprovalActorComesFromTrustedContext(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &governanceStub{}
	h := NewToolGovernanceHandler(func(string) ToolGovernanceAdmin { return store })
	r := gin.New()
	r.POST("/approvals", func(c *gin.Context) {
		c.Set("agent_tenant_id", "t1")
		c.Set("agent_actor_id", "trusted")
		h.RequestApproval(c)
	})
	req := httptest.NewRequest(http.MethodPost, "/approvals", strings.NewReader(`{"actor_id":"forged","run_id":"00000000-0000-0000-0000-000000000001","call_id":"c1","tool_name":"publish","arguments_hash":"abc"}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusAccepted || store.actor != "trusted" {
		t.Fatalf("status=%d actor=%q", w.Code, store.actor)
	}
}
