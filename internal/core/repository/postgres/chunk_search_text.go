package postgres

// chunkSearchTextExpr is the FTS/rerank text column (3.5 semantic identity injection).
const chunkSearchTextExpr = `COALESCE(NULLIF(c.search_text, ''), c.content)`
