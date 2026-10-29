package ugc

import "testing"

func TestIsTaskQueryableOrTerminal(t *testing.T) {
	cases := []struct {
		name   string
		status map[string]any
		want   bool
	}{
		{
			name:   "ready",
			status: map[string]any{"queryable": true, "readiness": "ready"},
			want:   true,
		},
		{
			name:   "terminal failure",
			status: map[string]any{"queryable": false, "readiness": "failed"},
			want:   true,
		},
		{
			name:   "terminal chunk mismatch",
			status: map[string]any{"queryable": false, "readiness": "completed_chunk_mismatch"},
			want:   true,
		},
		{
			name:   "still indexing",
			status: map[string]any{"queryable": false, "readiness": "indexing"},
			want:   false,
		},
		{
			name:   "queued",
			status: map[string]any{"queryable": false, "readiness": "queued"},
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isTaskQueryableOrTerminal(tc.status); got != tc.want {
				t.Fatalf("got %v want %v", got, tc.want)
			}
		})
	}
}
