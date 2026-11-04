package multimodal

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graydraft/core/config"
)

func TestPDFDocumentRouteOverridesImageOnlyForceVLM(t *testing.T) {
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "document_output", true: "ocr_failure"}[fail], func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if fail {
					http.Error(w, "OCR unavailable", http.StatusBadGateway)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"output":{"markdown":"Independent document text with preserved measurement 32.4."}}`))
			}))
			defer server.Close()
			cfg := &config.Config{}
			cfg.Multimodal.ForceVLMOnly = true
			p := &Processor{cfg: cfg, docIntel: &docIntelClient{enabled: true, baseURL: server.URL, http: server.Client()}}
			// Invalid native text forces the document adapter. A nil image client
			// makes an accidental image fallback fail this test immediately.
			out, err := p.processOne(context.Background(), "scan.pdf", "application/pdf", []byte{0xff, 0x00, 0x80}, "", nil)
			if fail {
				if err == nil || !strings.Contains(err.Error(), "PDF document OCR failed") {
					t.Fatalf("%v", err)
				}
				return
			}
			if err != nil || out.Route != routeOCR || !strings.Contains(out.Text, "32.4") {
				t.Fatalf("out=%+v err=%v", out, err)
			}
		})
	}
}

func TestProviderEndpointVersionNotDuplicated(t *testing.T) {
	for _, base := range []string{"https://host/api/v1", "https://host/api/v1/", "https://host/api/v1/chat/completions"} {
		c := &dashscopeClient{baseURL: base}
		if got := c.chatCompletionsURL(); got != "https://host/api/v1/chat/completions" {
			t.Fatal(got)
		}
	}
}

func TestPDFTextOutputBudget(t *testing.T) {
	b := &pdfTextBuffer{}
	b.Grow(maxPDFTextBytes)
	if _, err := b.Write(make([]byte, maxPDFTextBytes)); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("x")); err == nil {
		t.Fatal("unbounded PDF text")
	}
}
