// Package smartimport provides TF-IDF dynamic term extraction for adaptive
// information density scoring (optimization plan 4.3).
//
// Periodically extracts high-IDF terms from existing KB documents to enrich
// the domain term provider without hardcoded word lists. Run weekly or on-demand.
package smartimport

import (
	"math"
	"sort"
	"strings"
)

// DocumentCorpus is a set of documents for TF-IDF computation.
type DocumentCorpus struct {
	docs     []string
	docFreq  map[string]int // term → document frequency
	totalDocs int
}

// NewDocumentCorpus creates a corpus from a slice of document texts.
func NewDocumentCorpus(docs []string) *DocumentCorpus {
	dc := &DocumentCorpus{
		docs:      docs,
		docFreq:   make(map[string]int),
		totalDocs: len(docs),
	}
	dc.buildIndex()
	return dc
}

func (dc *DocumentCorpus) buildIndex() {
	for _, doc := range dc.docs {
		// Tokenize with bigram tokens
		tokens := bigramTokens(doc)
		seen := make(map[string]bool)
		for _, t := range tokens {
			if !seen[t] {
				seen[t] = true
				dc.docFreq[t]++
			}
		}
	}
}

// TFIDF computes TF-IDF scores for all terms in the corpus.
// Returns terms sorted by descending IDF (rarer terms first), up to topN.
func (dc *DocumentCorpus) TFIDF(topN int) []TermScore {
	if dc.totalDocs == 0 {
		return nil
	}

	type termScore struct {
		term  string
		idf   float64
		tfSum float64
	}
	var scores []termScore

	for term, df := range dc.docFreq {
		// IDF = log(N / df)
		idf := math.Log(float64(dc.totalDocs) / float64(df))
		// Skip very common terms (appear in > 80% of docs)
		if float64(df)/float64(dc.totalDocs) > 0.8 {
			continue
		}
		// Skip very short terms
		if len([]rune(term)) < 2 {
			continue
		}
		scores = append(scores, termScore{term: term, idf: idf})
	}

	// Sort by IDF descending (rarer = more specific = higher score)
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].idf > scores[j].idf
	})

	if topN > len(scores) {
		topN = len(scores)
	}

	result := make([]TermScore, topN)
	for i := 0; i < topN; i++ {
		// Normalize weight to [0.3, 0.9] range based on IDF rank
		weight := 0.9 - float64(i)*0.6/float64(topN)
		if weight < 0.3 {
			weight = 0.3
		}
		result[i] = TermScore{
			Term:   scores[i].term,
			Weight: math.Round(weight*100) / 100,
			IDF:    math.Round(scores[i].idf*100) / 100,
		}
	}
	return result
}

// TermScore is a weighted domain term extracted by TF-IDF.
type TermScore struct {
	Term   string  `json:"term"`
	Weight float64 `json:"weight"` // [0.3, 0.9] for info density scoring
	IDF    float64 `json:"idf"`
}

// ToDomainKeywords converts TF-IDF results to the DomainTermProvider format.
func (dc *DocumentCorpus) ToDomainKeywords(topN int) map[string]float64 {
	scores := dc.TFIDF(topN)
	kw := make(map[string]float64, len(scores))
	for _, s := range scores {
		kw[s.Term] = s.Weight
	}
	return kw
}

// TFIDFDomainTermProvider adapts TF-IDF results to the DomainTermProvider interface.
type TFIDFDomainTermProvider struct {
	keywords map[string]float64
	fallback DomainTermProvider
}

// NewTFIDFProvider creates a provider from TF-IDF extracted terms.
// Falls back to the default provider for terms not in the TF-IDF set.
func NewTFIDFProvider(corpusDocs []string, topN int, fallback DomainTermProvider) *TFIDFDomainTermProvider {
	dc := NewDocumentCorpus(corpusDocs)
	kw := dc.ToDomainKeywords(topN)

	// Merge with fallback (TF-IDF terms take precedence)
	if fallback != nil {
		for k, v := range fallback.DomainKeywords() {
			if _, exists := kw[k]; !exists {
				kw[k] = v
			}
		}
	}

	return &TFIDFDomainTermProvider{keywords: kw, fallback: fallback}
}

func (p *TFIDFDomainTermProvider) DomainKeywords() map[string]float64 {
	return p.keywords
}

// ExtractDomainTermsFromDocs is a convenience function that extracts top N domain terms
// from a set of document texts using TF-IDF.
func ExtractDomainTermsFromDocs(docs []string, topN int) []TermScore {
	if len(docs) == 0 {
		return nil
	}
	dc := NewDocumentCorpus(docs)
	return dc.TFIDF(topN)
}

// FilterMeaningfulTerms filters extracted terms to only keep those that appear
// meaningful (contain CJK characters, not pure ASCII/punctuation).
func FilterMeaningfulTerms(terms []TermScore) []TermScore {
	var filtered []TermScore
	for _, t := range terms {
		hasCJK := false
		for _, r := range t.Term {
			if r >= 0x4e00 && r <= 0x9fff {
				hasCJK = true
				break
			}
		}
		if hasCJK && len([]rune(t.Term)) >= 2 {
			filtered = append(filtered, t)
		}
	}
	return filtered
}

// MergeTermProviders merges multiple domain term providers, with later providers
// taking precedence for overlapping terms.
func MergeTermProviders(providers ...DomainTermProvider) DomainTermProvider {
	merged := make(map[string]float64)
	for _, p := range providers {
		if p == nil {
			continue
		}
		for k, v := range p.DomainKeywords() {
			k = strings.TrimSpace(k)
			if k != "" {
				merged[k] = v
			}
		}
	}
	return &TFIDFDomainTermProvider{keywords: merged}
}
