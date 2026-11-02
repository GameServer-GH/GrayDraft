package postgres

import (
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
