package agent

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
)

type RiskLevel string

const (
	RiskR0 RiskLevel = "R0"
	RiskR1 RiskLevel = "R1"
	RiskR2 RiskLevel = "R2"
	RiskR3 RiskLevel = "R3"
)

type ToolSpec struct {
	Name             string
	Version          string
	Risk             RiskLevel
	Scopes           []string
	Required         []string // top-level JSON object fields required by the tool
	TimeoutMS        int
	Idempotent       bool
	SchemaHash       string   `json:"schema_hash,omitempty"`
	Status           string   `json:"status,omitempty"`
	RunnerType       string   `json:"runner_type,omitempty"`
	RunnerCommand    string   `json:"runner_command,omitempty"`
	NetworkAllowlist []string `json:"network_allowlist,omitempty"`
	SecretRefs       []string `json:"secret_refs,omitempty"`
}

type ToolRegistry struct {
	mu    sync.RWMutex
	tools map[string]ToolSpec
}

func NewToolRegistry(specs ...ToolSpec) *ToolRegistry {
	r := &ToolRegistry{tools: make(map[string]ToolSpec, len(specs))}
	for _, s := range specs {
		r.tools[s.Name] = s
	}
	return r
}

func (r *ToolRegistry) Register(s ToolSpec) error {
	if s.Name == "" || s.Version == "" {
		return fmt.Errorf("agent: tool name and version are required")
	}
	if s.Risk == "" {
		s.Risk = RiskR0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.tools[s.Name]; exists {
		return fmt.Errorf("agent: tool %q already registered", s.Name)
	}
	r.tools[s.Name] = s
	return nil
}

func (r *ToolRegistry) Get(name string) (ToolSpec, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.tools[name]
	return s, ok
}

// Validate enforces the first server-side security boundary. Unknown fields
// are allowed for forward compatibility; required fields and JSON shape are not.
func (r *ToolRegistry) Validate(call ToolCall, grantedScopes map[string]bool) (ToolSpec, error) {
	spec, ok := r.Get(call.Name)
	if !ok {
		return ToolSpec{}, fmt.Errorf("agent: tool %q is not registered", call.Name)
	}
	if spec.Risk == RiskR3 {
		return ToolSpec{}, fmt.Errorf("agent: tool %q is disabled at R3", call.Name)
	}
	for _, scope := range spec.Scopes {
		if !grantedScopes[scope] {
			return ToolSpec{}, fmt.Errorf("agent: missing scope %q for tool %q", scope, call.Name)
		}
	}
	var obj map[string]json.RawMessage
	dec := json.NewDecoder(bytes.NewReader(call.Arguments))
	if err := dec.Decode(&obj); err != nil || obj == nil {
		return ToolSpec{}, fmt.Errorf("agent: invalid arguments for %q", call.Name)
	}
	for _, field := range spec.Required {
		if len(obj[field]) == 0 || string(obj[field]) == "null" {
			return ToolSpec{}, fmt.Errorf("agent: missing argument %q for %q", field, call.Name)
		}
	}
	return spec, nil
}
