package postgres

import (
	"fmt"
	"testing"

	"github.com/graydraft/core/model"
)

func TestPickGroupSpanPrefersEarliestTiedSliceOpening(t *testing.T) {
	all := []model.Chunk{
		{ID: "open", DocTitle: "论文 Report（5/15）", ChunkIndex: 1, Content: "We utilize InternEvo and FlashAttention with BF16."},
		{ID: "late", DocTitle: "论文 Report（5/15）", ChunkIndex: 4, Content: "InternEvo sharding Full-Replica"},
		{ID: "n1", DocTitle: "论文 Report（6/15）", ChunkIndex: 1, Content: "InternEvo later section"},
		{ID: "n2", DocTitle: "论文 Report（6/15）", ChunkIndex: 2, Content: "InternEvo repeated"},
	}
	got := pickGroupSpan(all, "论文 Report（6/15）", "n1", "InternEvo 注意力 精度", 8)
	openAt, lateAt := -1, -1
	for i, c := range got {
		switch c.ID {
		case "open":
			openAt = i
		case "late":
			lateAt = i
		}
	}
	if openAt < 0 || (lateAt >= 0 && lateAt < openAt) {
		t.Fatalf("opening not ahead of later repetition: %+v", got)
	}
}

func TestPickGroupSpanKeepsDecimalPercentSlice(t *testing.T) {
	all := []model.Chunk{
		{ID: "a", DocTitle: "论文 InternVL 2.5（1/14）", Content: "surpass 70% on MMMU, 3.7-point"},
		{ID: "b", DocTitle: "论文 InternVL 2.5（2/14）", Content: "architecture"},
		{ID: "c", DocTitle: "论文 InternVL 2.5（3/14）", Content: "MMMU with CoT reaches 70.1%"},
		{ID: "d", DocTitle: "论文 InternVL 2.5（8/14）", Content: "MMMU table"},
	}
	got := pickGroupSpan(all, "论文 InternVL 2.5（1/14）", "a", "InternVL2.5 MMMU 分数", 8)
	ids := map[string]bool{}
	for _, c := range got {
		ids[c.ID] = true
	}
	if !ids["c"] {
		t.Fatalf("decimal percent slice missing: %v", ids)
	}
	if ids["d"] {
		t.Fatal("part 8 is not the selected slice")
	}
}

func TestGroupOpeningsKeepsZeroBasedFirstChunk(t *testing.T) {
	for _, start := range []int{0, 1} {
		chunks := []model.Chunk{}
		for i := 0; i < 6; i++ {
			chunks = append(chunks, model.Chunk{ID: fmt.Sprint(i), DocID: "doc", DocTitle: "论文 source", ChunkIndex: start + i, Content: "body"})
		}
		got := pickGroupOpenings(chunks, []string{"source"})
		if len(got) != 4 || got[0].ChunkIndex != start {
			t.Fatalf("%+v", got)
		}
	}
}

func TestGroupOpeningsIncludesSiblingSegments(t *testing.T) {
	all := []model.Chunk{{ID: "header", DocID: "a", DocTitle: "论文 source", Content: "title"}, {ID: "body", DocID: "b", DocTitle: "论文 source", Content: "actual measured data", ChunkIndex: 0}}
	got := pickGroupOpenings(all, []string{"source"})
	if len(got) != 2 {
		t.Fatalf("sibling body lost: %+v", got)
	}
}

func TestGroupSpanNeverTruncatesRetrievedAnchor(t *testing.T) {
	all := []model.Chunk{}
	for i := 0; i < 20; i++ {
		all = append(all, model.Chunk{ID: fmt.Sprint(i), DocTitle: "source", ChunkIndex: i, Content: "scientific evidence"})
	}
	got := pickGroupSpan(all, "source", "19", "scientific", 4)
	if len(got) > 4 || got[0].ID != "19" {
		t.Fatalf("anchor lost: %+v", got)
	}
}

func TestRevisionSpanRetainsOpeningAndAnchorWithoutTitleCollision(t *testing.T) {
	ext := func(rev string, part float64) model.Extensions {
		return model.Extensions{"source_revision_sha256": rev, "segment_index": part}
	}
	all := []model.Chunk{{ID: "alien", DocID: "0", DocTitle: "Same title", DocExtensions: ext("other", 1), Content: "unrelated"}}
	for i := 0; i < 20; i++ {
		all = append(all, model.Chunk{ID: fmt.Sprint(i), DocID: fmt.Sprint(20 - i), DocTitle: "Same title", DocExtensions: ext("source", float64(i+1)), Content: fmt.Sprint(i)})
	}
	got := pickGroupSpan(all, "Same title", "18", "independent question", 8)
	seen := map[string]bool{}
	for _, c := range got {
		seen[c.ID] = true
	}
	if len(got) != 8 || got[0].ID != "18" || !seen["0"] || !seen["1"] || seen["alien"] {
		t.Fatalf("%+v", got)
	}
}
