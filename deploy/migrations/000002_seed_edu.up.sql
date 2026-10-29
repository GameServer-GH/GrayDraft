-- 教育场景初始数据：tutorial / paper / note，以及两条教程示例文档。

INSERT INTO public.kb_types (name, description, created_at, context_profile) VALUES ('tutorial', '书生教程、模型卡与 API 说明', '2026-10-28 11:59:06.045593+00', NULL);
INSERT INTO public.kb_types (name, description, created_at, context_profile) VALUES ('paper', '公开论文与技术报告页面', '2026-10-28 11:59:06.045593+00', NULL);
INSERT INTO public.kb_types (name, description, created_at, context_profile) VALUES ('note', '审批后沉淀的学习笔记', '2026-10-28 11:59:06.045593+00', NULL);

INSERT INTO public.kb_catalog_version (kb_type, version, latest_doc_updated_at, updated_at) VALUES ('tutorial', 1, '2026-10-28 11:59:06.045593+00', '2026-10-28 11:59:06.045593+00');
INSERT INTO public.kb_catalog_version (kb_type, version, latest_doc_updated_at, updated_at) VALUES ('paper', 1, '2026-10-28 11:59:06.045593+00', '2026-10-28 11:59:06.045593+00');
INSERT INTO public.kb_catalog_version (kb_type, version, latest_doc_updated_at, updated_at) VALUES ('note', 1, '2026-10-28 11:59:06.045593+00', '2026-10-28 11:59:06.045593+00');

INSERT INTO public.kb_documents (id, kb_type, title, source_url, content_hash, kb_version, index_ready, quality_score, last_accessed_at, created_at, updated_at, extensions, storage_tier, index_version) VALUES ('a0000000-0000-4000-8000-000000000001', 'tutorial', '检索说明', NULL, NULL, 1, true, 0.9, NULL, '2026-10-28 11:59:06.300463+00', '2026-10-28 11:59:06.300463+00', '{}', 'hot', 1);
INSERT INTO public.kb_documents (id, kb_type, title, source_url, content_hash, kb_version, index_ready, quality_score, last_accessed_at, created_at, updated_at, extensions, storage_tier, index_version) VALUES ('a0000000-0000-4000-8000-000000000002', 'tutorial', '自适应混合检索', NULL, NULL, 1, true, 0.85, NULL, '2026-10-28 11:59:06.300463+00', '2026-10-28 11:59:06.300463+00', '{}', 'hot', 1);

INSERT INTO public.kb_chunks (id, doc_id, kb_type, chunk_index, content, embedding, quality_score, index_ready, metadata, created_at, updated_at, search_text, embedding_structured, index_version, chunker_version, chunk_position) VALUES ('b0000000-0000-4000-8000-000000000001', 'a0000000-0000-4000-8000-000000000001', 'tutorial', 0, 'Core 提供 POST /api/v1/retrieval/search 混合检索接口，仅返回 index_ready=true 的切片。', NULL, 0.9, true, '{}', '2026-10-28 11:59:06.300463+00', '2026-10-28 11:59:06.300463+00', NULL, NULL, 1, '1', '');
INSERT INTO public.kb_chunks (id, doc_id, kb_type, chunk_index, content, embedding, quality_score, index_ready, metadata, created_at, updated_at, search_text, embedding_structured, index_version, chunker_version, chunk_position) VALUES ('b0000000-0000-4000-8000-000000000002', 'a0000000-0000-4000-8000-000000000002', 'tutorial', 0, '自适应混合检索根据查询类型动态调整向量与全文权重，并通过 RRF 融合多路召回结果。', NULL, 0.85, true, '{}', '2026-10-28 11:59:06.300463+00', '2026-10-28 11:59:06.300463+00', NULL, NULL, 1, '1', '');
INSERT INTO public.kb_chunks (id, doc_id, kb_type, chunk_index, content, embedding, quality_score, index_ready, metadata, created_at, updated_at, search_text, embedding_structured, index_version, chunker_version, chunk_position) VALUES ('b0000000-0000-4000-8000-000000000003', 'a0000000-0000-4000-8000-000000000002', 'tutorial', 1, 'index_ready 字段确保编排器不会检索到尚未完成向量索引的半成品文档。', NULL, 0.8, true, '{}', '2026-10-28 11:59:06.300463+00', '2026-10-28 11:59:06.300463+00', NULL, NULL, 1, '1', '');

REFRESH MATERIALIZED VIEW public.kb_global_index;
