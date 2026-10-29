package model

// KBScope identifies a knowledge base in the PaaS API (tenant + kb) and its storage partition.
type KBScope struct {
	TenantID          string `json:"tenant_id"`
	KBID              string `json:"kb_id"`
	StoragePartition  string `json:"storage_partition"` // internal kb_type for Postgres partitions
}

// EnsureTenantRequest registers a tenant (idempotent).
type EnsureTenantRequest struct {
	TenantID    string `json:"tenant_id" binding:"required"`
	DisplayName string `json:"display_name,omitempty"`
}

// EnsureKnowledgeBaseRequest registers a KB under a tenant (idempotent).
type EnsureKnowledgeBaseRequest struct {
	KBID              string `json:"kb_id" binding:"required"`
	DisplayName       string `json:"display_name,omitempty"`
	Description       string `json:"description,omitempty"`
	StoragePartition  string `json:"storage_partition,omitempty"` // optional; auto-generated if empty
}
