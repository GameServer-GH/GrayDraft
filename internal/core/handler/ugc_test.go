package handler

import (
	"testing"
	"time"
)

func TestTaskEventInterval(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want time.Duration
	}{
		{name: "default", raw: "", want: 500 * time.Millisecond},
		{name: "invalid", raw: "bad", want: 500 * time.Millisecond},
		{name: "too small", raw: "20", want: 100 * time.Millisecond},
		{name: "valid", raw: "750", want: 750 * time.Millisecond},
		{name: "too large", raw: "9000", want: 5 * time.Second},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := taskEventInterval(tc.raw); got != tc.want {
				t.Fatalf("got %s want %s", got, tc.want)
			}
		})
	}
}

func TestRecordTaskIndexLagSkipsNonQueryable(t *testing.T) {
	recordTaskIndexLag(map[string]any{
		"queryable":  false,
		"kb_type":    "gd_demo",
		"readiness":  "queued",
		"created_at": time.Now(),
	})
}
