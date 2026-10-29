package smartimport

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reBookTitle    = regexp.MustCompile(`[《「]([^》」\n]{2,80})[》」]`)
	reMarkdownAnyH = regexp.MustCompile(`^(#{1,6})\s+(.+)$`)
	reBoldLine     = regexp.MustCompile(`^\s*\*\*[^*]+\*\*\s*$`)
	reSkeletonSep  = regexp.MustCompile(`^\s*[-=*_]{3,}\s*$`)
)

// SkeletonLine describes one line in the structure pre-scan.
type SkeletonLine struct {
	LineNo int    `json:"line_no"` // 1-based
	Kind   string `json:"kind"`    // h1|h2|h3|h4|sep|bold|qa|text|empty
	Text   string `json:"text,omitempty"`
}

// DocAnalysis is macro-level document understanding (2.0 doc_analysis).
type DocAnalysis struct {
	Subject string `json:"subject,omitempty"`
	Version     string `json:"version,omitempty"`
	DocType     string `json:"doc_type"` // 单篇说明|综合合集|分段文档|问答集
	ParentTitle string `json:"parent_title,omitempty"`
}

// DocumentSkeleton is the full structure scan result.
type DocumentSkeleton struct {
	LineCount int            `json:"line_count"`
	Lines     []SkeletonLine `json:"lines"`
	Analysis  DocAnalysis    `json:"doc_analysis"`
}

// ScanSkeleton builds a line-numbered structure summary without sending full body to LLM.
func ScanSkeleton(raw string) DocumentSkeleton {
	raw = normalizeRaw(raw)
	textLines := strings.Split(raw, "\n")
	sk := DocumentSkeleton{
		LineCount: len(textLines),
		Lines:     make([]SkeletonLine, 0, len(textLines)),
	}
	for i, line := range textLines {
		no := i + 1
		trim := strings.TrimSpace(line)
		kind := "text"
		switch {
		case trim == "":
			kind = "empty"
		case reSkeletonSep.MatchString(line):
			kind = "sep"
		case reMarkdownAnyH.MatchString(trim):
			if m := reMarkdownAnyH.FindStringSubmatch(trim); len(m) >= 2 {
				switch len(m[1]) {
				case 1:
					kind = "h1"
				case 2:
					kind = "h2"
				case 3:
					kind = "h3"
				default:
					kind = "h4"
				}
			}
		case reBoldLine.MatchString(trim):
			kind = "bold"
		case reQALine.MatchString(trim):
			kind = "qa"
		}
		entry := SkeletonLine{LineNo: no, Kind: kind}
		if kind == "h1" || kind == "h2" || kind == "h3" || kind == "h4" || kind == "bold" {
			if m := reMarkdownAnyH.FindStringSubmatch(trim); len(m) > 2 {
				entry.Text = strings.TrimSpace(m[2])
			} else {
				entry.Text = strings.TrimSpace(trim)
			}
		}
		sk.Lines = append(sk.Lines, entry)
	}
	sk.Analysis = inferDocAnalysis(raw, sk.Lines)
	return sk
}

func inferDocAnalysis(raw string, lines []SkeletonLine) DocAnalysis {
	a := DocAnalysis{DocType: "单篇说明"}
	if m := reBookTitle.FindStringSubmatch(raw); len(m) > 1 {
		a.Subject = strings.TrimSpace(m[1])
	}
	for _, sl := range lines {
		if sl.Kind == "h1" && sl.Text != "" {
			a.ParentTitle = sl.Text
			break
		}
	}
	if a.ParentTitle == "" {
		a.ParentTitle = inferSubjectTitle(raw)
	}
	if a.ParentTitle == "" {
		a.ParentTitle = extractTitle(raw)
	}
	if a.ParentTitle == "" {
		a.ParentTitle = fallbackTitle(raw)
	}

	h2, h3, seps, qa := 0, 0, 0, 0
	for _, sl := range lines {
		switch sl.Kind {
		case "h2":
			h2++
		case "h3":
			h3++
		case "sep":
			seps++
		case "qa":
			qa++
		}
	}
	switch {
	case seps >= 2 || (h2 >= 2 && h3 >= 1):
		a.DocType = "综合合集"
	case qa >= 4 && h2 == 0:
		a.DocType = "问答集"
	case seps >= 1 && h2 >= 1:
		a.DocType = "分段文档"
	case h2 >= 2:
		a.DocType = "综合合集"
	}

	if a.Subject == "" && a.ParentTitle != "" {
		if i := strings.Index(a.ParentTitle, "："); i > 0 {
			a.Subject = strings.TrimSpace(a.ParentTitle[:i])
		} else if i := strings.Index(a.ParentTitle, ":"); i > 0 {
			a.Subject = strings.TrimSpace(a.ParentTitle[:i])
		}
	}
	return a
}

// FormatSkeletonForLLM renders a compact skeleton for Stage-1/2 prompts.
func FormatSkeletonForLLM(sk DocumentSkeleton) string {
	var b strings.Builder
	b.WriteString("line_no|kind|text\n")
	for _, sl := range sk.Lines {
		if sl.Kind == "empty" || sl.Kind == "text" {
			continue
		}
		text := sl.Text
		if len([]rune(text)) > 80 {
			text = string([]rune(text)[:80]) + "…"
		}
		b.WriteString(fmt.Sprintf("%d|%s|%s\n", sl.LineNo, sl.Kind, text))
	}
	return b.String()
}
