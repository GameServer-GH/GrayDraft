package smartimport

import "testing"

func TestSummarizeParentTitleFromChildTitles(t *testing.T) {
	tests := []struct {
		name   string
		titles []string
		want   string
	}{
		{
			name:   "stem separator",
			titles: []string{"InternVL · 视觉编码", "InternVL · 文本编码", "InternVL · 对齐训练"},
			want:   "InternVL",
		},
		{
			name:   "common prefix",
			titles: []string{"书生多模态接口", "书生多模态部署"},
			want:   "书生多模态",
		},
		{
			name:   "segment suffix stripped",
			titles: []string{"活动日历（1/3）", "活动日历（2/3）"},
			want:   "活动日历",
		},
		{
			name:   "single",
			titles: []string{"InternLM 部署说明"},
			want:   "InternLM 部署说明",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := SummarizeParentTitleFromChildTitles(tc.titles)
			if got != tc.want {
				t.Fatalf("got %q want %q", got, tc.want)
			}
		})
	}
}
