package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/confidence"
)

// ChunkNavigator reads a section window and entity edges inside one kb partition.
type ChunkNavigator interface {
	Window(ctx context.Context, kbType, chunkID string) ([]model.Chunk, error)
	Expand(ctx context.Context, kbType, entity string, limit int) ([]model.Chunk, error)
}

// GroupNavigator reads sibling slices of one imported work, and the opening of named documents.
type GroupNavigator interface {
	GroupSpan(ctx context.Context, kbType, chunkID, query string, limit int) ([]model.Chunk, error)
	Openings(ctx context.Context, kbType string, docIDs []string) ([]model.Chunk, error)
	GroupOpenings(ctx context.Context, kbType string, groups []string) ([]model.Chunk, error)
}

// DocCatalog lists workspace documents for theme / multi-doc questions (P2-4).
type DocCatalog interface {
	ListRecentDocSummaries(ctx context.Context, kbType string, limit int) ([]model.DocSummarySnippet, error)
}

// EducationTools is the retrieval surface used by the education tools.
type EducationTools struct {
	Searcher  Searcher
	Navigator ChunkNavigator
	Catalog   DocCatalog
	TenantID  string
	AllowedKB map[string]bool
}

func (t EducationTools) Lexical(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	return t.search(ctx, "knowledge.lexical", raw, floatPtr(1), floatPtr(0))
}

func (t EducationTools) Semantic(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	return t.search(ctx, "knowledge.semantic", raw, floatPtr(0), floatPtr(1))
}

