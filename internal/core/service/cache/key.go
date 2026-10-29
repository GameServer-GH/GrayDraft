package cache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// LookupParams identifies a cacheable search.
type LookupParams struct {
	Query          string
	KBType         string
	TopK           int
	UseGlobalIndex bool
	CatalogVersion int64
	CatalogUpdated time.Time
	QueryIntent    string
	// Semantic4DFingerprint is set when 4D SQL pre-filter is applied (see model.QuerySemantic4D.CacheFingerprint).
	Semantic4DFingerprint string
}

// BuildExactKey returns a cache key scoped to KB + query + intent.
// Uses CatalogUpdated timestamp (second granularity) for invalidation
// instead of the global CatalogVersion counter, so cache entries survive
// unrelated document imports to the same KB type.
// Purges on import are handled explicitly via RedisPurgeByKBType.
func BuildExactKey(p LookupParams) string {
	norm := normalizeQuery(p.Query)
	raw := fmt.Sprintf("%s|%s|%d|%d|%t|%s|%s",
		norm,
		p.KBType,
		p.TopK,
		p.CatalogUpdated.Unix(),
		p.UseGlobalIndex,
		p.QueryIntent,
		p.Semantic4DFingerprint,
	)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

func normalizeQuery(q string) string {
	return strings.ToLower(strings.TrimSpace(q))
}

// IndexKey groups semantic cache entries per kb catalog revision and optional query 4D fingerprint.
func IndexKey(kbType string, catalogVersion int64, catalogUpdated time.Time, semantic4DFingerprint string) string {
	fp := strings.TrimSpace(semantic4DFingerprint)
	if fp == "" {
		return fmt.Sprintf("%s:%d:%d", kbType, catalogVersion, catalogUpdated.Unix())
	}
	return fmt.Sprintf("%s:%d:%d:%s", kbType, catalogVersion, catalogUpdated.Unix(), fp)
}
