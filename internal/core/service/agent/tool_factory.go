package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

const EduPolicyVersion = "edu_workspace_v1"

// ToolFactory builds tenant-scoped RegistryExecutor from defaults and optional DB specs.
type ToolFactory struct {
	Defaults *ToolRegistry
	Cache    *TenantToolCache
	Build    func(run RunRecord) map[string]ToolFunc
}

func (f ToolFactory) RegistryFor(ctx context.Context, run RunRecord) (*ToolRegistry, error) {
	reg := cloneRegistry(f.Defaults)
	if f.Cache == nil {
		return reg, nil
	}
	specs, err := f.Cache.ForTenant(ctx, run.TenantID)
	if err != nil {
		return reg, err
	}
	reg.merge(specs)
	reg.ForceEducationNotePolicy()
	return reg, nil
}

func (f ToolFactory) Executor(ctx context.Context, run RunRecord) (ToolExecutor, error) {
	reg, err := f.RegistryFor(ctx, run)
	if err != nil {
		return nil, err
	}
	funcs := map[string]ToolFunc{}
	if f.Build != nil {
		funcs = f.Build(run)
	}
	return &RegistryExecutor{
		Registry: reg,
		Scopes:   map[string]bool{"knowledge:read": true, "note:write": true},
		Funcs:    funcs,
	}, nil
}

func cloneRegistry(src *ToolRegistry) *ToolRegistry {
	if src == nil {
		return DefaultEducationRegistry()
	}
	out := NewToolRegistry()
	src.mu.RLock()
	defer src.mu.RUnlock()
	for _, s := range src.tools {
		out.tools[s.Name] = s
	}
	return out
}

func (r *ToolRegistry) merge(specs []ToolSpec) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, s := range specs {
		if s.Name == "" {
			continue
		}
		if s.Risk == "" {
			s.Risk = RiskR0
		}
		r.tools[s.Name] = s
	}
}

// ForceEducationNotePolicy pins note.draft to an automatic, cited write.
// A published tenant spec must not put the note back behind an approval queue.
func (r *ToolRegistry) ForceEducationNotePolicy() {
	if r == nil {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.tools["note.draft"]
	if !ok {
		return
	}
	s.Risk = RiskR1
	s.Required = ensureRequired(s.Required, "title")
	s.Required = ensureRequired(s.Required, "body")
	s.Required = ensureRequired(s.Required, "citations")
	r.tools["note.draft"] = s
}

func ensureRequired(fields []string, name string) []string {
	for _, f := range fields {
		if f == name {
			return fields
		}
	}
	return append(fields, name)
}

// DefaultEducationRegistry is the only tool table: four retrieval entries and note.draft.
func DefaultEducationRegistry() *ToolRegistry {
	return NewToolRegistry(
		ToolSpec{Name: "knowledge.lexical", Version: "1.0.0", Risk: RiskR0, Scopes: []string{"knowledge:read"}, Required: []string{"query"}, TimeoutMS: 15000, Idempotent: true},
		ToolSpec{Name: "knowledge.semantic", Version: "1.0.0", Risk: RiskR0, Scopes: []string{"knowledge:read"}, Required: []string{"query"}, TimeoutMS: 15000, Idempotent: true},
		ToolSpec{Name: "knowledge.read", Version: "1.0.0", Risk: RiskR0, Scopes: []string{"knowledge:read"}, Required: []string{"chunk_id"}, TimeoutMS: 8000, Idempotent: true},
		ToolSpec{Name: "knowledge.expand", Version: "1.0.0", Risk: RiskR0, Scopes: []string{"knowledge:read"}, Required: []string{"entity"}, TimeoutMS: 15000, Idempotent: true},
		ToolSpec{Name: "note.draft", Version: "1.0.0", Risk: RiskR1, Scopes: []string{"note:write"}, Required: []string{"title", "body", "citations"}, TimeoutMS: 20000, Idempotent: false},
	)
}

func DefaultR0Registry() *ToolRegistry { return DefaultEducationRegistry() }

// NoteDraftTool submits a cited study note. Citations are required; a passing
// call is ingested directly. Approval is not the publish switch.
// ForceEducationNotePolicy keeps that rule even if a tenant tool spec still says R2.
type NoteDraftTool struct {
	Ingest    func(ctx context.Context, tenantID, kbType, title, body, sourceQuery string, citations []string) (string, error)
	TenantID  string
	AllowedKB map[string]bool
}

func (t NoteDraftTool) Call(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Title       string   `json:"title"`
		Body        string   `json:"body"`
		KBType      string   `json:"kb_type,omitempty"`
		SourceQuery string   `json:"source_query,omitempty"`
		Citations   []string `json:"citations,omitempty"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Body) == "" {
		return nil, fmt.Errorf("note.draft: title and body are required")
	}
	cited := make([]string, 0, len(in.Citations))
	for _, id := range in.Citations {
		id = strings.TrimSpace(id)
		if id != "" {
			cited = append(cited, id)
		}
	}
	if len(cited) == 0 {
		return nil, fmt.Errorf("note.draft: citations are required")
	}
	in.Citations = cited
	kb := strings.TrimSpace(in.KBType)
	if kb == "" && len(t.AllowedKB) == 1 {
		for name := range t.AllowedKB {
			kb = name
		}
	}
	if kb == "" {
		kb = "note"
	}
	if len(t.AllowedKB) > 0 && !t.AllowedKB[kb] && kb != "note" {
		return nil, fmt.Errorf("note.draft: kb scope denied: %s", kb)
	}
	if t.Ingest == nil {
		return nil, fmt.Errorf("note.draft: ingest unavailable")
	}
	taskID, err := t.Ingest(ctx, t.TenantID, kb, in.Title, in.Body, in.SourceQuery, in.Citations)
	if err != nil {
		return nil, err
	}
	return json.Marshal(map[string]any{
		"status":       "queued",
		"title":        in.Title,
		"kb_type":      kb,
		"task_id":      taskID,
		"source_query": in.SourceQuery,
		"citations":    in.Citations,
	})
}