func (t EducationTools) Read(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		ChunkID string   `json:"chunk_id"`
		DocIDs  []string `json:"doc_ids"`
		Groups  []string `json:"groups"`
		KBType  string   `json:"kb_type,omitempty"`
		Span    string   `json:"span,omitempty"`
		Query   string   `json:"query,omitempty"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("knowledge.read: invalid arguments")
	}
	kb, err := t.resolveKB(in.KBType, "knowledge.read")
	if err != nil {
		return nil, err
	}
	if t.Navigator == nil {
		return nil, fmt.Errorf("knowledge.read: navigator unavailable")
	}
	if len(in.Groups) > 0 {
		opener, ok := t.Navigator.(GroupNavigator)
		if !ok {
			return nil, fmt.Errorf("knowledge.read: openings unavailable")
		}
		chunks, err := opener.GroupOpenings(ctx, kb, in.Groups)
		if err != nil {
			return nil, err
		}
		return marshalHits("knowledge.read", "openings", kb, chunks)
	}
	if len(in.DocIDs) > 0 {
		opener, ok := t.Navigator.(GroupNavigator)
		if !ok {
			return nil, fmt.Errorf("knowledge.read: openings unavailable")
		}
		chunks, err := opener.Openings(ctx, kb, in.DocIDs)
		if err != nil {
			return nil, err
		}
		return marshalHits("knowledge.read", "openings", kb, chunks)
	}
	if strings.TrimSpace(in.ChunkID) == "" {
		return nil, fmt.Errorf("knowledge.read: chunk_id is required")
	}
	var chunks []model.Chunk
	if in.Span == "group" {
		if group, ok := t.Navigator.(GroupNavigator); ok {
			chunks, err = group.GroupSpan(ctx, kb, strings.TrimSpace(in.ChunkID), in.Query, 12)
		}
	}
	if chunks == nil && err == nil {
		chunks, err = t.Navigator.Window(ctx, kb, strings.TrimSpace(in.ChunkID))
	}
	if err != nil {
		return nil, err
	}
	return marshalHits("knowledge.read", in.ChunkID, kb, chunks)
}

func (t EducationTools) CatalogDocs(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		KBType string `json:"kb_type,omitempty"`
		Limit  int    `json:"limit,omitempty"`
		Query  string `json:"query,omitempty"`
	}
	_ = json.Unmarshal(raw, &in)
	kb, err := t.resolveKB(in.KBType, "knowledge.catalog")
	if err != nil {
		return nil, err
	}
	if t.Catalog == nil {
		return nil, fmt.Errorf("knowledge.catalog: catalog unavailable")
	}
	if in.Limit <= 0 || in.Limit > 30 {
		in.Limit = 20
	}
	docs, err := t.Catalog.ListRecentDocSummaries(ctx, kb, in.Limit)
	if err != nil {
		return nil, err
	}
	type row struct {
		DocID       string `json:"doc_id"`
		Title       string `json:"title"`
		Summary     string `json:"summary,omitempty"`
		DocGroup    string `json:"doc_group,omitempty"`
		ParentTitle string `json:"parent_title,omitempty"`
		IndexReady  bool   `json:"index_ready"`
	}
	q := strings.ToLower(strings.TrimSpace(in.Query))
	// A full question is an overview, not a title. Substring-filtering it
	// empties the catalog ("三篇论文各自讲什么" matches no title).
	if catalogOverviewQuery(q) {
		q = ""
	}
	out := make([]row, 0, len(docs))
	for _, d := range docs {
		if q != "" {
			blob := strings.ToLower(d.Title + " " + d.Summary + " " + d.ParentTitle)
			if !strings.Contains(blob, q) {
				continue
			}
		}
		out = append(out, row{
			DocID: d.DocID, Title: d.Title, Summary: d.Summary,
			DocGroup: d.DocGroup, ParentTitle: d.ParentTitle, IndexReady: d.IndexReady,
		})
	}
	return json.Marshal(map[string]any{
		"tool": "knowledge.catalog", "kb_type": kb, "query": in.Query,
		"total": len(out), "documents": out,
	})
}

func (t EducationTools) Expand(ctx context.Context, raw json.RawMessage) (json.RawMessage, error) {
	var in struct {
		Entity string `json:"entity"`
		KBType string `json:"kb_type,omitempty"`
		TopK   int    `json:"top_k,omitempty"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || strings.TrimSpace(in.Entity) == "" {
		return nil, fmt.Errorf("knowledge.expand: entity is required")
	}
	kb, err := t.resolveKB(in.KBType, "knowledge.expand")
	if err != nil {
		return nil, err
	}
	if t.Navigator == nil {
		return nil, fmt.Errorf("knowledge.expand: navigator unavailable")
	}
	if in.TopK <= 0 || in.TopK > 10 {
		in.TopK = 5
	}
	chunks, err := t.Navigator.Expand(ctx, kb, strings.TrimSpace(in.Entity), in.TopK)
	if err != nil {
		return nil, err
	}
	return marshalHits("knowledge.expand", in.Entity, kb, chunks)
}

