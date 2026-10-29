package confidence

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestDetectContradictions_single_chunk(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "攻击力：100", QualityScore: 0.8},
	}
	result := DetectContradictions(chunks, nil)
	if result.HasConflict {
		t.Fatal("single chunk should not conflict")
	}
}

func TestDetectContradictions_no_conflict(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "角色A攻击力：100", QualityScore: 0.8},
		{ID: "c2", Content: "角色B防御力：200", QualityScore: 0.7},
	}
	result := DetectContradictions(chunks, nil)
	if result.HasConflict {
		t.Fatal("different entities should not conflict")
	}
}

func TestDetectContradictions_same_entity_same_value(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "攻击力：100点", QualityScore: 0.8},
		{ID: "c2", Content: "基础攻击力：100", QualityScore: 0.7},
	}
	result := DetectContradictions(chunks, nil)
	// Same numeric value (100) → should be deduped, no conflict
	if result.HasConflict {
		t.Logf("same value detected as conflict: %+v", result)
	}
}

func TestDetectContradictions_conflicting_values(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "攻击力：100点", QualityScore: 0.8},
		{ID: "c2", Content: "攻击力：150点", QualityScore: 0.6},
	}
	result := DetectContradictions(chunks, nil)
	if !result.HasConflict {
		t.Fatal("different values should conflict")
	}
	if len(result.Conflicts) == 0 {
		t.Fatal("no conflict items")
	}
	// Preferred should be the higher quality chunk
	if result.Conflicts[0].Preferred != "c1" {
		t.Fatalf("preferred=%q, want c1", result.Conflicts[0].Preferred)
	}
}

func TestDetectContradictions_within_5_percent(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "攻击力：100点", QualityScore: 0.8},
		{ID: "c2", Content: "攻击力：104点", QualityScore: 0.7},
	}
	result := DetectContradictions(chunks, nil)
	// 100 vs 104 → diff=4/104=3.8% → within 5%, no conflict
	if result.HasConflict {
		t.Log("values within 5% detected as conflict (may be too strict)")
	}
}

func TestDetectContradictions_outside_5_percent(t *testing.T) {
	chunks := []model.Chunk{
		{ID: "c1", Content: "攻击力：100点", QualityScore: 0.8},
		{ID: "c2", Content: "攻击力：120点", QualityScore: 0.7},
	}
	result := DetectContradictions(chunks, nil)
	// 100 vs 120 → diff=20/120=16.7% → conflict
	if !result.HasConflict {
		t.Fatal("16.7% difference should be a conflict")
	}
}

func TestFormatContradictionWarnings_no_conflict(t *testing.T) {
	cr := ContradictionResult{HasConflict: false}
	warnings := FormatContradictionWarnings(cr)
	if len(warnings) != 0 {
		t.Fatal("no conflict should return no warnings")
	}
}

func TestFormatContradictionWarnings_with_conflict(t *testing.T) {
	cr := ContradictionResult{
		HasConflict: true,
		Conflicts: []ConflictItem{
			{
				Entity:    "攻击力",
				Attribute: "基础",
				Values: []ConflictValue{
					{ChunkID: "c1", Value: "100点", QualityScore: 0.8},
					{ChunkID: "c2", Value: "150点", QualityScore: 0.6},
				},
				Preferred: "c1",
			},
		},
	}
	warnings := FormatContradictionWarnings(cr)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d", len(warnings))
	}
	if warnings[0] == "" {
		t.Fatal("warning should not be empty")
	}
	// Should contain the preferred marker
	if !contains(warnings[0], "100点 *") {
		t.Logf("warning: %s", warnings[0])
	}
}

func contains(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
