package observability

import (
	"testing"
	"time"
)

func TestStrategyPrefix(t *testing.T) {
	if strategyPrefix("hybrid_rrf+concept+rerank") != "hybrid_rrf" {
		t.Fatalf("got %q", strategyPrefix("hybrid_rrf+concept+rerank"))
	}
	if strategyPrefix("cache_l1") != "cache_l1" {
		t.Fatalf("got %q", strategyPrefix("cache_l1"))
	}
}

func TestLabelOr(t *testing.T) {
	if labelOr("", "none") != "none" {
		t.Fatal("expected none")
	}
	if labelKB("") != "_all" {
		t.Fatal("expected _all")
	}
}

func TestRecordUGCTaskIndexLagSkipsInvalidCreatedAt(t *testing.T) {
	RecordUGCTaskIndexLag("kb", "ready", "not-time", time.Now())
	RecordUGCTaskIndexLag("kb", "ready", time.Now().Add(time.Second), time.Now())
}
