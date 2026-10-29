-- 回填 search_text：标题 + extensions + tags + 正文（与 Worker build_search_text 对齐）
WITH doc_entities AS (
    SELECT
        d.id AS doc_id,
        d.kb_type,
        COALESCE(d.title, '') AS title,
        COALESCE(
            NULLIF(
                concat_ws(
                    '/',
                    NULLIF(trim(d.title), ''),
                    NULLIF(trim(d.extensions->>'subject'), ''),
                    NULLIF(trim(d.extensions->>'parent_title'), ''),
                    NULLIF(trim(d.extensions->>'entity'), ''),
                    NULLIF(trim(d.extensions->>'aliases'), ''),
                ),
                ''
            ),
            COALESCE(d.title, '')
        ) AS entity_str
    FROM kb_documents d
),
doc_tags AS (
    SELECT doc_id, string_agg(DISTINCT tag, '/') AS tag_str
    FROM kb_doc_tags
    GROUP BY doc_id
)
UPDATE kb_chunks c
SET search_text = format(
    '[文档标题：%s] [主题：%s] [正文：%s]',
    e.title,
    CASE
        WHEN t.tag_str IS NOT NULL AND t.tag_str <> '' THEN e.entity_str || '/' || t.tag_str
        ELSE e.entity_str
    END,
    c.content
),
updated_at = now()
FROM doc_entities e
LEFT JOIN doc_tags t ON t.doc_id = e.doc_id
WHERE c.doc_id = e.doc_id
  AND c.kb_type = e.kb_type;
