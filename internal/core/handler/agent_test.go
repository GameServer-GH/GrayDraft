package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/graydraft/core/service/agent"
)

type agentStoreStub struct {
	run    agent.RunRecord
	events []agent.RunEvent
}

func (s *agentStoreStub) CreateRun(_ context.Context, run agent.RunRecord) (agent.RunRecord, error) {
	s.run = run
	return run, nil
}
func (s *agentStoreStub) AppendEvent(_ context.Context, ev agent.RunEvent) error {
	s.events = append(s.events, ev)
	return nil
}
func (s *agentStoreStub) ListEvents(_ context.Context, _, _ string, _, _ int64) ([]agent.RunEvent, error) {
	return s.events, nil
}
func (s *agentStoreStub) CancelRun(_ context.Context, _, _ string) error {
	s.run.Status = agent.StateCancelled
	return nil
}
func (s *agentStoreStub) UpdateStatus(_ context.Context, _ string, status agent.State, _, _ int) error {
	s.run.Status = status
	return nil
}

type launcherStub struct{}

func (launcherStub) Launch(agent.RunRecord, agent.Limits) error { return nil }

func TestAgentHandlerRequiresTenantHeaders(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.POST("/api/v2/agent-runs", NewAgentHandler(&agentStoreStub{}).WithLauncher(launcherStub{}).CreateRun)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/agent-runs", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAgentHandlerCreatesQueuedRunAndEvent(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := &agentStoreStub{}
	r := gin.New()
	r.POST("/api/v2/agent-runs", NewAgentHandler(store).WithLauncher(launcherStub{}).CreateRun)
	req := httptest.NewRequest(http.MethodPost, "/api/v2/agent-runs", strings.NewReader(`{"agent_id":"guide","policy_version":"p1","idempotency_key":"k1","prompt":"版本？"}`))
	req.Header.Set("X-Tenant-ID", "t1")
	req.Header.Set("X-Actor-ID", "u1")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusAccepted || len(store.events) != 1 {
		t.Fatalf("status=%d events=%d body=%s", rec.Code, len(store.events), rec.Body.String())
	}
}
