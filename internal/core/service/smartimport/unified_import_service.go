package smartimport

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/observability/agentlog"
	"github.com/graydraft/core/service/workspacecontext"
)

func (s *Service) runUnifiedImportLLM(ctx context.Context, req *model.SmartImportRequest, sess *agentlog.Session, forceSingle bool) {
	if s == nil || req == nil || !req.UseLLM {
		return
	}
	if req.Extensions == nil {
		req.Extensions = model.Extensions{}
	}
	if unifiedImportCacheHit(req.Extensions) {
		return
	}
	fn := s.llmFn(ctx, *req, sess)
	if fn == nil {
		return
	}

	content := normalizeRaw(req.RawText)
	if len([]rune(content)) < 10 {
		return
	}

	sourceHash := sha256.Sum256([]byte(content))
	req.Extensions["source_revision_sha256"] = hex.EncodeToString(sourceHash[:])
	req.Extensions["source_line_count"] = len(strings.Split(content, "\n"))
	sk := ScanSkeleton(content)

	rulesKB, rulesConf, rulesReason := ResolveKBType(req.HintKBType)
	if rulesKB == "" {
		return
	}
	rulesTitle := importTitleHint(req.Extensions)
	if rulesTitle == "" {
		rulesTitle = extractTitle(content)
	}
	if rulesTitle == "" {
		rulesTitle = preferDocumentTitle(content, sk.Analysis)
	}

	profile := s.resolveContextProfile(ctx, req)
	bg := workspacecontext.FormatForMultimodal(profile)
	userNote := ""
	if v, ok := req.Extensions["user_import_note"].(string); ok {
		userNote = strings.TrimSpace(v)
	}
	if userNote == "" {
		userNote = strings.TrimSpace(req.RawText)
		if len([]rune(userNote)) > 200 {
			userNote = string([]rune(userNote)[:200])
		}
	}

	params := UnifiedImportParams{
		Content:             content,
		UserNote:            userNote,
		AttachmentNames:     attachmentNamesFromExtensions(req.Extensions),
		WorkspaceBackground: bg,
		Skeleton:            sk,
		ForceSingle:         forceSingle,
		HintKBType:          req.HintKBType,
		RulesKBType:         rulesKB,
		RulesTitle:          rulesTitle,
		RulesConfidence:     rulesConf,
		RulesReason:         rulesReason,
		MixedContent:        contentLooksMixed(sk),
	}
	u, err := RunUnifiedImportLLM(params, fn)
	if err != nil {
		req.Extensions[extImportLLMUnifiedFailCnt] = 1
		u, err = RunUnifiedImportLLM(params, fn)
	}
	if err != nil {
		req.Extensions[extImportLLMUnifiedOK] = false
		req.Extensions[extImportLLMUnifiedFailCnt] = 2
		if sess != nil {
			sess.Step("unified_import_llm", map[string]any{"ok": false, "error": err.Error(), "retried": true})
		}
		return
	}

	ApplySemanticPivot(req, u.Pivot)
	req.Extensions = attachMappingDelta(req.Extensions, u.MappingDelta)
	applyUnifiedTitleToRaw(req, u.Title)
	storeUnifiedImportCache(req.Extensions, req.RawText, u)
	if sess != nil {
		step := map[string]any{"ok": true, "title": u.Title, "kb_type": u.KBType}
		if u.SplitPlan != nil {
			step["strategy"] = u.SplitPlan.Strategy
		}
		sess.Step("unified_import_llm", step)
	}
}

func attachmentNamesFromExtensions(ext model.Extensions) []string {
	if ext == nil {
		return nil
	}
	raw, ok := ext["multimodal_attachment_names"].([]any)
	if !ok {
		return nil
	}
	var names []string
	for _, v := range raw {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			names = append(names, strings.TrimSpace(s))
		}
	}
	return names
}

func unifiedForceSingle(req model.SmartImportRequest) bool {
	return isMultimodalConsolidated(req.Extensions) || !req.Split || strings.TrimSpace(req.TargetDocID) != ""
}
