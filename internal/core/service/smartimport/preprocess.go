package smartimport

import (
	"context"
	"fmt"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/multimodal"
	"github.com/graydraft/core/service/taxonomyevolve"
	"github.com/graydraft/core/service/workspacecontext"
)

func importTitleHint(ext map[string]any) string {
	if ext == nil {
		return ""
	}
	v, _ := ext["import_title_hint"].(string)
	return strings.TrimSpace(v)
}

func (s *Service) preprocessRequest(ctx context.Context, req *model.SmartImportRequest) ([]model.SmartImportMultimodalExtraction, []multimodal.ExtractionResult, error) {
	if s == nil || s.multimodal == nil || len(req.Attachments) == 0 {
		return nil, nil, nil
	}
	if !s.multimodal.Enabled() {
		return nil, nil, fmt.Errorf("multimodal processing is not configured (set DASHSCOPE_API_KEY)")
	}
	profile := s.resolveContextProfile(ctx, req)
	ctxText := workspacecontext.FormatForMultimodal(profile)
	if strings.TrimSpace(req.WorkspaceContext) == "" {
		req.WorkspaceContext = workspacecontext.ResolveWorkspaceContextText(profile, "")
	}
	req.Extensions = attachWorkspaceContextExtensions(req.Extensions, profile)

	results, err := s.multimodal.ProcessAttachments(ctx, req.Attachments, req.WorkspacePromptOverride, ctxText)
	if err != nil {
		return nil, nil, err
	}
	workspaceID := strings.TrimSpace(req.HintKBType)
	if workspaceID == "" {
		workspaceID = strings.TrimSpace(req.TenantID)
	}
	req.Extensions = multimodal.ApplyToExtensions(req.Extensions, workspaceID, results)

	// Explicit signal that multimodal processing completed.
	// Used for detection in later pipeline stages without relying on
	// attachment filenames (which are often machine-generated noise)
	// or title inference success (which may fail for free-form VLM text).
	if req.Extensions == nil {
		req.Extensions = map[string]any{}
	}
	req.Extensions["multimodal_processed"] = true

	userNote := strings.TrimSpace(req.RawText)
	attachNames := attachmentNamesFromResults(results)

	docTitle := multimodal.SanitizeDocumentTitle(multimodal.InferConsolidatedTitle(userNote, results))
	req.RawText = multimodal.MergeExtractionsIntoRawText(userNote, results, docTitle)
	if docTitle == "" || multimodal.IsGenericConsolidatedTitle(docTitle) {
		if t := multimodal.InferConsolidatedTitle(userNote, results); t != "" {
			docTitle = multimodal.SanitizeDocumentTitle(t)
		}
	}
	if docTitle != "" && !multimodal.IsGenericConsolidatedTitle(docTitle) {
		if h1 := multimodal.ExtractMarkdownH1(req.RawText); h1 == "" || multimodal.IsGenericConsolidatedTitle(h1) {
			req.RawText = multimodal.MergeExtractionsIntoRawText(userNote, results, docTitle)
		}
	}
	if multimodal.IsConsolidatedFactTable(req.RawText) {
		if req.Extensions == nil {
			req.Extensions = map[string]any{}
		}
		req.Extensions["multimodal_consolidated"] = true
		req.Extensions["ingest_chunk_strategy"] = "fact_table"
		if docTitle != "" {
			req.Extensions["import_title_hint"] = docTitle
		}
		if note := multimodal.ExtractUserImportNote(userNote); note != "" {
			req.Extensions["user_import_note"] = note
		}
	} else if docTitle != "" {
		if req.Extensions == nil {
			req.Extensions = map[string]any{}
		}
		req.Extensions["import_title_hint"] = docTitle
	}
	// Fallback: when InferConsolidatedTitle returns empty (free-form VLM
	// text without legacy JSON rows or markdown tables), derive a title
	// hint from the VLM output's first meaningful non-metadata line.
	if _, hasHint := req.Extensions["import_title_hint"]; !hasHint {
		if len(results) > 0 && strings.TrimSpace(results[0].Text) != "" {
			hint := multimodal.FirstMeaningfulLine(results[0].Text, 48)
			if hint != "" && !multimodal.IsGenericConsolidatedTitle(hint) {
				if req.Extensions == nil {
					req.Extensions = map[string]any{}
				}
				req.Extensions["import_title_hint"] = hint
			}
		}
	}
	if strings.TrimSpace(req.RawText) == "" {
		return nil, results, fmt.Errorf("no text after multimodal extraction")
	}
	if len(attachNames) > 0 {
		if req.Extensions == nil {
			req.Extensions = map[string]any{}
		}
		names := make([]any, len(attachNames))
		for i, n := range attachNames {
			names[i] = n
		}
		req.Extensions["multimodal_attachment_names"] = names
	}
	if req.Filename == "" && len(req.Attachments) > 0 {
		req.Filename = req.Attachments[0].Filename
	}
	if req.MimeType == "" && len(req.Attachments) > 0 {
		req.MimeType = req.Attachments[0].MimeType
	}
	// Cross-modal consistency voting + entity whitelist check (plan 9.1 + 9.2)
	profile = s.resolveContextProfile(ctx, req)
	tax := taxonomyevolve.Effective(ctx, req.HintKBType, &profile, "")
	entityWL := multimodal.EntityWhitelistFromTaxonomy(tax.DetectKeywords)

	if strings.TrimSpace(req.RawText) != "" {
		for i := range results {
			hallucinations := multimodal.CrossModalVerify(req.RawText, results[i].Text)
			if len(hallucinations) > 0 {
				if req.Extensions == nil {
					req.Extensions = model.Extensions{}
				}
				req.Extensions["multimodal_uncertain"] = true
				req.Extensions["multimodal_hallucinations"] = hallucinations
			}
		}
	}

	// Entity whitelist check: flag extractions with unknown entities
	for i := range results {
		known, unknown, unknownList := multimodal.ValidateExtractionEntities(results[i].Text, entityWL)
		_ = known
		if unknown > 0 {
			if req.Extensions == nil {
				req.Extensions = model.Extensions{}
			}
			req.Extensions["multimodal_uncertain"] = true
			if existing, ok := req.Extensions["multimodal_hallucinations"].([]string); ok {
				req.Extensions["multimodal_hallucinations"] = append(existing, unknownList...)
			} else {
				req.Extensions["multimodal_hallucinations"] = unknownList
			}
		}
	}

	summary := make([]model.SmartImportMultimodalExtraction, 0, len(results))
	for _, r := range results {
		summary = append(summary, model.SmartImportMultimodalExtraction{
			Filename: r.Filename,
			Route:    r.Route,
			Cached:   r.Cached,
			MD5:      r.MD5,
		})
	}
	return summary, results, nil
}

func validateSmartImportInput(req model.SmartImportRequest) error {
	if strings.TrimSpace(req.RawText) != "" || len(req.Attachments) > 0 {
		return nil
	}
	return fmt.Errorf("raw_text or attachments required")
}