func (t EducationTools) search(ctx context.Context, name string, raw json.RawMessage, ft, vec *float64) (json.RawMessage, error) {
	var in struct {
		Query  string `json:"query"`
		KBType string `json:"kb_type,omitempty"`
		TopK   int    `json:"top_k,omitempty"`
	}
	if err := json.Unmarshal(raw, &in); err != nil || strings.TrimSpace(in.Query) == "" {
		return nil, fmt.Errorf("%s: query is required", name)
	}
	kb, err := t.resolveKB(in.KBType, name)
	if err != nil {
		return nil, err
	}
	if t.Searcher == nil {
		return nil, fmt.Errorf("%s: retriever unavailable", name)
	}
	if in.TopK <= 0 || in.TopK > 10 {
		in.TopK = 5
	}
	ctx, span := StartAgentSpan(ctx, name, SpanKindRetrieval, 0, "", name)
	resp, err := t.Searcher.Search(ctx, model.SearchRequest{
		Query: in.Query, TenantID: t.TenantID, KBType: kb, TopK: in.TopK,
		FulltextWeight: ft, VectorWeight: vec, SkipCache: true,
	})
	span.End(err)
	if err != nil {
		return nil, err
	}
	if resp == nil {
		resp = &model.SearchResponse{}
	}
	grade := confidence.EvaluateRetrievalGrade(in.Query, resp.QueryIntent, resp.Chunks, nil)
	hits := make([]EvidenceHit, 0, len(resp.Chunks))
	for _, c := range resp.Chunks {
		hits = append(hits, EvidenceHit{
			SourceRevision: chunkSourceRevision(c), ChunkID: c.ID, DocID: c.DocID, Title: c.DocTitle, Content: c.Content, MediaURL: chunkMediaURL(c),
			Score: c.Score, RerankScore: c.RerankScore, Source: c.Source, RecallLane: normalizeRecallLane(c.Source),
		})
	}
	return json.Marshal(struct {
		Query          string        `json:"query"`
		Total          int           `json:"total"`
		Strategy       string        `json:"strategy,omitempty"`
		RetrievalGrade string        `json:"retrieval_grade"`
		CoverageScore  float64       `json:"coverage_score"`
		RetrievalScore float64       `json:"retrieval_score"`
		QueryIntent    string        `json:"query_intent,omitempty"`
		Hits           []EvidenceHit `json:"hits"`
	}{
		Query: in.Query, Total: len(hits), Strategy: resp.Strategy,
		RetrievalGrade: string(grade.Grade), CoverageScore: grade.CoverageScore, RetrievalScore: grade.Confidence,
		QueryIntent: resp.QueryIntent, Hits: hits,
	})
}

func (t EducationTools) resolveKB(kb, tool string) (string, error) {
	kb = strings.TrimSpace(kb)
	if len(t.AllowedKB) == 0 {
		return "", fmt.Errorf("%s: kb scope denied: no authorized kb", tool)
	}
	if kb == "" && len(t.AllowedKB) == 1 {
		for name := range t.AllowedKB {
			kb = name
		}
	}
	if kb == "" || !t.AllowedKB[kb] {
		allowed := make([]string, 0, len(t.AllowedKB))
		for name := range t.AllowedKB {
			allowed = append(allowed, name)
		}
		return "", fmt.Errorf("%s: kb scope denied: want one of %v, got %q", tool, allowed, kb)
	}
	return kb, nil
}

func marshalHits(tool, query, kb string, chunks []model.Chunk) (json.RawMessage, error) {
	hits := make([]EvidenceHit, 0, len(chunks))
	for _, c := range chunks {
		hits = append(hits, EvidenceHit{
			SourceRevision: chunkSourceRevision(c), ChunkID: c.ID, DocID: c.DocID, Title: c.DocTitle, Content: c.Content, MediaURL: chunkMediaURL(c),
			Score: c.Score, Source: c.Source, RecallLane: normalizeRecallLane(c.Source),
		})
	}
	return json.Marshal(map[string]any{
		"query": query, "kb_type": kb, "tool": tool, "total": len(hits), "hits": hits,
	})
}

func catalogOverviewQuery(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	if strings.ContainsAny(q, "？?，,。 ") {
		return true
	}
	return len([]rune(q)) >= 8
}

func floatPtr(v float64) *float64 { return &v }

func chunkMediaURL(c model.Chunk) string {
	if u := strings.TrimSpace(c.MediaURL); u != "" {
		return u
	}
	if u := mediaURLFromExtensions(c.Extensions); u != "" {
		return u
	}
	return mediaURLFromExtensions(c.DocExtensions)
}

func mediaURLFromExtensions(ext model.Extensions) string {
	if len(ext) == 0 {
		return ""
	}
	mm, ok := ext["multimodal"].(map[string]any)
	if !ok {
		return ""
	}
	assets, ok := mm["assets"].([]any)
	if !ok || len(assets) == 0 {
		return ""
	}
	first, ok := assets[0].(map[string]any)
	if !ok {
		return ""
	}
	u, _ := first["media_url"].(string)
	return strings.TrimSpace(u)
}

func chunkSourceRevision(c model.Chunk) string {
	v, _ := c.DocExtensions["source_revision_sha256"].(string)
	return v
}
