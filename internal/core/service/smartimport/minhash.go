// Package smartimport provides MinHash+LSH clustering for batch segment dedup.
// Replaces O(n²) pairwise similarity with locality-sensitive hashing to handle
// large imports (500+ segments) efficiently (optimization plan 1.2/1.6).
package smartimport

import (
	"hash/fnv"
	"math"
	"sort"
)

const (
	minhashNumHashes = 128
	minhashBands     = 16
	minhashRows      = minhashNumHashes / minhashBands // 8 rows per band
	minhashThreshold = 0.90
)

// MinHashSignature is a 128-element signature for a single segment.
type MinHashSignature [minhashNumHashes]uint64

// generateMinHashSeeds produces deterministic hash seeds.
func generateMinHashSeeds() []uint64 {
	seeds := make([]uint64, minhashNumHashes)
	for i := range seeds {
		seeds[i] = uint64(i*0x9e3779b9 + 0x517cc1b7)
	}
	return seeds
}

var minHashSeeds = generateMinHashSeeds()

// ComputeMinHash generates a MinHash signature for a string.
func ComputeMinHash(s string) MinHashSignature {
	tokens := bigramTokens(s)
	if len(tokens) == 0 {
		return MinHashSignature{}
	}

	var sig MinHashSignature
	for i := range sig {
		sig[i] = math.MaxUint64
	}

	for _, token := range tokens {
		for i, seed := range minHashSeeds {
			h := hashTokenWithSeed(token, seed)
			if h < sig[i] {
				sig[i] = h
			}
		}
	}
	return sig
}

func hashTokenWithSeed(token string, seed uint64) uint64 {
	h := fnv.New64a()
	h.Write([]byte(token))
	h.Write([]byte{byte(seed), byte(seed >> 8), byte(seed >> 16), byte(seed >> 24)})
	return h.Sum64()
}

// jaccardEstimate estimates Jaccard similarity from two MinHash signatures.
func jaccardEstimate(a, b MinHashSignature) float64 {
	match := 0
	for i := range a {
		if a[i] == b[i] {
			match++
		}
	}
	return float64(match) / float64(minhashNumHashes)
}

// LSHBandKey returns the band key for a signature band.
type LSHBandKey uint64

// bandKeys returns all LSH band keys for a signature.
func bandKeys(sig MinHashSignature) []LSHBandKey {
	keys := make([]LSHBandKey, minhashBands)
	for b := 0; b < minhashBands; b++ {
		h := fnv.New64a()
		for r := 0; r < minhashRows; r++ {
			idx := b*minhashRows + r
			h.Write([]byte{
				byte(sig[idx]),
				byte(sig[idx] >> 8),
				byte(sig[idx] >> 16),
				byte(sig[idx] >> 24),
				byte(sig[idx] >> 32),
				byte(sig[idx] >> 40),
				byte(sig[idx] >> 48),
				byte(sig[idx] >> 56),
			})
		}
		keys[b] = LSHBandKey(h.Sum64())
	}
	return keys
}

// clusterResult holds the cluster assignment for segments.
type clusterResult struct {
	Clusters [][]int // each inner slice is indices of segments in one cluster
	Singles  []int   // indices of unclustered segments
}

