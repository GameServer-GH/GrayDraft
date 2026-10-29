package agent

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graydraft/core/model"
)

type searcherStub struct{}

func (searcherStub) Search(context.Context, model.SearchRequest) (*model.SearchResponse, error) {
	return &model.SearchResponse{Total: 1, Chunks: []model.Chunk{{ID: "c1", Content: "教程正文", KBType: "tutorial", DocTitle: "教程"}}}, nil
}

func TestLexicalKeepsTenantScope(t *testing.T) {
	tool := EducationTools{Searcher: searcherStub{}, TenantID: "t1", AllowedKB: map[string]bool{"tutorial": true}}
	out, err := tool.Lexical(context.Background(), json.RawMessage(`{"query":"InternVL 负责什么","kb_type":"tutorial"}`))
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Total          int    `json:"total"`
		RetrievalGrade string `json:"retrieval_grade"`
		Hits           []struct {
			ID         string `json:"id"`
			RecallLane string `json:"recall_lane"`
		} `json:"hits"`
	}
	if err := json.Unmarshal(out, &got); err != nil || got.Total != 1 {
		t.Fatalf("out=%s err=%v", out, err)
	}
	if got.RetrievalGrade == "" || len(got.Hits) != 1 || got.Hits[0].ID != "c1" {
		t.Fatalf("got=%+v", got)
	}
	if _, err := tool.Lexical(context.Background(), json.RawMessage(`{"query":"x","kb_type":"paper"}`)); err == nil {
		t.Fatal("expected scope denial")
	}
}

func TestLexicalAutoFillsSingleAuthorizedKB(t *testing.T) {
	tool := EducationTools{Searcher: searcherStub{}, TenantID: "t1", AllowedKB: map[string]bool{"tutorial": true}}
	out, err := tool.Lexical(context.Background(), json.RawMessage(`{"query":"InternVL 负责什么"}`))
	if err != nil {
		t.Fatalf("expected auto-fill single kb: %v", err)
	}
	var got struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal(out, &got); err != nil || got.Total != 1 {
		t.Fatalf("out=%s err=%v", out, err)
	}
}

func TestLexicalRequiresKBTypeWhenMultiple(t *testing.T) {
	tool := EducationTools{Searcher: searcherStub{}, TenantID: "t1", AllowedKB: map[string]bool{"tutorial": true, "paper": true}}
	if _, err := tool.Lexical(context.Background(), json.RawMessage(`{"query":"x"}`)); err == nil {
		t.Fatal("expected denial when kb_type omitted with multiple authorized kbs")
	}
}
