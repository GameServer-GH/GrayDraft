package smartimport

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	"github.com/graydraft/core/model"
)

// MergeSegmentExtensions injects 2.0 context anchors (Anchor A/B/C) into UGC extensions.
func MergeSegmentExtensions(base model.Extensions, analysis DocAnalysis, seg PlannedSegment, index, total int) model.Extensions {
	out := cloneExtensions(base)
	if analysis.Subject != "" {
		out["subject"] = analysis.Subject
	}
	if analysis.ParentTitle != "" {
		out["parent_title"] = analysis.ParentTitle
	}
	if group := DocGroupID(analysis.ParentTitle, analysis.Subject, index, total); group != "" {
		out["doc_group"] = group
	}
	if analysis.Version != "" {
		out["version"] = analysis.Version
	}
	if analysis.DocType != "" {
		out["doc_type"] = analysis.DocType
	}
	if seg.ContextEnhancement != "" {
		out["context_enhancement"] = seg.ContextEnhancement
	}
	if seg.SearchIntent != "" {
		out["search_intent"] = seg.SearchIntent
	}
	if seg.SemanticSummary != "" {
		out["semantic_summary"] = seg.SemanticSummary
	}
	if seg.LineRange[0] > 0 {
		out["line_range"] = []int{seg.LineRange[0], seg.LineRange[1]}
	}
	if index > 0 && total > 0 {
		out["segment_index"] = index
		out["segment_total"] = total
	}
	if len(seg.Tags) > 0 {
		out["segment_tags"] = seg.Tags
	}
	if tags := DeriveSegmentSemantic4D(base, seg); tags != nil {
		out["semantic_tags_4d"] = tags
	}
	return out
}

// MergeDocumentExtensions copies document-level extensions (e.g. semantic pivot) into segment payload.
func MergeDocumentExtensions(base, doc model.Extensions) model.Extensions {
	return MergePivotExtensions(base, doc)
}

func cloneExtensions(base model.Extensions) model.Extensions {
	if base == nil {
		return model.Extensions{}
	}
	out := make(model.Extensions, len(base))
	for k, v := range base {
		out[k] = v
	}
	return out
}

// DocGroupID builds a stable source-level group key for multi-segment imports (P2-5).
func DocGroupID(parentTitle, subject string, index, total int) string {
	seed := parentTitle
	if seed == "" {
		seed = subject
	}
	if seed == "" && total <= 1 {
		return ""
	}
	if seed == "" {
		seed = fmt.Sprintf("seg-%d-of-%d", index, total)
	}
	sum := sha256.Sum256([]byte(seed))
	return "dg_" + hex.EncodeToString(sum[:8])
}
