package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// HealJob is a RAGOps auto-heal task (O-19).
type HealJob struct {
	ID          string          `json:"id"`
	KBType      string          `json:"kb_type"`
	DocID       string          `json:"doc_id"`
	ChunkID     string          `json:"chunk_id,omitempty"`
	Reason      string          `json:"reason"`
	Action      string          `json:"action"`
	Status      string          `json:"status"`
	StrikeCount int             `json:"strike_count"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// RecordChunkStrike upserts a daily strike for a chunk (O-19).
func (r *ChunkRepo) RecordChunkStrike(
	ctx context.Context,
	kbType, chunkID, source string,
	confidence *float64,
) error {
	cid, err := uuid.Parse(chunkID)
	if err != nil {
		return err
	}
	_, err = r.client.pool.Exec(ctx, `
		INSERT INTO ragops_chunk_strikes (chunk_id, kb_type, strike_date, source, confidence)
		VALUES ($1, $2, CURRENT_DATE, $3, $4)
		ON CONFLICT (chunk_id, kb_type, strike_date, source) DO UPDATE
		SET confidence = COALESCE(EXCLUDED.confidence, ragops_chunk_strikes.confidence)
	`, cid, kbType, source, confidence)
	return err
}

// CountRecentStrikes returns strike days in the last N days for a chunk.
func (r *ChunkRepo) CountRecentStrikes(ctx context.Context, kbType, chunkID string, days int) (int, error) {
	if days <= 0 {
		days = 3
	}
	cid, err := uuid.Parse(chunkID)
	if err != nil {
		return 0, err
	}
	var n int
	err = r.client.pool.QueryRow(ctx, `
		SELECT COUNT(DISTINCT strike_date)::int
		FROM ragops_chunk_strikes
		WHERE chunk_id = $1 AND kb_type = $2
		  AND strike_date >= CURRENT_DATE - ($3 || ' days')::interval
	`, cid, kbType, fmt.Sprint(days)).Scan(&n)
	return n, err
}

// CreateHealJob inserts a pending heal job if none active for doc (O-19).
func (r *ChunkRepo) CreateHealJob(
	ctx context.Context,
	kbType, docID, chunkID, reason, action string,
	strikeCount int,
) (*HealJob, error) {
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		return nil, err
	}
	var chunkUUID *uuid.UUID
	if chunkID != "" {
		parsed, err := uuid.Parse(chunkID)
		if err != nil {
			return nil, err
		}
		chunkUUID = &parsed
	}
	if action == "" {
		action = "rechunk"
	}

	var existing string
	err = r.client.pool.QueryRow(ctx, `
		SELECT id::text FROM ragops_heal_jobs
		WHERE doc_id = $1 AND kb_type = $2 AND status IN ('pending', 'running')
		LIMIT 1
	`, docUUID, kbType).Scan(&existing)
	if err == nil {
		return r.GetHealJob(ctx, existing)
	}
	if err != pgx.ErrNoRows {
		return nil, err
	}

	var job HealJob
	err = r.client.pool.QueryRow(ctx, `
		INSERT INTO ragops_heal_jobs (kb_type, doc_id, chunk_id, reason, action, strike_count)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id::text, kb_type, doc_id::text, COALESCE(chunk_id::text, ''), reason, action, status,
		          strike_count, metadata, created_at, updated_at
	`, kbType, docUUID, chunkUUID, reason, action, strikeCount).Scan(
		&job.ID, &job.KBType, &job.DocID, &job.ChunkID, &job.Reason, &job.Action, &job.Status,
		&job.StrikeCount, &job.Metadata, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// GetHealJob loads one heal job by id.
func (r *ChunkRepo) GetHealJob(ctx context.Context, id string) (*HealJob, error) {
	var job HealJob
	err := r.client.pool.QueryRow(ctx, `
		SELECT id::text, kb_type, doc_id::text, COALESCE(chunk_id::text, ''), reason, action, status,
		       strike_count, metadata, created_at, updated_at
		FROM ragops_heal_jobs WHERE id = $1::uuid
	`, id).Scan(
		&job.ID, &job.KBType, &job.DocID, &job.ChunkID, &job.Reason, &job.Action, &job.Status,
		&job.StrikeCount, &job.Metadata, &job.CreatedAt, &job.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

// ListHealJobs returns heal jobs filtered by status.
func (r *ChunkRepo) ListHealJobs(ctx context.Context, status string, limit int) ([]HealJob, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	q := `
		SELECT id::text, kb_type, doc_id::text, COALESCE(chunk_id::text, ''), reason, action, status,
		       strike_count, metadata, created_at, updated_at
		FROM ragops_heal_jobs
	`
	var args []any
	if status != "" {
		q += ` WHERE status = $1 ORDER BY created_at DESC LIMIT $2`
		args = []any{status, limit}
	} else {
		q += ` ORDER BY created_at DESC LIMIT $1`
		args = []any{limit}
	}
	rows, err := r.client.pool.Query(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []HealJob
	for rows.Next() {
		var job HealJob
		if err := rows.Scan(
			&job.ID, &job.KBType, &job.DocID, &job.ChunkID, &job.Reason, &job.Action, &job.Status,
			&job.StrikeCount, &job.Metadata, &job.CreatedAt, &job.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, job)
	}
	return out, rows.Err()
}

// UpdateHealJobStatus sets job status and optional metadata patch.
func (r *ChunkRepo) UpdateHealJobStatus(ctx context.Context, id, status string, meta json.RawMessage) error {
	_, err := r.client.pool.Exec(ctx, `
		UPDATE ragops_heal_jobs
		SET status = $2,
		    metadata = CASE WHEN $3::jsonb IS NULL THEN metadata ELSE metadata || $3::jsonb END,
		    updated_at = now()
		WHERE id = $1::uuid
	`, id, status, meta)
	return err
}

// LowQualityChunk is a candidate for heal scan.
type LowQualityChunk struct {
	ChunkID      string  `json:"chunk_id"`
	DocID        string  `json:"doc_id"`
	KBType       string  `json:"kb_type"`
	QualityScore float64 `json:"quality_score"`
	StrikeDays   int     `json:"strike_days"`
}

// ListLowQualityChunks finds chunks below threshold with recent strikes (O-19).
func (r *ChunkRepo) ListLowQualityChunks(
	ctx context.Context,
	qualityBelow float64,
	strikeDays, minStrikes int,
	limit int,
) ([]LowQualityChunk, error) {
	if qualityBelow <= 0 {
		qualityBelow = 0.45
	}
	if strikeDays <= 0 {
		strikeDays = 3
	}
	if minStrikes <= 0 {
		minStrikes = 2
	}
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT c.id::text, c.doc_id::text, c.kb_type, c.quality_score,
		       COUNT(DISTINCT s.strike_date)::int AS strike_days
		FROM kb_chunks c
		JOIN ragops_chunk_strikes s ON s.chunk_id = c.id AND s.kb_type = c.kb_type
		WHERE c.quality_score < $1
		  AND s.strike_date >= CURRENT_DATE - ($2 || ' days')::interval
		GROUP BY c.id, c.doc_id, c.kb_type, c.quality_score
		HAVING COUNT(DISTINCT s.strike_date) >= $3
		ORDER BY c.quality_score ASC, strike_days DESC
		LIMIT $4
	`, qualityBelow, fmt.Sprint(strikeDays), minStrikes, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []LowQualityChunk
	for rows.Next() {
		var row LowQualityChunk
		if err := rows.Scan(&row.ChunkID, &row.DocID, &row.KBType, &row.QualityScore, &row.StrikeDays); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, rows.Err()
}

// ChunkDocID returns doc_id for a chunk.
func (r *ChunkRepo) ChunkDocID(ctx context.Context, kbType, chunkID string) (string, error) {
	cid, err := uuid.Parse(chunkID)
	if err != nil {
		return "", err
	}
	var docID string
	err = r.client.pool.QueryRow(ctx, `
		SELECT doc_id::text FROM kb_chunks WHERE id = $1 AND kb_type = $2
	`, cid, kbType).Scan(&docID)
	return docID, err
}

// DemoteChunk lowers retrieval weight via quality_score (O-19 heal action).
func (r *ChunkRepo) DemoteChunk(ctx context.Context, kbType, chunkID string, penalty float64) error {
	if penalty <= 0 {
		penalty = 0.15
	}
	cid, err := uuid.Parse(chunkID)
	if err != nil {
		return err
	}
	_, err = r.client.pool.Exec(ctx, `
		UPDATE kb_chunks
		SET quality_score = GREATEST(0, quality_score - $3), updated_at = now()
		WHERE id = $1 AND kb_type = $2
	`, cid, kbType, penalty)
	return err
}
