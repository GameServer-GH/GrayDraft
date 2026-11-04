package agent

import (
	"regexp"
	"strings"
)

// These are auditable role observations, not semantic entailment verdicts.
// Missing/ambiguous bindings stay in review; only numeric exact contradictions reject.
type RoleObservation struct {
	PacketContentSHA256      string `json:"packet_content_sha256,omitempty"`
	RelatedRole              string `json:"related_role,omitempty"`
	Kind                     string `json:"kind"`
	Label                    string `json:"label"`
	Value                    string `json:"value,omitempty"`
	Scope                    string `json:"scope,omitempty"`
	Bound                    string `json:"bound,omitempty"`
	Polarity                 string `json:"polarity,omitempty"`
	ChunkID                  string `json:"chunk_id,omitempty"`
	PacketReferenceTruncated bool   `json:"packet_reference_truncated,omitempty"`
	ValueTruncated           bool   `json:"value_truncated,omitempty"`
}
type RoleReview struct {
	EvidenceOmitted int               `json:"evidence_omitted"`
	Candidate       RoleObservation   `json:"candidate"`
	Status          string            `json:"status"`
	Action          string            `json:"action"`
	Evidence        []RoleObservation `json:"evidence,omitempty"`
}
type ClaimReview struct {
	Disposition          string                   `json:"disposition"`
	RequestedCountScope  string                   `json:"requested_count_scope,omitempty"`
	RequestedCountObject string                   `json:"requested_count_object,omitempty"`
	Version              string                   `json:"version"`
	CandidateSHA256      string                   `json:"candidate_sha256"`
	Numeric              []NumericOperationReview `json:"numeric_operations,omitempty"`
	Roles                []RoleReview             `json:"role_checks,omitempty"`
	Truncated            bool                     `json:"review_truncated"`
	Coverage             string                   `json:"coverage"`
}

var reviewCountRE = regexp.MustCompile(`([0-9]+)\s*个\s*(工具|方法|参数|探测器|样本|模型|步骤)?`)
var reviewEnglishCountRE = regexp.MustCompile(`(?i)(?:exposes|provides|shows)\s+([0-9]+)\s+(tools|methods|parameters|detectors|samples|models|steps)`)
var reviewExplicitNameRE = regexp.MustCompile("(?i)(请求头(?:名)?(?:是|为)|(?:参数名|必填参数)(?:是|为)|header(?: name)?\\s*(?:is|:)|parameter name\\s*(?:is|:)|parameter\\s*:)\\s*[`\"' *]*([A-Za-z][A-Za-z0-9_-]*)")
var reviewReverseHeaderRE = regexp.MustCompile(`(?i)(?:uses|using)\s+(?:the\s+)?([A-Za-z][A-Za-z0-9_-]*)\s+header`)
var reviewMapKeyRE = regexp.MustCompile(`(?i)\b(headers|arguments)["']?\s*[=:]\s*\{\s*["']([A-Za-z][A-Za-z0-9_-]*)["']\s*:`)
var reviewStandaloneNameRE = regexp.MustCompile("^[`\"' ]*([A-Za-z][A-Za-z0-9_-]{0,63})[`\"' .]*$")
var reviewStatisticRE = regexp.MustCompile(`(?i)(信噪比|signal-to-noise(?: ratio)?|SNR|检测统计量|detection statistic|误警率|false.alarm(?: rate)?|显著性|significance)`)
var reviewCountQuestionRE = regexp.MustCompile(`(?:几|多少)(?:个)?(工具|方法|参数|探测器|样本|模型|步骤)`)

const reviewNumberAtom = `(?:[0-9]{1,3}(?:[ ,\x{00a0}\x{2009}\x{202f}][0-9]{3})+|[0-9]+(?:\.[0-9]+)?)(?:\s*(?:[eE][+-]?[0-9]+|[×x*]\s*10\s*\^?\s*[+-]?[0-9]+))?`

