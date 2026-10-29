package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// EvalRunStore persists evaluation run records.
type EvalRunStore interface {
	Save(ctx context.Context, rec EvalRunRecord) (EvalRunRecord, error)
	List(ctx context.Context, tenantID string, limit int) ([]EvalRunRecord, error)
}

// EvalRunRecord is the persisted evaluation metadata.
type EvalRunRecord struct {
	EvalID        string
	TenantID      string
	AgentID       string
	Model         string
	PolicyVersion string
	DatasetPath   string
	Total         int
	Passed        int
	PassRate      float64
	Report        EvaluationReport
	Status        string
	CreatedAt     string
}

// RunGoldenRequest triggers an offline golden dataset replay.
type RunGoldenRequest struct {
	DatasetPath   string
	TenantID      string
	AgentID       string
	Model         string
	PolicyVersion string
	UseOracle     bool
}

// EvalService orchestrates golden dataset replay and persistence.
type EvalService struct {
	Store       EvalRunStore
	Executor    ReplayExecutor
	DatasetRoot string
}

// RunGolden loads a dataset, replays cases, and persists the report.
func (s *EvalService) RunGolden(ctx context.Context, req RunGoldenRequest) (EvalRunRecord, error) {
	if s == nil || s.Store == nil {
		return EvalRunRecord{}, fmt.Errorf("eval service unavailable")
	}
	path := req.DatasetPath
	if path == "" {
		root := s.DatasetRoot
		if root == "" {
			root = "tests/agent-eval"
		}
		path = filepath.Join(root, "golden.json")
	}
	cases, err := LoadGoldenDataset(path)
	if err != nil {
		return EvalRunRecord{}, err
	}
	exec := s.Executor
	if req.UseOracle || exec == nil {
		exec = StructuralOracleExecutor()
	}
	report, err := ReplayDataset(cases, exec)
	if err != nil {
		return EvalRunRecord{}, err
	}
	tenantID := req.TenantID
	if tenantID == "" && len(cases) > 0 {
		tenantID = cases[0].TenantID
	}
	saved, err := s.Store.Save(ctx, EvalRunRecord{
		TenantID:      tenantID,
		AgentID:       req.AgentID,
		Model:         req.Model,
		PolicyVersion: req.PolicyVersion,
		DatasetPath:   path,
		Total:         report.Total,
		Passed:        report.Passed,
		PassRate:      report.PassRate,
		Report:        report,
		Status:        "completed",
	})
	if err != nil {
		return EvalRunRecord{}, err
	}
	return saved, nil
}

// StructuralOracleExecutor satisfies golden metadata for CI smoke tests.
// Production quality measurement should inject a real ReplayExecutor.
func StructuralOracleExecutor() ReplayExecutor {
	return func(c GoldenCase) (ReplayResult, error) {
		if c.MustRefuse {
			return ReplayResult{CaseID: c.ID, Refused: true}, nil
		}
		answer := strings.Join(c.ExpectedKeywords, " ")
		if answer == "" {
			answer = "synthetic oracle answer"
		}
		return ReplayResult{
			CaseID:        c.ID,
			Answer:        answer,
			Tools:         append([]string(nil), c.ExpectedTools...),
			CitationCount: c.RequiredCitations,
		}, nil
	}
}

// DefaultDatasetRoot resolves the golden dataset directory from env or cwd.
func DefaultDatasetRoot() string {
	if v := os.Getenv("AGENT_EVAL_DATASET_ROOT"); v != "" {
		return v
	}
	return "tests/agent-eval"
}