// ClusterSegmentsMinHash clusters segments by MinHash+LSH.
// Returns groups of indices that should be merged (similarity >= threshold).
func ClusterSegmentsMinHash(segments []string) clusterResult {
	n := len(segments)
	if n <= 1 {
		return clusterResult{Singles: indexSlice(n)}
	}

	// Compute signatures
	sigs := make([]MinHashSignature, n)
	for i, s := range segments {
		sigs[i] = ComputeMinHash(s)
	}

	// LSH bucketing
	buckets := make(map[LSHBandKey][]int)
	for i, sig := range sigs {
		for _, key := range bandKeys(sig) {
			buckets[key] = append(buckets[key], i)
		}
	}

	// Build candidate pairs from LSH buckets
	type pair struct{ a, b int }
	candidateSet := make(map[pair]bool)
	for _, indices := range buckets {
		for i := 0; i < len(indices); i++ {
			for j := i + 1; j < len(indices); j++ {
				a, b := indices[i], indices[j]
				if a > b {
					a, b = b, a
				}
				candidateSet[pair{a, b}] = true
			}
		}
	}

	// Union-Find for clustering
	parent := make([]int, n)
	for i := range parent {
		parent[i] = i
	}
	find := func(x int) int {
		for parent[x] != x {
			parent[x] = parent[parent[x]]
			x = parent[x]
		}
		return x
	}
	union := func(a, b int) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	// Verify candidates with exact MinHash estimate
	for p := range candidateSet {
		if jaccardEstimate(sigs[p.a], sigs[p.b]) >= minhashThreshold {
			union(p.a, p.b)
		}
	}

	// Collect clusters
	clusterMap := make(map[int][]int)
	for i := 0; i < n; i++ {
		root := find(i)
		clusterMap[root] = append(clusterMap[root], i)
	}

	var result clusterResult
	for _, indices := range clusterMap {
		if len(indices) > 1 {
			sort.Ints(indices)
			result.Clusters = append(result.Clusters, indices)
		} else {
			result.Singles = append(result.Singles, indices[0])
		}
	}
	sort.Ints(result.Singles)
	return result
}

// MergeClusteredSegments merges clustered segments, keeping the longest as primary.
func MergeClusteredSegments(segments []PlannedSegment, raw string, sk DocumentSkeleton) []PlannedSegment {
	if len(segments) <= 1 {
		return segments
	}

	contents := make([]string, len(segments))
	for i, seg := range segments {
		contents[i] = seg.Content
	}

	cr := ClusterSegmentsMinHash(contents)
	if len(cr.Clusters) == 0 {
		return segments
	}

	// Build result: singles first, then merged clusters
	var result []PlannedSegment

	// Add singles
	for _, idx := range cr.Singles {
		result = append(result, segments[idx])
	}

	// Merge clusters
	for _, cluster := range cr.Clusters {
		merged := mergeCluster(segments, cluster, raw, sk)
		result = append(result, merged)
	}

	// Sort by original line range
	sort.Slice(result, func(i, j int) bool {
		return result[i].LineRange[0] < result[j].LineRange[0]
	})

	return result
}

// mergeCluster merges a cluster of segments into one, using the longest as base.
func mergeCluster(segments []PlannedSegment, cluster []int, raw string, sk DocumentSkeleton) PlannedSegment {
	if len(cluster) == 0 {
		return PlannedSegment{}
	}
	if len(cluster) == 1 {
		return segments[cluster[0]]
	}

	// Find longest segment as primary
	best := cluster[0]
	bestLen := len([]rune(segments[best].Content))
	for _, idx := range cluster[1:] {
		l := len([]rune(segments[idx].Content))
		if l > bestLen {
			best = idx
			bestLen = l
		}
	}

	merged := segments[best]

	// Extend line range to cover all cluster members
	minStart := merged.LineRange[0]
	maxEnd := merged.LineRange[1]
	for _, idx := range cluster {
		if idx == best {
			continue
		}
		if segments[idx].LineRange[0] < minStart {
			minStart = segments[idx].LineRange[0]
		}
		if segments[idx].LineRange[1] > maxEnd {
			maxEnd = segments[idx].LineRange[1]
		}
	}

	merged.LineRange = [2]int{minStart, maxEnd}
	merged.Content = ExtractByLineRange(raw, minStart, maxEnd)
	merged.Title = segmentTitleFromRange(raw, minStart, maxEnd, sk.Analysis)
	merged.ContextEnhancement = buildContextEnhancement(sk.Analysis, merged.Title)
	if merged.SemanticSummary == "" {
		merged.SemanticSummary = merged.Title
	}

	return merged
}

func indexSlice(n int) []int {
	s := make([]int, n)
	for i := range s {
		s[i] = i
	}
	return s
}