var reviewNumberRE = regexp.MustCompile(`\b` + reviewNumberAtom + `(?:\s*/\s*` + reviewNumberAtom + `)?\b`)
var reviewMapAssignmentRE = regexp.MustCompile(`^\s*[=:]\s*\{`)
var reviewNumberSeparators = strings.NewReplacer(" ", "", ",", "", "\t", "", "\u00a0", "", "\u2009", "", "\u202f", "")
var reviewClauseRE = regexp.MustCompile(`[\n。！？;；]`)

func hasReviewCue(s string, cues ...string) bool {
	s = strings.ToLower(s)
	for _, cue := range cues {
		if strings.Contains(s, strings.ToLower(cue)) {
			return true
		}
	}
	return false
}
func reviewRoleObservations(text, query, chunk string) ([]RoleObservation, bool) {
	text = normalizeReviewText(text)
	var rows []RoleObservation
	add := func(r RoleObservation) {
		if len(rows) < 64 {
			if len([]rune(r.Value)) > 64 {
				r.Value = truncateRunes(r.Value, 64)
				r.ValueTruncated = true
			}
			r.ChunkID = chunk
			rows = append(rows, r)
		}
	}
	for _, clause := range reviewClauseRE.Split(text, -1) {
		if clause == "" {
			continue
		}
		for _, m := range reviewCountRE.FindAllStringSubmatch(clause, -1) {
			label := m[2]
			if label == "" {
				label = "unspecified_object"
			}
			scope := reviewCountScope(clause)
			add(RoleObservation{Kind: "count", Label: label, Value: m[1], Scope: scope})
		}
		for _, m := range reviewEnglishCountRE.FindAllStringSubmatch(clause, -1) {
			labels := map[string]string{"tools": "工具", "methods": "方法", "parameters": "参数", "detectors": "探测器", "samples": "样本", "models": "模型", "steps": "步骤"}
			scope := "available_count"
			if hasReviewCue(m[0], "shows") {
				scope = "example_count"
			}
			add(RoleObservation{Kind: "count", Label: labels[strings.ToLower(m[2])], Value: m[1], Scope: scope})
		}
		names := map[string]bool{}
		addName := func(name, label, scope string) {
			key := label + "\x00" + name
			if names[key] || len(name) > 64 || hasReviewCue(name, "sk-", "pk-", "bearer") {
				return
			}
			names[key] = true
			add(RoleObservation{Kind: "identifier", Label: label, Value: name, Scope: scope})
		}
		for _, m := range reviewExplicitNameRE.FindAllStringSubmatchIndex(clause, -1) {
			// In "请求头为 headers={...}" the variable is a container, not the name.
			if reviewMapAssignmentRE.MatchString(clause[m[1]:]) {
				continue
			}
			label := "parameter_name"
			if hasReviewCue(clause[m[2]:m[3]], "请求头", "header") {
				label = "header_name"
			}
			addName(clause[m[4]:m[5]], label, "explicit_syntax_binding_unverified")
		}
		for _, m := range reviewReverseHeaderRE.FindAllStringSubmatch(clause, -1) {
			addName(m[1], "header_name", "explicit_syntax_binding_unverified")
		}
		for _, m := range reviewMapKeyRE.FindAllStringSubmatch(clause, -1) {
			label := "parameter_name"
			if strings.EqualFold(m[1], "headers") {
				label = "header_name"
			}
			addName(m[2], label, "map_key_binding_unverified")
		}
		if chunk == "" && hasReviewCue(query, "请求头", "参数名", "必填参数") {
			if m := reviewStandaloneNameRE.FindStringSubmatch(strings.TrimSpace(clause)); len(m) > 1 {
				label := "parameter_name"
				if hasReviewCue(query, "请求头") {
					label = "header_name"
				}
				addName(m[1], label, "question_target_binding_unverified")
			}
		}
		positions := reviewStatisticRE.FindAllStringIndex(clause, -1)
		for i, position := range positions {
			labelText := clause[position[0]:position[1]]
			end := len(clause)
			if i+1 < len(positions) {
				end = positions[i+1][0]
			}
			tail := string([]rune(clause[position[1]:end])[:min(80, len([]rune(clause[position[1]:end])))])
			label := reviewStatisticLabel(labelText)
			if i+1 < len(positions) && strings.TrimSpace(strings.Trim(tail, ":： \t")) == "" && hasReviewCue(tail, ":", "：") {
				next := positions[i+1]
				other := reviewStatisticLabel(clause[next[0]:next[1]])
				if other != label {
					add(RoleObservation{Kind: "statistic", Label: label, RelatedRole: other, Scope: "nested_other_role_before_value"})
				}
			}
			bound := "unspecified"
			if hasReviewCue(tail, "小于", "less than", "<") {
				bound = "lower_than"
			} else if hasReviewCue(tail, "大于", "greater than", ">") {
				bound = "greater_than"
			}
			for _, value := range reviewNumberRE.FindAllString(tail, 8) {
				add(RoleObservation{Kind: "statistic", Label: label, Value: reviewNumberSeparators.Replace(value), Bound: bound, Scope: "local_numeric_lexeme_not_entailment"})
			}
		}
		for _, role := range []struct {
			label string
			cues  []string
		}{{"assumption", []string{"假设", "如果", "限制", "assuming", "prior"}}, {"explanation", []string{"可用", "解释", "相符", "支持", "consistent with", "suggest"}}, {"estimate", []string{"估计", "推断", "推导", "estimated", "inferred"}}, {"certainty", []string{"已证实", "唯一确定", "确定了", "首次", "证明了", "confirmed", "first detection"}}} {
			if !hasReviewCue(clause, role.cues...) {
				continue
			}
			polarity := "unresolved"
			if hasReviewCue(clause, "没有", "未", "不能", "无法", "并非", "不是", "不代表", "不意味着", "not ") {
				polarity = "negation_present"
			}
			add(RoleObservation{Kind: "epistemic", Label: role.label, Polarity: polarity, Scope: "clause_cue_not_semantic_proof"})
		}
	}
	return rows, len(rows) >= 64
}

