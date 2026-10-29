package taxonomyevolve

import (
	"context"
	"sync/atomic"

	"github.com/graydraft/core/model"
)

var defaultSvc atomic.Pointer[Service]

// SetDefault registers the process-wide taxonomy evolution service.
func SetDefault(s *Service) {
	defaultSvc.Store(s)
}

// Effective returns workspace-aware taxonomy (falls back to static catalog).
func Effective(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) *model.DomainTaxonomy {
	if s := defaultSvc.Load(); s != nil && s.resolver != nil {
		return s.resolver.Effective(ctx, kbType, profile, workspaceText)
	}
	return model.DefaultCatalog().ResolveTaxonomy(profile, workspaceText, "")
}

// CacheKeySegment for query4d / retrieval cache busting.
func CacheKeySegment(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) string {
	if s := defaultSvc.Load(); s != nil && s.resolver != nil {
		return s.resolver.CacheKeySegment(ctx, kbType, profile, workspaceText)
	}
	tax := model.DefaultCatalog().ResolveTaxonomy(profile, workspaceText, "")
	if tax == nil {
		return "_"
	}
	return tax.Domain
}

// EnqueueMappingDelta runs Loop A when service is configured.
func EnqueueMappingDelta(ctx context.Context, kbType string, delta model.MappingDelta) {
	if s := defaultSvc.Load(); s != nil {
		s.EnqueueRecordDelta(ctx, kbType, delta)
	}
}

// EnqueueBlueprint schedules workspace taxonomy blueprint generation.
func EnqueueBlueprint(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) {
	if s := defaultSvc.Load(); s != nil {
		s.enqueueBlueprint(ctx, kbType, profile, workspaceText)
	}
}
