package handler

import (
	"net/http"
	"net/url"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProxyLogFields(t *testing.T) {
	gin.SetMode(gin.TestMode)
	cases := []struct {
		name       string
		rawURL     string
		wantTaskID string
		wantDocID  string
		wantKBType string
		wantReady  string
	}{
		{
			name:       "ugc task status",
			rawURL:     "/api/v1/ugc/tasks/task-1/events?readiness=queued",
			wantTaskID: "task-1",
			wantReady:  "queued",
		},
		{
			name:       "kb document path",
			rawURL:     "/api/v1/kb/gd_demo/documents/doc-1",
			wantDocID:  "doc-1",
			wantKBType: "gd_demo",
		},
		{
			name:       "query fallback",
			rawURL:     "/api/v1/retrieval/search?kb_type=gd_query&doc_id=doc-query",
			wantDocID:  "doc-query",
			wantKBType: "gd_query",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			u, err := url.Parse(tc.rawURL)
			if err != nil {
				t.Fatal(err)
			}
			ctx := &gin.Context{Request: &http.Request{URL: u}}
			got := proxyLogFields(ctx)
			if got.taskID != tc.wantTaskID || got.docID != tc.wantDocID || got.kbType != tc.wantKBType || got.readiness != tc.wantReady {
				t.Fatalf("got %+v", got)
			}
		})
	}
}
