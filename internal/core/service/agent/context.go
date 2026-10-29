package agent

import (
	"errors"
	"strings"
	"time"
)

// TenantContext is the only scope object accepted by the Agent application
// layer. HTTP request fields are deliberately not part of this type.
type TenantContext struct {
	TenantID       string
	ActorID        string
	AuthorizedKBs  []string
	AgentID        string
	ConversationID string
	Scopes         map[string]bool
	TraceID        string
	RequestID      string
	PolicyVersion  string
}

type Principal struct {
	TenantID string
	ActorID  string
	Scopes   []string
}

func BuildTenantContext(p Principal, agentID, conversationID, policyVersion string, kbIDs []string) (TenantContext, error) {
	if strings.TrimSpace(p.TenantID) == "" || strings.TrimSpace(p.ActorID) == "" {
		return TenantContext{}, errors.New("agent: authenticated tenant and actor are required")
	}
	if strings.TrimSpace(agentID) == "" || strings.TrimSpace(policyVersion) == "" {
		return TenantContext{}, errors.New("agent: agent and policy version are required")
	}
	scopes := make(map[string]bool, len(p.Scopes))
	for _, scope := range p.Scopes {
		if strings.TrimSpace(scope) != "" {
			scopes[scope] = true
		}
	}
	return TenantContext{TenantID: p.TenantID, ActorID: p.ActorID, AuthorizedKBs: append([]string(nil), kbIDs...), AgentID: agentID, ConversationID: conversationID, Scopes: scopes, PolicyVersion: policyVersion}, nil
}

type AgentProfile struct {
	AgentID         string
	PolicyVersion   string
	MaxSteps        int
	MaxToolCalls    int
	AllowedTools    []string
	RequireCitation bool
}

func (p AgentProfile) Limits(deadline time.Time) Limits {
	return Limits{MaxSteps: p.MaxSteps, MaxToolCalls: p.MaxToolCalls, Deadline: deadline}
}
