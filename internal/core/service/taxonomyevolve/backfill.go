package taxonomyevolve

import (
	"context"
	"log/slog"
	"time"
)

// EnqueueShadowBackfill schedules async 4D realignment after blueprint version bump.
func (s *Service) EnqueueShadowBackfill(ctx context.Context, kbType string, version int) {
	if s == nil || kbType == "" || version < 1 {
		return
	}
	go func() {
		bg, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()
		if err := s.RunShadowBackfill(bg, kbType, version); err != nil {
			slog.Warn("taxonomy shadow backfill", "kb_type", kbType, "version", version, "error", err)
		}
	}()
}

// RunShadowBackfill invokes Worker to align document semantic_tags_4d to workspace blueprint.
func (s *Service) RunShadowBackfill(ctx context.Context, kbType string, version int) error {
	if s == nil || kbType == "" {
		return nil
	}
	if s.repo != nil {
		if st, err := s.repo.GetWorkspaceTaxonomy(ctx, kbType); err == nil && st != nil {
			st.BlueprintBackfillVersion = version
			_ = s.repo.SaveWorkspaceTaxonomy(ctx, st)
		}
	}
	wc := NewWorkerClient(s.cfg)
	if wc == nil {
		slog.Info("taxonomy shadow backfill skipped (no worker url)", "kb_type", kbType, "version", version)
		return nil
	}
	limit := 80
	if s.cfg != nil && s.cfg.TaxonomyEvolve.BackfillBatchSize > 0 {
		limit = s.cfg.TaxonomyEvolve.BackfillBatchSize
	}
	resp, err := wc.ShadowBackfill(ctx, kbType, version, limit)
	if err != nil {
		return err
	}
	slog.Info("taxonomy shadow backfill done", "kb_type", kbType, "version", version, "updated", resp.Updated, "skipped", resp.Skipped)
	s.resolver.Invalidate(kbType)
	s.clearAlignCache()
	if resp.Updated > 0 {
		s.purgeCaches(ctx, kbType, "taxonomy_shadow_backfill")
	}
	return nil
}

func (s *Service) purgeCaches(ctx context.Context, kbType, reason string) {
	if kbType == "" {
		return
	}
	if s.cache != nil {
		n, err := s.cache.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			slog.Warn("taxonomy retrieval cache purge failed", "kb_type", kbType, "reason", reason, "error", err)
		} else {
			slog.Info("taxonomy retrieval cache purged", "kb_type", kbType, "reason", reason, "removed", n)
		}
	}
	if s.answer != nil {
		n, err := s.answer.RedisPurgeByKBType(ctx, kbType)
		if err != nil {
			slog.Warn("taxonomy answer cache purge failed", "kb_type", kbType, "reason", reason, "error", err)
		} else {
			slog.Info("taxonomy answer cache purged", "kb_type", kbType, "reason", reason, "removed", n)
		}
	}
}
