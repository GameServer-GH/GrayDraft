package taxonomyevolve

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/graydraft/core/model"
)

type taxonomyRepo interface {
	GetWorkspaceTaxonomy(ctx context.Context, kbType string) (*model.WorkspaceTaxonomyState, error)
	SaveWorkspaceTaxonomy(ctx context.Context, st *model.WorkspaceTaxonomyState) error
	GetContextProfile(ctx context.Context, kbType string) (model.ContextProfile, error)
	ListRecentDocSummaries(ctx context.Context, kbType string, limit int) ([]model.DocSummarySnippet, error)
}

// Resolver returns effective per-workspace taxonomy (blueprint + YAML scaffold + DB overlay).
type Resolver struct {
	repo      taxonomyRepo
	onNeedBP  func(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string)
	mu        sync.RWMutex
	mem       map[string]resolvedEntry
}

type resolvedEntry struct {
	tax     *model.DomainTaxonomy
	version int
}

// NewResolver creates a taxonomy resolver.
func NewResolver(repo taxonomyRepo) *Resolver {
	return &Resolver{repo: repo, mem: make(map[string]resolvedEntry)}
}

func (r *Resolver) setBlueprintTrigger(fn func(context.Context, string, *model.ContextProfile, string)) {
	r.onNeedBP = fn
}

// Effective loads merged taxonomy for a kb_type partition.
func (r *Resolver) Effective(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) *model.DomainTaxonomy {
	if r == nil {
		return fallbackTaxonomy(profile, workspaceText)
	}
	kbType = strings.TrimSpace(kbType)
	if kbType == "" {
		return fallbackTaxonomy(profile, workspaceText)
	}
	var st *model.WorkspaceTaxonomyState
	if r.repo != nil {
		overlay, err := r.repo.GetWorkspaceTaxonomy(ctx, kbType)
		if err == nil {
			st = overlay
		}
	}
	if st != nil {
		if tax, ok := r.loadMem(kbType, st.Version); ok {
			return tax
		}
	}
	if st != nil && st.BlueprintReady() {
		tax := model.BuildFromBlueprint(st)
		if tax != nil {
			r.storeMem(kbType, st.Version, tax)
			return tax
		}
	}
	if r.onNeedBP != nil {
		r.onNeedBP(ctx, kbType, profile, workspaceText)
	}
	if domain := workspaceDomainHint(profile, workspaceText, st); domain != "" {
		tax := model.WorkspacePendingTaxonomy(domain, st)
		if tax != nil {
			ver := 0
			if st != nil {
				ver = st.Version
			}
			r.storeMem(kbType, ver, tax)
			return tax
		}
	}
	seed := model.DefaultCatalog().Get(model.UniversalDomainKey)
	tax := model.BuildEffectiveTaxonomy(seed, st, model.GlobalStyleScaffold())
	if st != nil && st.Version > 0 {
		r.storeMem(kbType, st.Version, tax)
	}
	return tax
}

func (r *Resolver) loadMem(kbType string, version int) (*model.DomainTaxonomy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ent, ok := r.mem[kbType]
	if !ok || ent.version != version || ent.tax == nil {
		return nil, false
	}
	return ent.tax, true
}

func (r *Resolver) storeMem(kbType string, version int, tax *model.DomainTaxonomy) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.mem[kbType] = resolvedEntry{tax: tax, version: version}
}

// Invalidate drops in-process cache for kb_type (after taxonomy version bump).
func (r *Resolver) Invalidate(kbType string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.mem, kbType)
}

// CacheKeySegment returns kb_type|domain|version for query4d cache keys.
func (r *Resolver) CacheKeySegment(ctx context.Context, kbType string, profile *model.ContextProfile, workspaceText string) string {
	tax := r.Effective(ctx, kbType, profile, workspaceText)
	if tax == nil {
		return "_"
	}
	ver := 0
	if r.repo != nil {
		if st, err := r.repo.GetWorkspaceTaxonomy(ctx, kbType); err == nil && st != nil {
			ver = st.Version
		}
	}
	return fmt.Sprintf("%s|v%d", tax.Domain, ver)
}

func fallbackTaxonomy(profile *model.ContextProfile, workspaceText string) *model.DomainTaxonomy {
	_ = profile
	_ = workspaceText
	seed := model.DefaultCatalog().Get(model.UniversalDomainKey)
	return model.BuildEffectiveTaxonomy(seed, nil, model.GlobalStyleScaffold())
}
