package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// ToolFunc is the narrow adapter used by game-specific R0 tools. It receives
// validated JSON only; tenant and scope checks happen before invocation.
type ToolFunc func(context.Context, json.RawMessage) (json.RawMessage, error)

type RegistryExecutor struct {
	Registry *ToolRegistry
	Scopes   map[string]bool
	Funcs    map[string]ToolFunc
}

func (e *RegistryExecutor) Execute(ctx context.Context, call ToolCall) (Observation, error) {
	obs := Observation{CallID: call.CallID, Name: call.Name}
	if e == nil || e.Registry == nil {
		return obs, fmt.Errorf("agent: tool registry unavailable")
	}
	if _, err := e.Registry.Validate(call, e.Scopes); err != nil {
		return obs, err
	}
	fn, ok := e.Funcs[call.Name]
	if !ok {
		return obs, fmt.Errorf("agent: tool %q has no executor", call.Name)
	}
	out, err := fn(ctx, call.Arguments)
	obs.Output, obs.Err = out, err
	return obs, err
}
