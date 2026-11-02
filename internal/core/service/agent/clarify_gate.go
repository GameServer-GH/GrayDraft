package agent

import (
	"encoding/json"
	"regexp"
	"strings"
)

var reCatalogPart = regexp.MustCompile(`[（(](\d+)/\d+[）)]`)

const (
	refuseNoEvidenceTemplate = "本工作区资料中没有找到相关依据，无法回答该问题。"
	clarifyAskTemplate       = "本工作区资料未写明您要查的对象。请补充具体概念、参数名或文档范围后再试。"
)

// hasUsefulEvidence reports whether observations already contain retrievable chunk text
// that should block an empty clarify.
func hasUsefulEvidence(obs []Observation) bool {
	if catalogHasDocs(obs) {
		return true
	}
	b := CollectEvidence(obs)
	if len(b.Hits) == 0 {
		return false
	}
	grade := strings.ToUpper(strings.TrimSpace(b.RetrievalGrade))
	if grade == "D" && b.Total == 0 {
		return false
	}
	for _, h := range b.Hits {
		if strings.TrimSpace(h.Content) != "" || strings.TrimSpace(h.ChunkID) != "" {
			return true
		}
	}
	return false
}

// catalogOverviewAnswer lists distinct document titles for a theme question.
// Catalog rows are often slices of one paper; the part marker is not a separate work.
func catalogOverviewAnswer(obs []Observation) string {
	seen := map[string]bool{}
	var titles []string
	for _, o := range obs {
		if o.Name != "knowledge.catalog" || len(o.Output) == 0 || o.Err != nil {
			continue
		}
		var payload struct {
			Documents []struct {
				Title       string `json:"title"`
				ParentTitle string `json:"parent_title"`
				Summary     string `json:"summary"`
			} `json:"documents"`
		}
		if json.Unmarshal(o.Output, &payload) != nil {
			continue
		}
		for _, d := range payload.Documents {
			name := strings.TrimSpace(d.ParentTitle)
			if name == "" {
				name = catalogGroupTitle(d.Title)
			}
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			line := name
			if s := strings.TrimSpace(d.Summary); s != "" {
				line += "：" + s
			}
			titles = append(titles, line)
		}
	}
	if len(titles) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("工作区里的材料按标题分为：\n")
	for _, t := range titles {
		b.WriteString("- ")
		b.WriteString(t)
		b.WriteString("\n")
	}
	return strings.TrimSpace(b.String())
}

func catalogGroupTitle(title string) string {
	t := strings.TrimSpace(title)
	t = strings.TrimPrefix(t, "论文 ")
	t = strings.TrimSpace(reCatalogPart.ReplaceAllString(t, ""))
	return t
}

func hasChunkEvidence(obs []Observation) bool {
	b := CollectEvidence(obs)
	for _, h := range b.Hits {
		if strings.TrimSpace(h.Content) != "" || strings.TrimSpace(h.ChunkID) != "" {
			return true
		}
	}
	return false
}

// isCatalogListQuestion is a directory ask (which documents exist), not a request
// to explain what each document says.
func isContentOverview(q string) bool {
	if isCatalogListQuestion(q) {
		return false
	}
	return strings.Contains(q, "讲什么") || strings.Contains(q, "各自") || strings.Contains(q, "主题")
}

// openingGroups lists distinct works in catalog order, for reading each opening slice.
func openingGroups(obs []Observation, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	var names []string
	seen := map[string]bool{}
	for _, o := range obs {
		if o.Name != "knowledge.catalog" || len(o.Output) == 0 || o.Err != nil {
			continue
		}
		var payload struct {
			Documents []struct {
				Title string `json:"title"`
			} `json:"documents"`
		}
		if json.Unmarshal(o.Output, &payload) != nil {
			continue
		}
		for _, d := range payload.Documents {
			name := catalogGroupTitle(d.Title)
			if name == "" || seen[name] {
				continue
			}
			seen[name] = true
			names = append(names, name)
			if len(names) >= limit {
				return names
			}
		}
	}
	return names
}

