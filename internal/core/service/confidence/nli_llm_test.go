package confidence

import (
	"context"
	"testing"
)

func TestNormalizeNLILabel(t *testing.T) {
	cases := map[string]string{
		"entailment":                 "entailment",
		"Entailment.":                "entailment",
		"label: contradiction":       "contradiction",
		"neutral\n":                  "neutral",
		"something else":             "neutral",
	}
	for in, want := range cases {
		if got := normalizeNLILabel(in); got != want {
			t.Fatalf("%q → %q, want %q", in, got, want)
		}
	}
}

type stubCompleter struct{ out string }

func (s stubCompleter) Complete(context.Context, string) (string, error) {
	return s.out, nil
}

func TestLLMCoverageNLI(t *testing.T) {
	nli := LLMCoverageNLI{LLM: stubCompleter{out: "entailment"}}
	label, err := nli.CheckEntailment(context.Background(), []string{"版本是 1.2.3"}, "当前版本是 1.2.3")
	if err != nil || label != "entailment" {
		t.Fatalf("label=%s err=%v", label, err)
	}
}
