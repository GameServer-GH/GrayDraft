package postgres

// chunkExtensionsSelect merges document extensions with per-chunk metadata (chunk keys override).
const chunkExtensionsSelect = `(COALESCE(d.extensions, '{}'::jsonb) || COALESCE(c.metadata, '{}'::jsonb)) AS extensions`

// docExtensionsSelect is the raw document-level extensions (for frontend article_ref lookup).
const docExtensionsSelect = `COALESCE(d.extensions, '{}'::jsonb) AS doc_extensions`
