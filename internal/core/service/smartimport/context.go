package smartimport

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/graydraft/core/model"
	"github.com/graydraft/core/service/workspacecontext"
)

func (s *Service) resolveContextProfile(ctx context.Context, req *model.SmartImportRequest) model.ContextProfile {
	kbType := partitionFromRequest(*req)
	stored := model.ContextProfile{KBType: kbType}
	if s != nil && s.docs != nil && kbType != "" {
		if p, err := s.docs.GetContextProfile(ctx, kbType); err == nil {
			stored = p
		}
	}
	return workspacecontext.MergeProfile(req.ContextProfile, stored, kbType)
}

func partitionFromRequest(req model.SmartImportRequest) string {
	if hint := strings.TrimSpace(req.HintKBType); hint != "" {
		return hint
	}
	return strings.TrimSpace(req.TenantID)
}

func attachWorkspaceContextExtensions(ext model.Extensions, profile model.ContextProfile) model.Extensions {
	if profile.IsEmpty() {
		return ext
	}
	if ext == nil {
		ext = model.Extensions{}
	}
	b, err := json.Marshal(profile)
	if err != nil {
		return ext
	}
	var m map[string]any
	if json.Unmarshal(b, &m) == nil {
		ext["workspace_context_profile"] = m
	}
	if formatted := workspacecontext.FormatForWorkspaceContext(profile); formatted != "" {
		ext["workspace_context_text"] = formatted
	}
	return ext
}
