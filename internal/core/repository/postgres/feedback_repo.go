package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// VoteFeedbackResult aggregates DB changes from up/down vote.
type VoteFeedbackResult struct {
	ChunksUpdated int
	Docs          []VoteDocResult
	DeletedDocIDs []string
}

// VoteDocResult is per-document quality change.
type VoteDocResult struct {
	DocID         string
	OldQuality    float64
	NewQuality    float64
	Deleted       bool
	ChunksTouched int
}

// ApplyDownvote lowers quality_score on chunks/docs; deletes docs below deleteBelow.
func (r *ChunkRepo) ApplyDownvote(
	ctx context.Context,
	kbType string,
	chunkIDs []string,
	penalty, deleteBelow float64,
) (*VoteFeedbackResult, error) {
	if penalty <= 0 {
		penalty = 0.12
	}
	if deleteBelow <= 0 {
		deleteBelow = 0.35
	}
	return r.applyVote(ctx, kbType, chunkIDs, voteParams{
		chunkSQL:    `SET quality_score = GREATEST(0, quality_score - $1)`,
		chunkArg:    penalty,
		docAgg:      `COALESCE(MIN(quality_score), $3)`,
		deleteBelow: deleteBelow,
	})
}

// ApplyUpvote raises quality_score on chunks/docs (capped at 1).
func (r *ChunkRepo) ApplyUpvote(
	ctx context.Context,
	kbType string,
	chunkIDs []string,
	boost float64,
) (*VoteFeedbackResult, error) {
	if boost <= 0 {
		boost = 0.08
	}
	return r.applyVote(ctx, kbType, chunkIDs, voteParams{
		chunkSQL: `SET quality_score = LEAST(1, quality_score + $1)`,
		chunkArg: boost,
		docAgg:   `COALESCE(MAX(quality_score), $3)`,
	})
}

type voteParams struct {
	chunkSQL    string
	chunkArg    float64
	docAgg      string
	deleteBelow float64 // 0 = no delete
}

func (r *ChunkRepo) applyVote(
	ctx context.Context,
	kbType string,
	chunkIDs []string,
	p voteParams,
) (*VoteFeedbackResult, error) {
	ids, err := parseUUIDs(chunkIDs)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return &VoteFeedbackResult{}, nil
	}

	tx, err := r.client.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	sql := fmt.Sprintf(`
		UPDATE kb_chunks
		%s, updated_at = now()
		WHERE id = ANY($2::uuid[])
		  AND ($3 = '' OR kb_type = $3)
	`, p.chunkSQL)
	tag, err := tx.Exec(ctx, sql, p.chunkArg, ids, kbType)
	if err != nil {
		return nil, fmt.Errorf("update chunks: %w", err)
	}
	chunksUpdated := int(tag.RowsAffected())

	docRows, err := tx.Query(ctx, `
		SELECT DISTINCT doc_id::text
		FROM kb_chunks
		WHERE id = ANY($1::uuid[])
		  AND ($2 = '' OR kb_type = $2)
	`, ids, kbType)
	if err != nil {
		return nil, err
	}
	var docIDs []string
	for docRows.Next() {
		var docID string
		if err := docRows.Scan(&docID); err != nil {
			docRows.Close()
			return nil, err
		}
		docIDs = append(docIDs, docID)
	}
	docRows.Close()
	if err := docRows.Err(); err != nil {
		return nil, err
	}

	out := &VoteFeedbackResult{ChunksUpdated: chunksUpdated}
	for _, docID := range docIDs {
		docUUID, err := uuid.Parse(docID)
		if err != nil {
			continue
		}
		var oldQ float64
		err = tx.QueryRow(ctx, `
			SELECT quality_score FROM kb_documents WHERE id = $1 AND ($2 = '' OR kb_type = $2)
		`, docUUID, kbType).Scan(&oldQ)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return nil, err
		}

		aggSQL := fmt.Sprintf(`
			SELECT %s FROM kb_chunks WHERE doc_id = $1 AND ($2 = '' OR kb_type = $2)
		`, p.docAgg)
		var newQ float64
		if err := tx.QueryRow(ctx, aggSQL, docUUID, kbType, oldQ).Scan(&newQ); err != nil {
			return nil, err
		}

		_, err = tx.Exec(ctx, `
			UPDATE kb_documents
			SET quality_score = $3, updated_at = now()
			WHERE id = $1 AND ($2 = '' OR kb_type = $2)
		`, docUUID, kbType, newQ)
		if err != nil {
			return nil, err
		}

		res := VoteDocResult{DocID: docID, OldQuality: oldQ, NewQuality: newQ, ChunksTouched: 1}
		if p.deleteBelow > 0 && newQ < p.deleteBelow {
			delTag, err := tx.Exec(ctx, `
				DELETE FROM kb_documents WHERE id = $1 AND ($2 = '' OR kb_type = $2)
			`, docUUID, kbType)
			if err != nil {
				return nil, fmt.Errorf("delete doc: %w", err)
			}
			if delTag.RowsAffected() > 0 {
				res.Deleted = true
				out.DeletedDocIDs = append(out.DeletedDocIDs, docID)
			}
		}
		out.Docs = append(out.Docs, res)
	}

	if kbType != "" && len(out.Docs) > 0 {
		bumpCatalogVersionTx(ctx, tx, kbType)
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}
	return out, nil
}

// ListChunkIDsByDoc returns chunk ids for a document (for doc-level feedback).
func (r *ChunkRepo) ListChunkIDsByDoc(ctx context.Context, kbType, docID string) ([]string, error) {
	docUUID, err := uuid.Parse(docID)
	if err != nil {
		return nil, fmt.Errorf("invalid doc_id: %w", err)
	}
	rows, err := r.client.pool.Query(ctx, `
		SELECT id::text FROM kb_chunks
		WHERE doc_id = $1 AND ($2 = '' OR kb_type = $2)
	`, docUUID, kbType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	var out []uuid.UUID
	for _, s := range ids {
		if s == "" {
			continue
		}
		u, err := uuid.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid chunk_id %q: %w", s, err)
		}
		out = append(out, u)
	}
	return out, nil
}
