package smartimport

import (
	"testing"
	"time"
)

func TestKnowledgeScore_Balanced(t *testing.T) {
	signal := KnowledgeScoreSignal{
		SearchHits: 50,
		Citations:  20,
		ThumbsUp:   10,
		ThumbsDown: 2,
		CreatedAt:  time.Now().AddDate(0, -1, 0),
		DocType:    "单篇说明",
	}
	score := KnowledgeScore(signal, time.Now())
	if score < 0.3 || score > 1.0 {
		t.Errorf("balanced signals score = %.3f, expected [0.3, 1.0]", score)
	}
}

func TestKnowledgeScore_AllZero(t *testing.T) {
	signal := KnowledgeScoreSignal{
		CreatedAt: time.Now().AddDate(0, -6, 0),
		DocType:   "综合合集",
	}
	score := KnowledgeScore(signal, time.Now())
	if score < 0 || score > 0.3 {
		t.Errorf("zero signals score = %.3f, expected [0, 0.3]", score)
	}
}

func TestKnowledgeScore_HighEngagement(t *testing.T) {
	signal := KnowledgeScoreSignal{
		SearchHits: 500,
		Citations:  200,
		ThumbsUp:   100,
		ThumbsDown: 1,
		CreatedAt:  time.Now().AddDate(0, 0, -7),
		DocType:    "单篇说明",
	}
	score := KnowledgeScore(signal, time.Now())
	if score < 0.7 {
		t.Errorf("high engagement score = %.3f, expected >= 0.7", score)
	}
}

func TestLogNorm(t *testing.T) {
	if logNorm(0, 100) != 0 {
		t.Error("logNorm(0) should be 0")
	}
	n := logNorm(100, 100)
	if n < 0.9 || n > 1.0 {
		t.Errorf("logNorm(100, 100) = %.3f, expected [0.9, 1.0]", n)
	}
}

func TestHalfLifeByDocType(t *testing.T) {
	if halfLifeByDocType("单篇说明") <= halfLifeByDocType("综合合集") {
		t.Error("教程 half-life should be longer than 合集")
	}
}

func TestEvaluateLifecycle_Keep(t *testing.T) {
	scores := []float64{0.5, 0.5, 0.5}
	decision := EvaluateLifecycle(scores, 1, 10)
	if decision.Action != "keep" {
		t.Errorf("stable scores should keep, got %s", decision.Action)
	}
}

func TestEvaluateLifecycle_Downgrade(t *testing.T) {
	scores := make([]float64, 35)
	for i := range scores {
		scores[i] = 0.15
	}
	decision := EvaluateLifecycle(scores, 1, 10)
	if decision.Action != "downgrade" {
		t.Errorf("30+ days below 0.2 should downgrade, got %s", decision.Action)
	}
}

func TestEvaluateLifecycle_Deprecate(t *testing.T) {
	scores := make([]float64, 65)
	for i := range scores {
		scores[i] = 0.05
	}
	decision := EvaluateLifecycle(scores, 1, 10)
	if decision.Action != "deprecate" && decision.Action != "frozen" {
		t.Errorf("60+ days below 0.1 should deprecate, got %s", decision.Action)
	}
}

func TestEvaluateLifecycle_Frozen(t *testing.T) {
	scores := make([]float64, 65)
	for i := range scores {
		scores[i] = 0.05
	}
	decision := EvaluateLifecycle(scores, 1, 1) // entityDocCount = 1
	if decision.Action != "frozen" {
		t.Errorf("single doc entity should be frozen, got %s", decision.Action)
	}
}

func TestScanCacheForLowQuality(t *testing.T) {
	entries := []CacheEntry{
		{MD5: "abc", QualityScore: 0.25, Retries: 0},
		{MD5: "def", QualityScore: 0.35, Retries: 2},
		{MD5: "ghi", QualityScore: 0.60, Retries: 0},
	}
	results := ScanCacheForLowQuality(entries)
	if len(results) != 2 {
		t.Errorf("expected 2 low-quality entries, got %d", len(results))
	}
	// def should be quarantined (quality 0.35 + retries >= 2)
	for _, r := range results {
		if r.MD5 == "def" && r.Action != "quarantine" {
			t.Errorf("entry def action = %s, want quarantine", r.Action)
		}
	}
}
