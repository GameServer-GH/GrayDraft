// Package smartimport provides unstructured long text fallback splitting.
// When a document > 5000 runes has no headings or separators, splits at
// sentence boundaries into manageable chunks (optimization plan 7.1 / 5.4).
package smartimport

import (
	"strings"
)

// splitUnstructuredLongText splits text > 5000 runes with no structural markers
// at sentence boundaries (。！？and newlines) into chunks of 800-2000 runes.
func splitUnstructuredLongText(raw string, sk DocumentSkeleton) []PlannedSegment {
	raw = normalizeRaw(raw)
	runes := []rune(raw)
	const maxChunk = 2000
	const minChunk = 800

	if len(runes) <= maxChunk {
		return []PlannedSegment{singleSegment(raw, sk.Analysis)}
	}

	// Collect sentence boundaries
	boundaries := collectSentenceBoundaries(runes)

	var segments []PlannedSegment
	chunkStart := 0
	lastBreak := 0

	for _, boundary := range boundaries {
		if boundary-chunkStart < minChunk && boundary < len(runes) {
			lastBreak = boundary
			continue
		}
		if boundary-chunkStart > maxChunk && lastBreak > chunkStart {
			// Flush at last viable break point
			content := string(runes[chunkStart:lastBreak])
			seg := buildSegment(content, chunkStart, lastBreak, raw, sk)
			if seg.Content != "" {
				segments = append(segments, seg)
			}
			chunkStart = lastBreak
		} else if boundary-chunkStart >= minChunk {
			content := string(runes[chunkStart:boundary])
			seg := buildSegment(content, chunkStart, boundary, raw, sk)
			if seg.Content != "" {
				segments = append(segments, seg)
			}
			chunkStart = boundary
		}
		lastBreak = boundary
	}

	// Flush remaining
	if chunkStart < len(runes) {
		content := string(runes[chunkStart:])
		seg := buildSegment(content, chunkStart, len(runes), raw, sk)
		if seg.Content != "" {
			segments = append(segments, seg)
		}
	}

	if len(segments) <= 1 {
		return []PlannedSegment{singleSegment(raw, sk.Analysis)}
	}
	return segments
}

func collectSentenceBoundaries(runes []rune) []int {
	var boundaries []int
	for i, r := range runes {
		switch r {
		case '\u3002', '\uff01', '\uff1f', '\n': // 。！？and newline
			boundaries = append(boundaries, i+1)
		}
	}
	if len(boundaries) == 0 || boundaries[len(boundaries)-1] < len(runes) {
		boundaries = append(boundaries, len(runes))
	}
	return boundaries
}

func buildSegment(content string, runeStart, runeEnd int, raw string, sk DocumentSkeleton) PlannedSegment {
	content = strings.TrimSpace(content)
	if len([]rune(content)) < 40 {
		return PlannedSegment{}
	}
	return PlannedSegment{
		Content:   content,
		LineRange: [2]int{runeToLine(runeStart, raw), runeToLine(runeEnd, raw)},
		Title:     segmentTitleFromRange(raw, runeToLine(runeStart, raw), runeToLine(runeEnd, raw), sk.Analysis),
	}
}

func runeToLine(runeOffset int, raw string) int {
	runes := []rune(raw)
	count := 1
	for i := 0; i < runeOffset && i < len(runes); i++ {
		if runes[i] == '\n' {
			count++
		}
	}
	return count
}

// ── Jaccard-based smart short segment merging ───────────────────────────────

// mergeShortSegments merges short segments (< 150 runes) into adjacent segments
// using Jaccard keyword overlap as the direction signal (optimization plan 7.2).
// Complete sentences (ending with 。！？) are preserved as independent segments.
func mergeShortSegments(segs []PlannedSegment, raw string, sk DocumentSkeleton) []PlannedSegment {
	if len(segs) <= 1 {
		return segs
	}
	const shortThreshold = 150
	const minJaccardForMerge = 0.03

	var result []PlannedSegment
	i := 0
	for i < len(segs) {
		seg := segs[i]
		contentRunes := len([]rune(seg.Content))

		// Not short — keep as-is
		if contentRunes >= shortThreshold {
			result = append(result, seg)
			i++
			continue
		}

		// Complete sentence — keep independent
		if isCompleteSentence(seg.Content) {
			result = append(result, seg)
			i++
			continue
		}

		// Short, incomplete sentence — compute Jaccard with neighbors
		jPrev := 0.0
		jNext := 0.0
		if len(result) > 0 {
			jPrev = jaccardOverlap(result[len(result)-1].Content, seg.Content)
		}
		if i+1 < len(segs) {
			jNext = jaccardOverlap(seg.Content, segs[i+1].Content)
		}

		// Decide direction
		switch {
		case jPrev >= jNext && jPrev >= minJaccardForMerge && len(result) > 0:
			// Merge into previous
			prev := &result[len(result)-1]
			prev.LineRange[1] = seg.LineRange[1]
			prev.Content = ExtractByLineRange(raw, prev.LineRange[0], prev.LineRange[1])
			prev.Title = segmentTitleFromRange(raw, prev.LineRange[0], prev.LineRange[1], sk.Analysis)
			i++
		case jNext > jPrev && jNext >= minJaccardForMerge && i+1 < len(segs):
			// Merge with next
			next := segs[i+1]
			merged := PlannedSegment{
				Content:   ExtractByLineRange(raw, seg.LineRange[0], next.LineRange[1]),
				LineRange: [2]int{seg.LineRange[0], next.LineRange[1]},
				Title:     segmentTitleFromRange(raw, seg.LineRange[0], next.LineRange[1], sk.Analysis),
			}
			result = append(result, merged)
			i += 2
		default:
			// Both sides low — keep independent
			result = append(result, seg)
			i++
		}
	}
	return result
}

func isCompleteSentence(content string) bool {
	content = strings.TrimSpace(content)
	if content == "" {
		return false
	}
	runes := []rune(content)
	last := runes[len(runes)-1]
	return last == '\u3002' || last == '\uff01' || last == '\uff1f'
}

func jaccardOverlap(a, b string) float64 {
	ta := bigramTokens(a)
	tb := bigramTokens(b)
	if len(ta) == 0 && len(tb) == 0 {
		return 0
	}
	setA := make(map[string]struct{}, len(ta))
	for _, t := range ta {
		setA[t] = struct{}{}
	}
	intersection := 0
	union := len(setA)
	seen := make(map[string]struct{}, len(tb))
	for _, t := range tb {
		if _, ok := setA[t]; ok {
			if _, counted := seen[t]; !counted {
				intersection++
				seen[t] = struct{}{}
			}
		}
		if _, ok := setA[t]; !ok {
			union++
		}
	}
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
