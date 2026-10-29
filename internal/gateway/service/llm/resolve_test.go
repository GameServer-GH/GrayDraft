package llm

import (
	"testing"

	"github.com/graydraft/gateway/config"
)

func testCfg() *config.Config {
	cfg := &config.Config{}
	cfg.Models.LLMFast.Provider = "deepseek"
	cfg.Models.LLMFast.Model = "deepseek-v4-flash"
	cfg.Models.LLM.Provider = "deepseek"
	cfg.Models.LLM.Model = "deepseek-v4-pro"
	return cfg
}

func TestResolveBinding(t *testing.T) {
	cfg := testCfg()
	cases := []struct {
		in       string
		wantModel string
		complex  bool
	}{
		{"", "deepseek-v4-flash", false},
		{"default", "deepseek-v4-flash", false},
		{"fast", "deepseek-v4-flash", false},
		{"complex", "deepseek-v4-pro", true},
		{"pro", "deepseek-v4-pro", true},
		{"deepseek-v4-flash", "deepseek-v4-flash", false},
		{"deepseek-v4-pro", "deepseek-v4-pro", true},
		{"custom-model", "custom-model", true},
	}
	for _, tc := range cases {
		_, model, gotComplex := resolveBinding(cfg, tc.in)
		if model != tc.wantModel {
			t.Fatalf("resolveBinding(%q) model=%q want %q", tc.in, model, tc.wantModel)
		}
		if gotComplex != tc.complex {
			t.Fatalf("resolveBinding(%q) complex=%v want %v", tc.in, gotComplex, tc.complex)
		}
	}
}
