package classifier

import (
	"regexp"
	"strings"
	"unicode"

	"context"

	"github.com/graydraft/core/config"
	"github.com/graydraft/core/service/llmclient"
)

var (
	reVersion   = regexp.MustCompile(`(?i)v?\d+\.\d+(\.\d+)?`)
	reCode      = regexp.MustCompile(`[A-Z]{2,}[-_]?\d{2,}|[A-Z][a-z]+[A-Z][a-zA-Z]+`)
	reQuoted    = regexp.MustCompile(`["'「」『』](.+?)["'」』]`)
	reMultiKB   = regexp.MustCompile(`(?i)(教程|论文|笔记|文档|ugc|跨|对比|比较|区别|分别)`)
)

// Classifier assigns query intent in-process (<5ms, rule-based).
type Classifier struct {
	cfg *config.Config
	llm *llmclient.Client
}

// New creates a query classifier.
func New(cfg *config.Config) *Classifier {
	return &Classifier{cfg: cfg}
}

// WithLLM enables optional LLM fallback (O-09).
func (c *Classifier) WithLLM(llm *llmclient.Client) *Classifier {
	c.llm = llm
	return c
}

// Classify analyzes query text and returns intent with confidence.
func (c *Classifier) Classify(query string) (Intent, float64, string) {
	q := strings.TrimSpace(query)
	if q == "" {
		return IntentUnknown, 0, "empty_query"
	}

	scores := map[Intent]float64{
		IntentProperNoun: scoreProperNoun(q),
		IntentFact:        scoreFact(q),
		IntentConcept:     scoreConcept(q),
		IntentHybrid:      scoreHybrid(q),
	}

	best := IntentUnknown
	bestScore := 0.0
	for intent, s := range scores {
		if s > bestScore {
			bestScore = s
			best = intent
		}
	}

	if bestScore < 0.35 {
		if c.cfg != nil && c.cfg.Classifier.LLMEnabled && c.llm != nil {
			return classifyWithLLM(context.Background(), c.llm, q)
		}
		return IntentConcept, 0.5, "default_concept"
	}
	return best, clamp(bestScore, 0.35, 0.99), string(best)
}

// Route builds retrieval weights and flags for an intent.
func (c *Classifier) Route(intent Intent) RoutePlan {
	w := c.cfg.Classifier.Weights
	plan := RoutePlan{
		Intent:     intent,
		Confidence: 0.8,
		Reason:     "rule_router",
	}

	switch intent {
	case IntentProperNoun:
		plan.FulltextWeight = pick(w.ProperNoun.Fulltext, 0.78)
		plan.VectorWeight = pick(w.ProperNoun.Vector, 0.22)
	case IntentConcept:
		plan.FulltextWeight = pick(w.Concept.Fulltext, 0.22)
		plan.VectorWeight = pick(w.Concept.Vector, 0.78)
	case IntentFact:
		plan.FulltextWeight = pick(w.Fact.Fulltext, 0.58)
		plan.VectorWeight = pick(w.Fact.Vector, 0.42)
	case IntentHybrid:
		plan.FulltextWeight = pick(w.Hybrid.Fulltext, 0.45)
		plan.VectorWeight = pick(w.Hybrid.Vector, 0.55)
		plan.UseGlobalIndex = true
		plan.GlobalIndexBoost = true
	default:
		plan.FulltextWeight = 0.5
		plan.VectorWeight = 0.5
	}

	normalizeWeights(&plan)
	return plan
}

// ClassifyAndRoute is a convenience for retrieval.
func (c *Classifier) ClassifyAndRoute(query string) RoutePlan {
	if c == nil || !c.cfg.Classifier.Enabled {
		ft := 0.5
		vec := 0.5
		if c != nil {
			ft = c.cfg.Retrieval.FulltextWeight
			vec = c.cfg.Retrieval.VectorWeight
		}
		return RoutePlan{
			Intent:         IntentUnknown,
			FulltextWeight: ft,
			VectorWeight:   vec,
			Reason:         "classifier_disabled",
		}
	}
	intent, conf, reason := c.Classify(query)
	plan := c.Route(intent)
	plan.Confidence = conf
	if reason != "" {
		plan.Reason = reason
	}
	return plan
}

func scoreProperNoun(q string) float64 {
	s := 0.0
	if reVersion.MatchString(q) {
		s += 0.35
	}
	if reCode.MatchString(q) {
		s += 0.4
	}
	if reQuoted.MatchString(q) {
		s += 0.25
	}
	if hasLatinProperCase(q) {
		s += 0.35
	}
	runes := []rune(q)
	if len(runes) <= 20 && !strings.Contains(q, " ") && !strings.Contains(q, "？") {
		s += 0.2
	}
	if strings.Contains(q, "GrayDraft") || strings.Contains(q, "GRAYDRAFT") {
		s += 0.5
	}
	return s
}

func scoreFact(q string) float64 {
	s := 0.0
	factWords := []string{"什么", "多少", "何时", "哪里", "谁", "是否", "有没有", "几", "what", "when", "where", "who", "how many", "是否"}
	for _, w := range factWords {
		if strings.Contains(strings.ToLower(q), strings.ToLower(w)) {
			s += 0.35
			break
		}
	}
	if strings.HasSuffix(q, "？") || strings.HasSuffix(q, "?") {
		s += 0.15
	}
	if strings.Contains(q, "是什么") || strings.Contains(q, "什么是") {
		s += 0.45
	}
	return s
}

func scoreConcept(q string) float64 {
	s := 0.0
	words := []string{"如何", "怎么", "为什么", "原理", "机制", "介绍", "概述", "理解", "区别", "how to", "why", "explain"}
	for _, w := range words {
		if strings.Contains(strings.ToLower(q), strings.ToLower(w)) {
			s += 0.4
			break
		}
	}
	if strings.Contains(q, "自适应") || strings.Contains(q, "混合检索") || strings.Contains(q, "工作流") {
		s += 0.35
	}
	if len([]rune(q)) > 25 {
		s += 0.1
	}
	return s
}

func scoreHybrid(q string) float64 {
	s := 0.0
	if reMultiKB.MatchString(q) {
		s += 0.55
	}
	if strings.Contains(q, "和") && (strings.Contains(q, "区别") || strings.Contains(q, "对比")) {
		s += 0.5
	}
	if strings.Count(q, "、") >= 2 || strings.Count(q, ",") >= 2 {
		s += 0.25
	}
	return s
}

func hasLatinProperCase(q string) bool {
	words := strings.Fields(q)
	for _, w := range words {
		if len(w) < 2 {
			continue
		}
		if unicode.IsUpper(rune(w[0])) && containsLower(w[1:]) {
			return true
		}
	}
	return false
}

func containsLower(s string) bool {
	for _, r := range s {
		if unicode.IsLower(r) {
			return true
		}
	}
	return false
}

func pick(v, def float64) float64 {
	if v > 0 {
		return v
	}
	return def
}

func normalizeWeights(plan *RoutePlan) {
	sum := plan.FulltextWeight + plan.VectorWeight
	if sum <= 0 {
		plan.FulltextWeight = 0.5
		plan.VectorWeight = 0.5
		return
	}
	plan.FulltextWeight /= sum
	plan.VectorWeight /= sum
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
