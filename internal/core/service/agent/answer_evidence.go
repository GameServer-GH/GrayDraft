package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// Prepare once, then reuse the exact bytes for generation and validation.
// Retrieval observations remain untouched for planning and subsequent steps.
func prepareAnswerEvidence(prompt []byte, observations []Observation) ([]Observation, string) {
	groups := overviewGroups(observations)
	overview := isContentOverview(CurrentUserQuery(string(prompt))) && len(groups) > 1
	wanted := map[string]bool{}
	for _, g := range groups {
		wanted[g] = true
	}
	var prepared []Observation
	for _, o := range observations {
		if o.Err != nil || len(o.Output) == 0 {
			continue
		}
		if strings.HasPrefix(o.Name, "knowledge.") {
			if o.Name == "knowledge.catalog" {
				// Catalog discovery metadata is not body evidence. Keep only
				// source identities used to choose overview groups.
				var catalog struct {
					Total     int `json:"total"`
					Documents []struct {
						Title string `json:"title"`
						DocID string `json:"doc_id,omitempty"`
					} `json:"documents"`
				}
				if json.Unmarshal(o.Output, &catalog) != nil {
					continue
				}
				o.Output, _ = json.Marshal(catalog)
				o.AnswerPrepared = true
				prepared = append(prepared, o)
				continue
			}
			if overview && o.Name != "knowledge.read" {
				continue
			}
			var payload knowledgeSearchPayload
			if json.Unmarshal(o.Output, &payload) != nil {
				continue
			}
			if overview {
				hits := make([]EvidenceHit, 0, len(payload.Hits))
				for _, h := range payload.Hits {
					if wanted[catalogGroupTitle(h.Title)] {
						hits = append(hits, h)
					}
				}
				payload.Hits = hits
				o.Output, _ = json.Marshal(payload)
			}
			payload.RetrievalQuery = payload.Query
			payload.Query = CurrentUserQuery(string(prompt))
			o.Output, _ = json.Marshal(payload)
			o.Output = []byte(answerObservation(o, 12000))
			o.AnswerPrepared = true
		} else {
			// Not admissible as factual evidence; use the same bounded context in composition.
			text := string(o.Output)
			var decoded string
			if json.Unmarshal(o.Output, &decoded) == nil {
				text = decoded
			}
			o.Output = contextObservationJSON(truncateRunes(text, 2400))
			o.AnswerPrepared = true
		}
		prepared = append(prepared, o)
	}
	return prepared, answerEvidenceDigest(prepared)
}

func answerEvidenceAudit(obs []Observation, hash string) string {
	type reference struct {
		ChunkID        string `json:"chunk_id"`
		DocID          string `json:"doc_id,omitempty"`
		ContentSHA256  string `json:"content_sha256"`
		OriginalSHA256 string `json:"original_content_sha256,omitempty"`
		SourceRevision string `json:"source_revision_sha256,omitempty"`
		Start          int    `json:"start_rune"`
		End            int    `json:"end_rune"`
		Truncated      bool   `json:"truncated"`
	}
	var refs []reference
	for _, h := range CollectEvidence(obs).Hits {
		sum := sha256.Sum256([]byte(h.Content))
		refs = append(refs, reference{h.ChunkID, h.DocID, hex.EncodeToString(sum[:]), h.OriginalContentSHA256, h.SourceRevision, h.ContentStartRune, h.ContentEndRune, h.Truncated})
	}
	raw, _ := json.Marshal(struct {
		Hash       string      `json:"evidence_packet_sha256"`
		References []reference `json:"references"`
	}{hash, refs})
	return string(raw)
}

func answerEvidenceDigest(obs []Observation) string {
	raw, err := json.Marshal(obs)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}
