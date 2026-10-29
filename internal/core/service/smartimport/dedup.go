package smartimport

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/repository/postgres"
)

// DedupResult describes whether to create, update, or skip ingest.
type DedupResult struct {
	Action          string
	ExistingDocID   string
	ExistingVersion int64
	Reason          string
}

// ResolveDedup implements three-layer dedup architecture (optimization plan 1.1):
//   Layer 1: Exact content_hash match → unchanged.
//   Layer 2: Entity-normalized title + edit distance / LCS → candidate duplicate.
//   Layer 3: Embedding semantic similarity (caller must supply similarity score).
//
// targetDocID non-empty: force update that document; empty: check layers.
// For update scenarios with entity-aware downgrade (plan 1.4):
//   If primaryGameA != primaryGameB, effective similarity *= 0.7.
func ResolveDedup(ctx context.Context, docs *postgres.DocumentRepo, draft Draft, targetDocID string) (DedupResult, error) {
	return ResolveDedupWithEntity(ctx, docs, draft, targetDocID, "", 0)
}

// ResolveDedupWithEntity adds entity-aware dedup with optional embedding similarity score.
// primaryGame is the normalized entity name; similarity is the embedding cosine score (0 if unavailable).
func ResolveDedupWithEntity(ctx context.Context, docs *postgres.DocumentRepo, draft Draft, targetDocID, primaryGame string, similarity float64) (DedupResult, error) {
	targetDocID = strings.TrimSpace(targetDocID)
	if targetDocID != "" {
		existing, err := docs.GetPanelDocument(ctx, draft.KBType, targetDocID)
		if err != nil {
			return DedupResult{}, err
		}
		if existing == nil {
			return DedupResult{}, fmt.Errorf("target document %s not found in kb_type %s", targetDocID, draft.KBType)
		}
		// Layer 1: exact hash match
		byHash, err := docs.FindByContentHash(ctx, draft.KBType, draft.ContentHash)
		if err != nil {
			return DedupResult{}, err
		}
		if byHash != nil && byHash.ID == existing.ID {
			return DedupResult{
				Action:          "unchanged",
				ExistingDocID:   existing.ID,
				ExistingVersion: existing.KBVersion,
				Reason:          "target document: same content_hash (layer 1)",
			}, nil
		}
		return DedupResult{
			Action:          "update",
			ExistingDocID:   existing.ID,
			ExistingVersion: existing.KBVersion,
			Reason:          "target document selected for update",
		}, nil
	}

	// Layer 1: exact hash
	byHash, err := docs.FindByContentHash(ctx, draft.KBType, draft.ContentHash)
	if err != nil {
		return DedupResult{}, err
	}
	if byHash != nil {
		return DedupResult{
			Action:          "unchanged",
			ExistingDocID:   byHash.ID,
			ExistingVersion: byHash.KBVersion,
			Reason:          "same content_hash (layer 1)",
		}, nil
	}

	// Layer 2: title match (entity normalization deferred to caller via similarity param)
	if draft.Title != "" {
		byTitle, err := docs.FindByTitle(ctx, draft.KBType, draft.Title)
		if err != nil {
			return DedupResult{}, err
		}
		if byTitle != nil {
			if byTitle.ContentHash == draft.ContentHash {
				return DedupResult{
					Action:          "unchanged",
					ExistingDocID:   byTitle.ID,
					ExistingVersion: byTitle.KBVersion,
					Reason:          "same title + hash (layer 2)",
				}, nil
			}
			return DedupResult{
				Action:          "update",
				ExistingDocID:   byTitle.ID,
				ExistingVersion: byTitle.KBVersion,
				Reason:          "matched title, content changed (layer 2)",
			}, nil
		}
	}

	// Layer 3: embedding similarity (caller-supplied)
	if similarity > 0 {
		threshold := dedupThresholdByDocType("")
		if primaryGame == "" {
			// Entity-missing fallback (plan 1.5): no high-confidence entity extracted,
			// raise threshold to 0.92 to prevent false positive dedup.
			threshold = 0.92
		}
		if similarity >= threshold {
			return DedupResult{
				Action: "blocked_similar",
				Reason: fmt.Sprintf("semantic similarity %.2f >= threshold %.2f (layer 3, entity_missing=%v)", similarity, threshold, primaryGame == ""),
			}, nil
		}
	}

	return DedupResult{Action: "create", Reason: "new document (no duplicate detected across 3 layers)"}, nil
}

// dedupThresholdByDocType returns the dedup rejection threshold for a given doc type.
func dedupThresholdByDocType(docType string) float64 {
	switch strings.TrimSpace(docType) {
	case "单篇说明":
		return 0.80
	case "综合合集", "多模态合并":
		return 0.85
	case "问答集":
		return 0.82
	default:
		return 0.85
	}
}

// ApplyEntityDedupDowngrade adjusts similarity for entity-aware dedup (plan 1.4).
// When comparing documents with different subject entities, similarity is
// downgraded by 0.7 to avoid false positives on structurally-similar series guides.
func ApplyEntityDedupDowngrade(similarity float64, entityA, entityB string) float64 {
	entityA = strings.TrimSpace(entityA)
	entityB = strings.TrimSpace(entityB)
	if entityA == "" || entityB == "" {
		return similarity
	}
	if entityA != entityB {
		return similarity * 0.7
	}
	return similarity
}

// resolveDedupLegacy matches by content hash / title (tests and optional future use).
func resolveDedupLegacy(ctx context.Context, docs *postgres.DocumentRepo, draft Draft) (DedupResult, error) {
	byHash, err := docs.FindByContentHash(ctx, draft.KBType, draft.ContentHash)
	if err != nil {
		return DedupResult{}, err
	}
	if byHash != nil {
		if byHash.ContentHash == draft.ContentHash {
			return DedupResult{
				Action:          "unchanged",
				ExistingDocID:   byHash.ID,
				ExistingVersion: byHash.KBVersion,
				Reason:          "same content_hash",
			}, nil
		}
	}

	byTitle, err := docs.FindByTitle(ctx, draft.KBType, draft.Title)
	if err != nil {
		return DedupResult{}, err
	}
	if byTitle != nil {
		if byTitle.ContentHash == draft.ContentHash {
			return DedupResult{
				Action:          "unchanged",
				ExistingDocID:   byTitle.ID,
				ExistingVersion: byTitle.KBVersion,
				Reason:          "same title and content_hash",
			}, nil
		}
		return DedupResult{
			Action:          "update",
			ExistingDocID:   byTitle.ID,
			ExistingVersion: byTitle.KBVersion,
			Reason:          "matched title, content changed",
		}, nil
	}

	if byHash != nil {
		return DedupResult{
			Action:          "update",
			ExistingDocID:   byHash.ID,
			ExistingVersion: byHash.KBVersion,
			Reason:          "matched content lineage",
		}, nil
	}

	return DedupResult{Action: "create", Reason: "new document"}, nil
}
