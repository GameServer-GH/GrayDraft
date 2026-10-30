package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/graydraft/core/model"
)

type stubCatalog struct {
	docs []model.DocSummarySnippet
}

func (s stubCatalog) ListRecentDocSummaries(context.Context, string, int) ([]model.DocSummarySnippet, error) {
	return s.docs, nil
}

func TestEducationToolsCatalogDocs(t *testing.T) {
	edu := EducationTools{
		Catalog: stubCatalog{docs: []model.DocSummarySnippet{
			{DocID: "1", Title: "InternVL", Summary: "vision", DocGroup: "dg_a", IndexReady: true},
			{DocID: "2", Title: "CrystalX", Summary: "xrd", DocGroup: "dg_b", IndexReady: true},
		}},
		AllowedKB: map[string]bool{"gd_demo": true},
	}
	raw, err := edu.CatalogDocs(context.Background(), json.RawMessage(`{"kb_type":"gd_demo","query":"intern"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Total     int `json:"total"`
		Documents []struct {
			Title string `json:"title"`
		} `json:"documents"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 1 || out.Documents[0].Title != "InternVL" {
		t.Fatalf("out=%+v", out)
	}
}

func TestEducationToolsCatalogOverviewKeepsAllDocs(t *testing.T) {
	edu := EducationTools{
		Catalog: stubCatalog{docs: []model.DocSummarySnippet{
			{DocID: "1", Title: "InternVL", Summary: "vision", IndexReady: true},
			{DocID: "2", Title: "InternLM2", Summary: "language", IndexReady: true},
		}},
		AllowedKB: map[string]bool{"gd_demo": true},
	}
	raw, err := edu.CatalogDocs(context.Background(), json.RawMessage(`{"kb_type":"gd_demo","query":"三篇论文各自讲什么"}`))
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 2 {
		t.Fatalf("overview filtered the catalog: %+v", out)
	}
}

func TestCatalogOverviewGroupsPaperSlices(t *testing.T) {
	obs := []Observation{{
		Name: "knowledge.catalog",
		Output: json.RawMessage(`{"total":3,"documents":[
			{"title":"论文 InternLM2 Technical Report（1/15）"},
			{"title":"论文 InternLM2 Technical Report（4/15）"},
			{"title":"论文 InternVL（5/15）"},
			{"title":"论文 InternVL 2.5（1/14）"}
		]}`),
	}}
	got := catalogOverviewAnswer(obs)
	if !strings.Contains(got, "InternLM2 Technical Report") || !strings.Contains(got, "InternVL") || !strings.Contains(got, "InternVL 2.5") {
		t.Fatal(got)
	}
	if strings.Count(got, "InternLM2 Technical Report") != 1 {
		t.Fatal(got)
	}
}

func TestFirstHopFromIntentHybridUsesCatalog(t *testing.T) {
	if FirstHopFromIntent("hybrid") != "knowledge.catalog" {
		t.Fatal(FirstHopFromIntent("hybrid"))
	}
}