func reviewAnswerClaims(answer string, bundle EvidenceBundle) ClaimReview {
	numeric, truncated := reviewNumericOperations(answer)
	result := ClaimReview{Version: "claim-review-v2", CandidateSHA256: candidateReviewHash(answer), Numeric: numeric, Truncated: truncated, Coverage: "partial_rules; unmatched expressions and semantic bindings remain unverified"}
	if match := reviewCountQuestionRE.FindStringSubmatch(bundle.Query); len(match) > 1 {
		result.RequestedCountObject = match[1]
		result.RequestedCountScope = reviewCountScope(bundle.Query)
	}
	candidates, capped := reviewRoleObservations(answer, bundle.Query, "")
	result.Truncated = result.Truncated || capped
	if len(candidates) > 24 {
		candidates = candidates[:24]
		result.Truncated = true
	}
	var source []RoleObservation
	// Only the actual packet contents supplied by the caller are admitted here.
	for _, hit := range bundle.Hits {
		rows, limited := reviewRoleObservations(hit.Content, "", hit.ChunkID)
		for i := range rows {
			rows[i].PacketReferenceTruncated = hit.Truncated
			rows[i].PacketContentSHA256 = candidateReviewHash(hit.Content)
		}
		source = append(source, rows...)
		result.Truncated = result.Truncated || limited
	}
	for _, candidate := range candidates {
		check := RoleReview{Candidate: candidate, Status: "needs_review", Action: "review"}
		sameRoleValue, otherRoleValue := false, false
		seen := map[RoleObservation]bool{}
		for priority := 2; priority >= 0; priority-- {
			for _, e := range source {
				if e.Kind != candidate.Kind {
					continue
				}
				score := 0
				if e.Label == candidate.Label {
					score = 1
				}
				if candidate.Value != "" && e.Value == candidate.Value {
					score = 2
				}
				if score != priority || seen[e] {
					continue
				}
				seen[e] = true
				if candidate.Value != "" && e.Value == candidate.Value {
					if e.Label == candidate.Label && (candidate.Kind != "count" || (e.Scope == candidate.Scope && e.Scope != "unbound_count" && e.Scope != "ambiguous_count_scope")) {
						sameRoleValue = true
					} else {
						otherRoleValue = true
					}
				}
				if len(check.Evidence) < 8 {
					check.Evidence = append(check.Evidence, e)
				} else {
					check.EvidenceOmitted++
					result.Truncated = true
				}
			}
		}
		if sameRoleValue {
			check.Status = "role_value_observed_binding_unverified"
		}
		if candidate.Kind == "identifier" && !sameRoleValue {
			check.Status = "identifier_not_observed_in_packet"
		}
		if candidate.Kind == "identifier" && otherRoleValue && !sameRoleValue {
			check.Status = "identifier_seen_under_other_role"
		}
		if candidate.Kind == "count" && !sameRoleValue && len(check.Evidence) > 0 {
			check.Status = "count_value_not_observed_for_role"
		}
		if candidate.Kind == "statistic" && otherRoleValue && !sameRoleValue {
			check.Status = "value_seen_under_other_role"
		}
		if candidate.Kind == "epistemic" && candidate.Label == "certainty" && candidate.Polarity != "negation_present" {
			sourceCertainty := false
			for _, e := range source {
				if e.Kind == "epistemic" && e.Label == "certainty" && e.Polarity != "negation_present" {
					sourceCertainty = true
				}
			}
			if !sourceCertainty {
				check.Status = "certainty_strength_unverified"
			}
		}
		if len(check.Evidence) == 0 {
			check.Status = "no_role_evidence_in_packet"
		}
		if candidate.Kind == "count" && candidate.Label == result.RequestedCountObject && result.RequestedCountScope != "" && result.RequestedCountScope != "unbound_count" && result.RequestedCountScope != "ambiguous_count_scope" && candidate.Scope != result.RequestedCountScope {
			check.Status = "count_scope_requires_review"
		}
		if candidate.Kind == "statistic" && candidate.Scope == "nested_other_role_before_value" {
			check.Status = "statistic_role_binding_ambiguous"
		}
		// Even a literal value match does not bind it to the same entity, conditions,
		// counting set or certainty level. Never upgrade it to semantic verification.
		result.Roles = append(result.Roles, check)
	}
	result.Disposition = "not_covered"
	if len(result.Numeric) > 0 {
		result.Disposition = "checked_subset_consistent"
	}
	if len(result.Roles) > 0 || result.Truncated {
		result.Disposition = "review_required"
	}
	for _, operation := range result.Numeric {
		if operation.Action == "review" && result.Disposition != "reject" {
			result.Disposition = "review_required"
		}
		if operation.Action == "reject" {
			result.Disposition = "reject"
		}
	}
	return result
}

func reviewCountScope(text string) string {
	examples := hasReviewCue(text, "典型", "示例", "example")
	available := hasReviewCue(text, "暴露", "提供", "available", "exposes")
	switch {
	case examples && available:
		return "ambiguous_count_scope"
	case examples:
		return "example_count"
	case available:
		return "available_count"
	default:
		return "unbound_count"
	}
}
func reviewStatisticLabel(text string) string {
	switch {
	case hasReviewCue(text, "信噪比", "signal-to-noise", "snr"):
		return "snr"
	case hasReviewCue(text, "检测统计量", "detection statistic"):
		return "detection_statistic"
	case hasReviewCue(text, "误警率", "false"):
		return "false_alarm"
	default:
		return "significance"
	}
}
