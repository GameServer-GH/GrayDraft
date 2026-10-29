package postgres

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/graydraft/core/model"
)

// GetContextProfile loads kb_types.context_profile.
func (r *DocumentRepo) GetContextProfile(ctx context.Context, kbType string) (model.ContextProfile, error) {
	if kbType == "" {
		return model.ContextProfile{}, fmt.Errorf("kb_type required")
	}
	var raw []byte
	err := r.client.pool.QueryRow(ctx, `
		SELECT COALESCE(context_profile::text, 'null')
		FROM kb_types WHERE name = $1
	`, kbType).Scan(&raw)
	if err != nil {
		return model.ContextProfile{}, err
	}
	if string(raw) == "null" || len(raw) == 0 {
		return model.ContextProfile{KBType: kbType}, nil
	}
	var p model.ContextProfile
	if err := json.Unmarshal(raw, &p); err != nil {
		return model.ContextProfile{}, err
	}
	if p.KBType == "" {
		p.KBType = kbType
	}
	return p, nil
}

// SetContextProfile upserts context_profile for a kb_type (ensures row exists).
func (r *DocumentRepo) SetContextProfile(ctx context.Context, kbType string, profile model.ContextProfile) error {
	if kbType == "" {
		return fmt.Errorf("kb_type required")
	}
	profile.KBType = kbType
	b, err := json.Marshal(profile)
	if err != nil {
		return err
	}
	_, err = r.client.pool.Exec(ctx, `
		INSERT INTO kb_types (name, description, context_profile)
		VALUES ($1, $2, $3::jsonb)
		ON CONFLICT (name) DO UPDATE SET
			context_profile = EXCLUDED.context_profile,
			description = CASE
				WHEN kb_types.description IS NULL OR kb_types.description = '' THEN EXCLUDED.description
				ELSE kb_types.description
			END
	`, kbType, profile.Description, string(b))
	return err
}