// openingDocIDs picks one slice per work, preferring the first part, for a content overview.
func openingDocIDs(obs []Observation, limit int) []string {
	if limit <= 0 {
		limit = 3
	}
	type cand struct {
		id   string
		part int
		ok   bool
	}
	best := map[string]cand{}
	for _, o := range obs {
		if o.Name != "knowledge.catalog" || len(o.Output) == 0 || o.Err != nil {
			continue
		}
		var payload struct {
			Documents []struct {
				DocID string `json:"doc_id"`
				Title string `json:"title"`
			} `json:"documents"`
		}
		if json.Unmarshal(o.Output, &payload) != nil {
			continue
		}
		for _, d := range payload.Documents {
			name := catalogGroupTitle(d.Title)
			if name == "" || d.DocID == "" {
				continue
			}
			part, ok := slicePartNum(d.Title)
			cur, seen := best[name]
			if !seen || (ok && (!cur.ok || part < cur.part)) {
				best[name] = cand{id: d.DocID, part: part, ok: ok}
			}
		}
	}
	ids := make([]string, 0, len(best))
	for _, c := range best {
		ids = append(ids, c.id)
		if len(ids) >= limit {
			break
		}
	}
	return ids
}

func slicePartNum(title string) (int, bool) {
	m := reCatalogPart.FindStringSubmatch(title)
	if len(m) < 2 {
		return 0, false
	}
	n := 0
	for _, r := range m[1] {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int(r-'0')
	}
	return n, true
}

func isCatalogListQuestion(q string) bool {
	q = strings.TrimSpace(q)
	if q == "" {
		return false
	}
	for _, s := range []string{"有哪些", "有什么文档", "哪些论文", "哪些文档", "列出", "目录"} {
		if strings.Contains(q, s) {
			return true
		}
	}
	return false
}

func catalogHasDocs(obs []Observation) bool {
	for _, o := range obs {
		if o.Name != "knowledge.catalog" || len(o.Output) == 0 || o.Err != nil {
			continue
		}
		var payload struct {
			Total int `json:"total"`
		}
		if json.Unmarshal(o.Output, &payload) == nil && payload.Total > 0 {
			return true
		}
	}
	return false
}

func evidenceIsEmpty(obs []Observation) bool {
	if catalogHasDocs(obs) {
		return false
	}
	b := CollectEvidence(obs)
	if len(b.Hits) > 0 {
		for _, h := range b.Hits {
			if strings.TrimSpace(h.Content) != "" {
				return false
			}
		}
	}
	grade := strings.ToUpper(strings.TrimSpace(b.RetrievalGrade))
	return len(b.Hits) == 0 || grade == "D" || b.Total == 0 && len(b.texts()) == 0
}

// resolveClarify applies P0 clarify gates:
// - useful evidence / catalog docs → force final (streamer writes from observations)
// - empty / grade D after tools ran → explicit refuse final
// - no tools yet / truly missing object → keep clarify with fixed copy
func resolveClarify(d Decision, obs []Observation) Decision {
	if d.Decision != DecisionClarify {
		return d
	}
	if hasUsefulEvidence(obs) {
		d.Decision = DecisionFinal
		d.Answer = ""
		if note := strings.TrimSpace(d.ThoughtNote); note == "" {
			d.ThoughtNote = "已有证据，整理回答"
		}
		return d
	}
	searched := false
	for _, o := range obs {
		if strings.HasPrefix(o.Name, "knowledge.") {
			searched = true
			break
		}
	}
	if searched && evidenceIsEmpty(obs) {
		d.Decision = DecisionFinal
		d.Answer = refuseNoEvidenceTemplate
		if note := strings.TrimSpace(d.ThoughtNote); note == "" {
			d.ThoughtNote = "资料中没有依据"
		}
		return d
	}
	if strings.TrimSpace(d.Answer) == "" || strings.Contains(d.Answer, "补充") {
		d.Answer = clarifyAskTemplate
	}
	return d
}
