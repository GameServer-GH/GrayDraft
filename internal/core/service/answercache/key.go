package answercache

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"strings"
	"time"
)

// LookupParams identifies a cacheable chat answer.
type LookupParams struct {
	Query                 string
	KBType                string
	TopK                  int
	CatalogVersion        int64
	CatalogUpdated        time.Time
	Semantic4DFingerprint string
	Grade                 string // Q&A optimization v2: A/B/C/D
}

// BuildKey returns a version-aware exact cache key including grade.
func BuildKey(p LookupParams) string {
	norm := strings.ToLower(strings.TrimSpace(p.Query))
	raw := fmt.Sprintf("%s|%s|%d|%d|%d|%s|%s",
		norm,
		p.KBType,
		p.TopK,
		p.CatalogVersion,
		p.CatalogUpdated.Unix(),
		p.Semantic4DFingerprint,
		p.Grade,
	)
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// BuildKeyPattern returns a Redis SCAN pattern matching all grade variants of a query.
func BuildKeyPattern(p LookupParams) string {
	norm := strings.ToLower(strings.TrimSpace(p.Query))
	raw := fmt.Sprintf("%s|%s|%d",
		norm,
		p.KBType,
		p.TopK,
	)
	sum := md5.Sum([]byte(raw))
	prefix := hex.EncodeToString(sum[:])[:16]
	return prefix + "*"
}
