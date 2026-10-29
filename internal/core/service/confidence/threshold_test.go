package confidence

import (
	"testing"

	"github.com/graydraft/core/model"
)

func TestRetrievalGateThreshold_fact(t *testing.T) {
	th := RetrievalGateThreshold("fact")
	if th != gateThresholdFact {
		t.Fatalf("fact threshold: %.2f", th)
	}
}

func TestRetrievalGateThreshold_concept(t *testing.T) {
	th := RetrievalGateThreshold("concept")
	if th != gateThresholdConcept {
		t.Fatalf("concept threshold: %.2f", th)
	}
}

func TestRetrievalGateThreshold_default(t *testing.T) {
	th := RetrievalGateThreshold("")
	if th != gateThresholdDefault {
		t.Fatalf("default threshold: %.2f", th)
	}
	th = RetrievalGateThreshold("unknown")
	if th != gateThresholdDefault {
		t.Fatalf("unknown threshold: %.2f", th)
	}
}

func TestRetrievalGateThresholdDynamic_high_quality(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.85, TitleMatched: true, EntityMatched: true},
	}
	th := RetrievalGateThresholdDynamic("fact", chunks)
	// fact base=0.25, minus 0.05+0.02+0.02 = 0.09 → 0.16
	if th >= gateThresholdFact {
		t.Fatalf("high quality should reduce threshold: got %.2f, base=%.2f", th, gateThresholdFact)
	}
	if th < gateFloor {
		t.Fatalf("should not go below floor: %.2f", th)
	}
}

func TestRetrievalGateThresholdDynamic_medium_quality(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.70, TitleMatched: false, EntityMatched: false},
	}
	th := RetrievalGateThresholdDynamic("fact", chunks)
	if th != gateThresholdFact-0.03 {
		t.Fatalf("medium quality threshold: %.2f", th)
	}
}

func TestRetrievalGateThresholdDynamic_low_quality(t *testing.T) {
	chunks := []model.Chunk{
		{QualityScore: 0.30, TitleMatched: false, EntityMatched: false},
	}
	th := RetrievalGateThresholdDynamic("fact", chunks)
	if th != gateThresholdFact {
		t.Fatalf("low quality should not reduce threshold: %.2f", th)
	}
}

func TestRetrievalGateThresholdDynamic_empty_chunks(t *testing.T) {
	th := RetrievalGateThresholdDynamic("fact", nil)
	if th != gateThresholdFact {
		t.Fatalf("empty chunks threshold: %.2f", th)
	}
}

func TestRetrievalGateThresholdDynamic_floor_ceiling(t *testing.T) {
	// Force threshold below floor
	chunks := []model.Chunk{
		{QualityScore: 0.95, TitleMatched: true, EntityMatched: true},
	}
	th := RetrievalGateThresholdDynamic("concept", chunks)
	// concept base = 0.15, minus ~0.09 = 0.06, floor is 0.12
	if th != gateFloor {
		t.Fatalf("should hit floor: got %.2f, want %.2f", th, gateFloor)
	}
}

func TestComputeBaseCoverage_gradeA(t *testing.T) {
	cov := computeBaseCoverage(GradeA, 0.85, 5)
	if cov < 0.80 {
		t.Fatalf("grade A coverage too low: %.2f", cov)
	}
}

func TestComputeBaseCoverage_gradeB(t *testing.T) {
	cov := computeBaseCoverage(GradeB, 0.65, 3)
	if cov < 0.40 {
		t.Fatalf("grade B coverage too low: %.2f", cov)
	}
}

func TestComputeBaseCoverage_gradeC(t *testing.T) {
	cov := computeBaseCoverage(GradeC, 0.30, 3)
	if cov < 0.10 {
		t.Fatalf("grade C coverage too low: %.2f", cov)
	}
}

func TestComputeBaseCoverage_gradeD(t *testing.T) {
	cov := computeBaseCoverage(GradeD, 0, 0)
	if cov != 0 {
		t.Fatalf("grade D coverage should be 0: %.2f", cov)
	}
}
