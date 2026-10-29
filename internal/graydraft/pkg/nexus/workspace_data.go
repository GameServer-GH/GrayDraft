package nexus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
)

// WorkspaceDataStats is GET /api/v1/kb/{kb_type}/workspace-data/stats.
type WorkspaceDataStats struct {
	KBType                 string `json:"kb_type"`
	KnowledgeDocuments     int64  `json:"knowledge_documents"`
	RedisRetrievalEntries  int    `json:"redis_retrieval_entries"`
	RedisAnswerEntries     int    `json:"redis_answer_entries"`
}

// GetWorkspaceDataStats returns knowledge and redis cache counts for a kb_type partition.
func (c *Client) GetWorkspaceDataStats(ctx context.Context, kbType string) (*WorkspaceDataStats, error) {
	if c == nil {
		return nil, fmt.Errorf("nexus client not configured")
	}
	kb := url.PathEscape(kbType)
	u := fmt.Sprintf("%s/api/v1/kb/%s/workspace-data/stats", c.base(), kb)
	var out WorkspaceDataStats
	if err := c.getJSON(ctx, u, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PurgeWorkspaceDataResponse is POST /api/v1/kb/{kb_type}/workspace-data/purge.
type PurgeWorkspaceDataResponse struct {
	KBType               string `json:"kb_type"`
	Target               string `json:"target"`
	DocumentsRemoved     int64  `json:"documents_removed,omitempty"`
	RetrievalKeysRemoved int    `json:"retrieval_keys_removed,omitempty"`
	AnswerKeysRemoved    int    `json:"answer_keys_removed,omitempty"`
}

// PurgeWorkspaceData clears knowledge documents or redis hot cache for kb_type.
// target must be "knowledge" or "redis".
func (c *Client) PurgeWorkspaceData(ctx context.Context, kbType, target string) (*PurgeWorkspaceDataResponse, error) {
	if c == nil {
		return nil, fmt.Errorf("nexus client not configured")
	}
	body, _ := json.Marshal(map[string]string{"target": target})
	kb := url.PathEscape(kbType)
	path := fmt.Sprintf("/api/v1/kb/%s/workspace-data/purge", kb)
	var out PurgeWorkspaceDataResponse
	if err := c.postJSON(ctx, path, body, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
