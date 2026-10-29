package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/graydraft/core/model"
)

// GetWorkspaceTaxonomy loads workspace_4d_taxonomy for kb_type (nil state if missing).
func (r *DocumentRepo) GetWorkspaceTaxonomy(ctx context.Context, kbType string) (*model.WorkspaceTaxonomyState, error) {
	if kbType == "" {
		return nil, fmt.Errorf("kb_type required")
	}
	var domain string
	var version int
	var scenes, styles, candidates, stats []byte
	var updatedAt time.Time
	err := r.client.pool.QueryRow(ctx, `
		SELECT domain, version,
			COALESCE(scenes::text, '[]'),
			COALESCE(styles::text, '[]'),
			COALESCE(candidates::text, '[]'),
			COALESCE(stats::text, '{}'),
			updated_at
		FROM workspace_4d_taxonomy WHERE kb_type = $1
	`, kbType).Scan(&domain, &version, &scenes, &styles, &candidates, &stats, &updatedAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	st := &model.WorkspaceTaxonomyState{KBType: kbType, Domain: domain, Version: version}
	_ = json.Unmarshal(scenes, &st.Scenes)
	_ = json.Unmarshal(styles, &st.Styles)
	_ = json.Unmarshal(candidates, &st.Candidates)
	unmarshalWorkspaceStats(stats, st)
	return st, nil
}

// SaveWorkspaceTaxonomy upserts workspace taxonomy state.
func (r *DocumentRepo) SaveWorkspaceTaxonomy(ctx context.Context, st *model.WorkspaceTaxonomyState) error {
	if st == nil || st.KBType == "" {
		return fmt.Errorf("kb_type required")
	}
	scenes, _ := json.Marshal(st.Scenes)
	styles, _ := json.Marshal(st.Styles)
	candidates, _ := json.Marshal(st.Candidates)
	stats := marshalWorkspaceStats(st)
	_, err := r.client.pool.Exec(ctx,
		`INSERT INTO kb_types (name, description) VALUES ($1, '') ON CONFLICT (name) DO NOTHING`,
		st.KBType)
	if err != nil {
		return err
	}
	_, err = r.client.pool.Exec(ctx,
		`INSERT INTO workspace_4d_taxonomy (kb_type, domain, version, scenes, styles, candidates, stats, updated_at)
		VALUES ($1, $2, $3, $4::jsonb, $5::jsonb, $6::jsonb, $7::jsonb, NOW())
		ON CONFLICT (kb_type) DO UPDATE SET
			domain = EXCLUDED.domain,
			version = EXCLUDED.version,
			scenes = EXCLUDED.scenes,
			styles = EXCLUDED.styles,
			candidates = EXCLUDED.candidates,
			stats = EXCLUDED.stats,
			updated_at = NOW()`,
		st.KBType, st.Domain, st.Version, string(scenes), string(styles), string(candidates), string(stats))
	return err
}
