package smartimport

import "strings"

// mergeLLMRefinement merges LLM title hints; kb_type stays on workspace partition when locked.
func mergeLLMRefinement(rulesType, rulesTitle string, rulesConf float64, rulesReason string, content string, llm llmDraft, lockKBType bool) (string, string, float64, string) {
	kbType := rulesType
	conf := rulesConf
	reason := rulesReason
	if strings.TrimSpace(llm.Reason) != "" {
		reason += "; llm: " + strings.TrimSpace(llm.Reason)
	}
	if lockKBType {
		reason += " (workspace kb_type locked)"
	}

	title := rulesTitle
	if t := strings.TrimSpace(llm.Title); t != "" && titleAppearsInContent(t, content) {
		if isGenericFranchiseTitle(t, franchiseFromContent(content)) {
			if subj := inferSubjectTitle(content); subj != "" {
				t = subj
			}
		}
		title = t
	} else if t != "" && rulesTitle == "" {
		reason += "; llm title rejected (not in source)"
	}
	return kbType, title, conf, reason
}
