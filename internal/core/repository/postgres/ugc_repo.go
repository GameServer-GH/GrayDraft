package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// UGCRepo tracks async ingest tasks.
type UGCRepo struct {
	client *Client
}

// NewUGCRepo creates a UGC task repository.
func NewUGCRepo(client *Client) *UGCRepo {
	return &UGCRepo{client: client}
}

// KBTypeExists checks kb_types registry.
func (r *UGCRepo) KBTypeExists(ctx context.Context, kbType string) (bool, error) {
	var exists bool
	err := r.client.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM kb_types WHERE name = $1)
	`, kbType).Scan(&exists)
	return exists, err
}

// CreateTask inserts a pending ingest task.
func (r *UGCRepo) CreateTask(ctx context.Context, taskID uuid.UUID, kbType, title string, docID *uuid.UUID) error {
	_, err := r.client.pool.Exec(ctx, `
		INSERT INTO ugc_ingest_tasks (id, doc_id, kb_type, title, status)
		VALUES ($1, $2, $3, $4, 'pending')
	`, taskID, docID, kbType, title)
	return err
}

// UpdateTaskCompleted marks task done after Worker ingest.
func (r *UGCRepo) UpdateTaskCompleted(ctx context.Context, taskID uuid.UUID, docID uuid.UUID, chunkCount int) error {
	_, err := r.client.pool.Exec(ctx, `
		UPDATE ugc_ingest_tasks
		SET status = 'completed', doc_id = $2, chunk_count = $3,
		    current_stage = COALESCE(NULLIF(current_stage, ''), 'indexed'),
		    ready_at = COALESCE(ready_at, now()),
		    updated_at = now()
		WHERE id = $1
	`, taskID, docID, chunkCount)
	return err
}

// SetTaskStage records the active ingest stage for observability (P2-1).
func (r *UGCRepo) SetTaskStage(ctx context.Context, taskID uuid.UUID, stage string) error {
	if stage == "" {
		return nil
	}
	_, err := r.client.pool.Exec(ctx, `
		UPDATE ugc_ingest_tasks
		SET current_stage = $2, updated_at = now()
		WHERE id = $1
	`, taskID, stage)
	return err
}

// UpdateTaskFailed marks task failed.
func (r *UGCRepo) UpdateTaskFailed(ctx context.Context, taskID uuid.UUID, errMsg string) error {
	_, err := r.client.pool.Exec(ctx, `
		UPDATE ugc_ingest_tasks
		SET status = 'failed', error_message = $2, updated_at = now()
		WHERE id = $1
	`, taskID, errMsg)
	return err
}

// StageEvent is a stage transition log entry (V1-09).
type StageEvent struct {
	TaskID         string `json:"task_id"`
	TraceID        string `json:"trace_id,omitempty"`
	Stage          string `json:"stage"`
	Status         string `json:"status"`
	Attempt        int    `json:"attempt"`
	IndexVersion   *int   `json:"index_version,omitempty"`
	ChunkerVersion string `json:"chunker_version,omitempty"`
	ErrorCode      string `json:"error_code,omitempty"`
	ErrorMessage   string `json:"error_message,omitempty"`
}

// InsertStageEvent writes a stage transition record (V1-09).
func (r *UGCRepo) InsertStageEvent(ctx context.Context, ev StageEvent) error {
	taskUUID, err := uuid.Parse(ev.TaskID)
	if err != nil {
		return fmt.Errorf("invalid task_id: %w", err)
	}
	_, err = r.client.pool.Exec(ctx, `
		INSERT INTO ugc_ingest_stage_events
			(task_id, trace_id, stage, status, attempt,
			 index_version, chunker_version, error_code, error_message)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`, taskUUID, ev.TraceID, ev.Stage, ev.Status, ev.Attempt,
		ev.IndexVersion, ev.ChunkerVersion, ev.ErrorCode, ev.ErrorMessage)
	return err
}

// ResetTaskPending marks a failed task as pending for DLQ replay.
func (r *UGCRepo) ResetTaskPending(ctx context.Context, taskID uuid.UUID) error {
	_, err := r.client.pool.Exec(ctx, `
		UPDATE ugc_ingest_tasks
		SET status = 'pending', error_message = NULL, updated_at = now()
		WHERE id = $1 AND status IN ('failed', 'processing')
	`, taskID)
	return err
}

// GetTask returns task status for polling API.
func (r *UGCRepo) GetTask(ctx context.Context, taskID uuid.UUID) (map[string]any, error) {
	row := r.client.pool.QueryRow(ctx, `
		SELECT
			t.id::text,
			COALESCE(t.doc_id::text, ''),
			t.kb_type,
			COALESCE(t.title, ''),
			t.status,
			COALESCE(t.error_message, ''),
			t.chunk_count,
			t.created_at,
			t.updated_at,
			d.id IS NOT NULL,
			COALESCE(d.index_ready, false),
			COUNT(c.id)::int,
			(COUNT(c.id) FILTER (WHERE c.index_ready))::int,
			(COUNT(c.id) FILTER (WHERE c.index_ready AND c.embedding IS NOT NULL))::int
		FROM ugc_ingest_tasks t
		LEFT JOIN kb_documents d ON d.id = t.doc_id
		LEFT JOIN kb_chunks c ON c.doc_id = t.doc_id AND c.kb_type = t.kb_type
		WHERE t.id = $1
		GROUP BY t.id, d.id, d.index_ready
	`, taskID)

	var id, docID, kbType, title, status, errMsg string
	var chunkCount, totalChunkCount, indexedChunkCount, embeddedChunkCount int
	var docExists, indexReady bool
	var createdAt, updatedAt any
	if err := row.Scan(
		&id, &docID, &kbType, &title, &status, &errMsg, &chunkCount, &createdAt, &updatedAt,
		&docExists, &indexReady, &totalChunkCount, &indexedChunkCount, &embeddedChunkCount,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}
	chunkCountMatches := chunkCount <= 0 || totalChunkCount >= chunkCount
	queryable := status == "completed" && docID != "" && indexReady && indexedChunkCount > 0 && chunkCountMatches
	embeddingReady := indexedChunkCount > 0 && embeddedChunkCount >= indexedChunkCount
	readiness := "queued"
	blockingReason := ""
	switch {
	case status == "failed":
		readiness = "failed"
		blockingReason = errMsg
	case queryable:
		readiness = "ready"
	case status == "completed" && !docExists:
		readiness = "completed_without_doc"
		blockingReason = "task completed but document row is missing"
	case status == "completed" && !indexReady:
		readiness = "completed_not_indexed"
		blockingReason = "document index_ready is false"
	case status == "completed" && indexedChunkCount == 0:
		readiness = "completed_without_chunks"
		blockingReason = "no index_ready chunks found"
	case status == "completed" && !chunkCountMatches:
		readiness = "completed_chunk_mismatch"
		blockingReason = "indexed chunk count has not caught up with task chunk_count"
	case status == "processing":
		readiness = "indexing"
		blockingReason = "worker is processing the task"
	case status == "pending":
		readiness = "queued"
		blockingReason = "task is queued"
	}
	validation := map[string]any{
		"task_completed":       status == "completed",
		"doc_exists":           docExists,
		"document_index_ready": indexReady,
		"chunks_indexed":       indexedChunkCount > 0,
		"chunk_count_matches":  chunkCountMatches,
		"embedding_ready":      embeddingReady,
		"blocking_reason":      blockingReason,
	}
	return map[string]any{
		"task_id": id, "doc_id": docID, "kb_type": kbType, "title": title,
		"status": status, "error_message": errMsg, "chunk_count": chunkCount,
		"doc_exists":  docExists,
		"index_ready": indexReady, "total_chunk_count": totalChunkCount,
		"indexed_chunk_count": indexedChunkCount, "embedded_chunk_count": embeddedChunkCount,
		"embedding_ready": embeddingReady,
		"queryable":       queryable, "readiness": readiness,
		"readiness_validation": validation,
		"created_at":           createdAt, "updated_at": updatedAt,
	}, nil
}

// GetStageEvents returns all stage transition events for a task (V1-09).
func (r *UGCRepo) GetStageEvents(ctx context.Context, taskID uuid.UUID) ([]map[string]any, error) {
	rows, err := r.client.pool.Query(ctx, `
		SELECT
			id,
			COALESCE(trace_id, ''),
			stage,
			status,
			attempt,
			index_version,
			COALESCE(chunker_version, ''),
			COALESCE(error_code, ''),
			COALESCE(error_message, ''),
			created_at
		FROM ugc_ingest_stage_events
		WHERE task_id = $1
		ORDER BY created_at
	`, taskID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []map[string]any
	for rows.Next() {
		var id int64
		var traceID, stage, status, chunkerVer, errorCode, errorMsg string
		var attempt, indexVersion *int
		var createdAt any
		if err := rows.Scan(&id, &traceID, &stage, &status, &attempt, &indexVersion,
			&chunkerVer, &errorCode, &errorMsg, &createdAt); err != nil {
			return nil, err
		}
		ev := map[string]any{
			"id":              id,
			"trace_id":        traceID,
			"stage":           stage,
			"status":          status,
			"attempt":         attempt,
			"index_version":   indexVersion,
			"chunker_version": chunkerVer,
			"error_code":      errorCode,
			"error_message":   errorMsg,
			"created_at":      createdAt,
		}
		events = append(events, ev)
	}
	return events, rows.Err()
}
