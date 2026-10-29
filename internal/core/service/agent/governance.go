package agent

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

type ApprovalStore interface {
	IsApproved(context.Context, string, string, string, string) bool
}
type AuditRecorder interface {
	Record(context.Context, string, string, string, RiskLevel, string, string, string, time.Duration, error)
}
type ApprovalManager struct {
	mu       sync.RWMutex
	approved map[string]time.Time
}

func NewApprovalManager() *ApprovalManager { return &ApprovalManager{approved: map[string]time.Time{}} }
func (m *ApprovalManager) Approve(runID, callID string, ttl time.Duration) {
	m.mu.Lock()
	m.approved[runID+":"+callID] = time.Now().Add(ttl)
	m.mu.Unlock()
}
func (m *ApprovalManager) IsApproved(_ context.Context, runID, callID, _, _ string) bool {
	m.mu.RLock()
	t, ok := m.approved[runID+":"+callID]
	m.mu.RUnlock()
	return ok && time.Now().Before(t)
}

type GovernedExecutor struct {
	Registry  *ToolRegistry
	Inner     ToolExecutor
	Scopes    map[string]bool
	Approvals ApprovalStore
	Audit     AuditRecorder
	RunID     string
	Timeout   time.Duration
	MaxOutput int
	Runner    SandboxRunner
	Secrets   SecretProvider
}

func (g GovernedExecutor) Execute(ctx context.Context, call ToolCall) (Observation, error) {
	start := time.Now()
	if g.Registry == nil || g.Inner == nil {
		err := fmt.Errorf("agent: governed executor is not configured")
		return Observation{CallID: call.CallID, Name: call.Name, Err: err}, err
	}
	spec, err := g.Registry.Validate(call, g.Scopes)
	if err != nil {
		return Observation{CallID: call.CallID, Name: call.Name, Err: err}, err
	}
	if spec.Risk == RiskR3 {
		return Observation{CallID: call.CallID, Name: call.Name, Err: fmt.Errorf("tool R3 disabled")}, fmt.Errorf("tool R3 disabled")
	}
	if spec.Risk == RiskR2 && (g.Approvals == nil || !g.Approvals.IsApproved(ctx, g.RunID, call.CallID, call.Name, ArgumentsHash(call))) {
		err := fmt.Errorf("%w for tool %q", ErrApprovalRequired, call.Name)
		return Observation{CallID: call.CallID, Name: call.Name, Err: err}, err
	}
	timeout := g.Timeout
	if spec.TimeoutMS > 0 {
		timeout = time.Duration(spec.TimeoutMS) * time.Millisecond
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	c, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	var obs Observation
	if spec.RunnerType == "docker" {
		if g.Runner == nil {
			err = fmt.Errorf("sandbox runner is not configured")
		} else {
			secrets, secretErr := SecretEnv(c, g.Secrets, spec.SecretRefs)
			if secretErr != nil {
				err = secretErr
			} else {
				obs, err = g.Runner.Run(c, spec, call, secrets)
			}
		}
	} else {
		obs, err = g.Inner.Execute(c, call)
	}
	if len(obs.Output) > g.MaxOutput && g.MaxOutput > 0 {
		obs.Output = obs.Output[:g.MaxOutput]
		obs.Err = fmt.Errorf("tool output limit exceeded")
		err = obs.Err
	}
	if g.Audit != nil {
		g.Audit.Record(ctx, g.RunID, call.CallID, call.Name, spec.Risk, ArgumentsHash(call), OutputHash(obs.Output), status(err), time.Since(start), err)
	}
	return obs, err
}
func status(err error) string {
	if err == nil {
		return "success"
	}
	return "error"
}
func ArgumentsHash(c ToolCall) string {
	h := sha256.Sum256(c.Arguments)
	return hex.EncodeToString(h[:])
}

func OutputHash(output []byte) string {
	h := sha256.Sum256(output)
	return hex.EncodeToString(h[:])
}
