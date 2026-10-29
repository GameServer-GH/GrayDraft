// Package smartimport provides post-import knowledge quality scoring and lifecycle
// management (optimization plan chapter 10).
//
// Tracks search hits, citations, and user feedback to compute a knowledge_score
// that drives automatic downgrading and deprecation of low-quality content.
package smartimport

import (
	"math"
	"time"
)

// KnowledgeScoreSignal holds the runtime signals for a document.
type KnowledgeScoreSignal struct {
	SearchHits   int     `json:"search_hits"`
	Citations    int     `json:"citations"`
	ThumbsUp     int     `json:"thumbs_up"`
	ThumbsDown   int     `json:"thumbs_down"`
	CreatedAt    time.Time `json:"created_at"`
	DocType      string  `json:"doc_type"`
}

// KnowledgeScore computes the composite knowledge quality score.
//
// Formula (plan 10.1):
//   score = search_hits_norm × 0.20
//         + citations_norm × 0.25
//         + thumbs_up_norm × 0.30
//         - thumbs_down_norm × 0.15
//         + time_decay × 0.10
//
// All input signals are normalized to [0, 1] before weighting.
func KnowledgeScore(signal KnowledgeScoreSignal, now time.Time) float64 {
	// Normalize signals to [0, 1] using log scaling to avoid linear dominance
	searchNorm := logNorm(float64(signal.SearchHits), 100)
	citationNorm := logNorm(float64(signal.Citations), 50)
	upNorm := logNorm(float64(signal.ThumbsUp), 30)
	downNorm := logNorm(float64(signal.ThumbsDown), 30)

	// Time decay: half-life depends on doc_type
	halfLifeDays := halfLifeByDocType(signal.DocType)
	ageDays := now.Sub(signal.CreatedAt).Hours() / 24
	decay := math.Exp(-math.Ln2 * ageDays / halfLifeDays)
	if decay < 0.05 {
		decay = 0.05 // floor to prevent total obsolescence
	}

	score := searchNorm*0.20 +
		citationNorm*0.25 +
		upNorm*0.30 -
		downNorm*0.15 +
		decay*0.10

	// Clamp to [0, 1]
	return math.Max(0, math.Min(1, score))
}

// logNorm normalizes a count to [0, 1] using log scaling.
// maxExpected is the count at which the normalized value reaches ~0.95.
func logNorm(count, maxExpected float64) float64 {
	if count <= 0 {
		return 0
	}
	if maxExpected <= 0 {
		maxExpected = 1
	}
	return math.Min(1.0, math.Log2(count+1)/math.Log2(maxExpected+1))
}

// halfLifeByDocType returns the half-life in days for a doc type.
// News/announcements decay faster; evergreen guides decay slower.
func halfLifeByDocType(docType string) float64 {
	switch docType {
	case "综合合集", "问答集":
		return 180 // 6 months
	case "单篇说明":
		return 365 // 1 year
	default:
		return 270 // 9 months
	}
}

// KnowledgeScoreDecision determines the lifecycle action for a document.
type KnowledgeScoreDecision struct {
	Score     float64 `json:"score"`
	Action    string  `json:"action"` // "keep", "downgrade", "deprecate", "frozen"
	Reason    string  `json:"reason"`
	DaysBelow float64 `json:"days_below,omitempty"`
}

// EvaluateLifecycle decides whether to downgrade, deprecate, or keep a document.
// Cold-start protection: if deprecation would zero out an entity's document count,
// the document is frozen (downgraded but not removed).
func EvaluateLifecycle(
	scores []float64, // historical scores ordered by time (oldest first)
	daysBetweenSamples float64, // average days between score samples
	entityDocCount int, // total documents for this entity in kb_type
) KnowledgeScoreDecision {
	if len(scores) == 0 {
		return KnowledgeScoreDecision{Score: 0, Action: "keep", Reason: "no data"}
	}

	latest := scores[len(scores)-1]

	// Count consecutive days below thresholds
	daysBelow02 := countConsecutiveDaysBelow(scores, 0.2, daysBetweenSamples)
	daysBelow01 := countConsecutiveDaysBelow(scores, 0.1, daysBetweenSamples)

	switch {
	case daysBelow01 >= 60:
		if entityDocCount <= 1 {
			return KnowledgeScoreDecision{
				Score: latest, Action: "frozen",
				Reason: "deprecation would zero entity coverage; ranking downgraded only",
				DaysBelow: daysBelow01,
			}
		}
		return KnowledgeScoreDecision{
			Score: latest, Action: "deprecate",
			Reason: "score < 0.1 for 60+ days",
			DaysBelow: daysBelow01,
		}
	case daysBelow02 >= 30:
		return KnowledgeScoreDecision{
			Score: latest, Action: "downgrade",
			Reason: "score < 0.2 for 30+ days",
			DaysBelow: daysBelow02,
		}
	default:
		return KnowledgeScoreDecision{
			Score: latest, Action: "keep",
			Reason: "score acceptable",
		}
	}
}

// countConsecutiveDaysBelow counts how many consecutive days (from most recent
// backward) the score has been below the threshold.
func countConsecutiveDaysBelow(scores []float64, threshold, daysBetween float64) float64 {
	if daysBetween <= 0 {
		daysBetween = 1
	}
	days := 0.0
	for i := len(scores) - 1; i >= 0; i-- {
		if scores[i] < threshold {
			days += daysBetween
		} else {
			break
		}
	}
	return days
}

// ── Low-quality cache scanner (optimization plan 3.3) ───────────────────────

// CacheScanResult holds the result of scanning a single cache entry.
type CacheScanResult struct {
	MD5          string  `json:"md5"`
	QualityScore float64 `json:"quality_score"`
	Action       string  `json:"action"` // "keep", "re_extract", "quarantine"
}

// ScanCacheForLowQuality identifies cache entries below the quality threshold
// that should be re-extracted or quarantined.
func ScanCacheForLowQuality(entries []CacheEntry) []CacheScanResult {
	var results []CacheScanResult
	for _, entry := range entries {
		qs := entry.QualityScore
		var action string
		switch {
		case qs < 0.3:
			action = "re_extract"
		case qs < 0.4 && entry.Retries >= 2:
			action = "quarantine"
		case qs < 0.4:
			action = "re_extract"
		default:
			action = "keep"
		}
		if action != "keep" {
			results = append(results, CacheScanResult{
				MD5:          entry.MD5,
				QualityScore: qs,
				Action:       action,
			})
		}
	}
	return results
}

// CacheEntry represents a multimodal cache record for scanning.
type CacheEntry struct {
	MD5          string
	QualityScore float64
	Retries      int
}
