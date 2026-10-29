package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// LauncherReplayConfig wires golden replay through the production Launcher.
type LauncherReplayConfig struct {
	Launcher      *Launcher
	PolicyVersion string
	AgentID       string
	ActorID       string
	AuthorizedKBs []string
	Limits        Limits
	BatchID       string
}

// NewLauncherReplayExecutor returns a ReplayExecutor that runs each case via Launcher.Run.
func NewLauncherReplayExecutor(cfg LauncherReplayConfig) ReplayExecutor {
	if cfg.ActorID == "" {
		cfg.ActorID = "eval-runner"
	}
	if cfg.AgentID == "" {
		cfg.AgentID = "eval"
	}
	if cfg.BatchID == "" {
		cfg.BatchID = uuid.NewString()
	}
	if len(cfg.AuthorizedKBs) == 0 {
		cfg.AuthorizedKBs = []string{"gd_console_demo"}
	}
	return func(c GoldenCase) (ReplayResult, error) {
		if cfg.Launcher == nil || cfg.Launcher.Store == nil {
			return ReplayResult{}, fmt.Errorf("agent replay: launcher unavailable")
		}
		deadline := cfg.Limits.Deadline
		if deadline.IsZero() {
			deadline = time.Now().Add(90 * time.Second)
		}
		limits := cfg.Limits.normalized()
		limits.Deadline = deadline
		run := RunRecord{
			RunID:          uuid.NewString(),
			TenantID:       c.TenantID,
			ActorID:        cfg.ActorID,
			AgentID:        cfg.AgentID,
			PolicyVersion:  cfg.PolicyVersion,
			Prompt:         []byte(c.Query),
			AuthorizedKBs:  append([]string(nil), cfg.AuthorizedKBs...),
			Deadline:       deadline,
			IdempotencyKey: fmt.Sprintf("eval:%s:%s", cfg.BatchID, c.ID),
		}
		if c.Version != "" {
			run.Prompt = []byte("version=" + c.Version + "\n" + string(run.Prompt))
		}
		result, err := cfg.Launcher.Run(context.Background(), run, limits)
		replay := ResultToReplay(c.ID, result)
		if err != nil && result.State == "" {
			return replay, err
		}
		return replay, nil
	}
}

// ResultToReplay maps a completed Agent Result into offline scoring input.
func ResultToReplay(caseID string, result Result) ReplayResult {
	tools := uniqueToolNames(result.Observations)
	bundle := CollectEvidence(result.Observations)
	citations := countAnswerCitations(result.Answer, bundle)
	return ReplayResult{
		CaseID:        caseID,
		Answer:        result.Answer,
		Tools:         tools,
		CitationCount: citations,
		Refused:       IsRefusalAnswer(result.Answer),
	}
}

func uniqueToolNames(obs []Observation) []string {
	seen := map[string]bool{}
	var out []string
	for _, o := range obs {
		if o.Name == "" || seen[o.Name] {
			continue
		}
		seen[o.Name] = true
		out = append(out, o.Name)
	}
	return out
}

func countAnswerCitations(answer string, bundle EvidenceBundle) int {
	for _, marker := range []string{"[cite:", "[[doc", "[doc:", "引用["} {
		if n := strings.Count(answer, marker); n > 0 {
			return n
		}
	}
	if len(bundle.Hits) > 0 {
		return len(bundle.Hits)
	}
	return 0
}

// IsRefusalAnswer detects safe refusal responses from the answer gate.
func IsRefusalAnswer(answer string) bool {
	answer = strings.TrimSpace(answer)
	if answer == "" {
		return true
	}
	for _, prefix := range []string{
		"当前知识库未找到可靠参考",
		"检索到的版本信息存在冲突",
		"当前证据不足",
		"无法给出可靠答案",
		"无法回答该问题",
	} {
		if strings.Contains(answer, prefix) {
			return true
		}
	}
	return false
}
