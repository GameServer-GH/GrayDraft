-- Deterministic, idempotent retrieval-quality fixture. Never use these IDs in production data.
INSERT INTO kb_types (name, description)
VALUES ('regression_fixture', 'Deterministic retrieval regression fixture')
ON CONFLICT (name) DO NOTHING;

INSERT INTO kb_documents (id, kb_type, title, content_hash, index_ready, quality_score, extensions)
VALUES
('c63de565-7b6c-523e-a059-b7f009445f44','regression_fixture','index_ready 状态说明','fixture-index-ready',true,0.9,'{}'),
('1e77c99f-c8d4-5741-89a4-03c925860933','regression_fixture','InternVL 使用说明','fixture-internvl-guide',true,0.9,'{}'),
('15c3473f-ffd5-58f9-a25c-dae47d3ce652','regression_fixture','InternLM 推理参数','fixture-internlm-params',true,0.9,'{}'),
('ca0b0248-6ad2-5237-bd1b-3e8b46d59a51','regression_fixture','InternVL 别名与简称','fixture-alias',true,0.9,'{}'),
('6a1c1868-3ea2-55f8-92e2-210010681e8f','regression_fixture','向量检索与全文检索','fixture-concept',true,0.9,'{}'),
('009a7494-8e91-5b77-80e6-ab5b1594e25d','regression_fixture','InternVL 与 InternLM 的分工','fixture-multi-entity',true,0.9,'{}'),
('86671b6e-f07e-54b4-942f-575f5f9a5bbd','regression_fixture','实验室日志','fixture-noise',true,0.5,'{}')
ON CONFLICT (id) DO UPDATE SET index_ready = true, updated_at = now();

INSERT INTO kb_chunks (id, doc_id, kb_type, chunk_index, content, search_text, quality_score, index_ready, metadata)
VALUES
('c02c536c-4fed-5265-9730-46a6d45dfb5f','c63de565-7b6c-523e-a059-b7f009445f44','regression_fixture',0,'index_ready 表示文档和切片已完成索引，可以进入检索候选池。','index_ready 状态说明 index_ready 文档 切片 索引 检索',0.9,true,'{}'),
('71ef378b-6685-5595-9422-c013875b05ec','1e77c99f-c8d4-5741-89a4-03c925860933','regression_fixture',0,'InternVL 是书生多模态模型。调用时关注视觉编码、输入长度和图像分辨率。','InternVL 使用说明 书生多模态 internvl 视觉编码 输入长度',0.9,true,'{}'),
('1ed6f064-ad40-559a-b12f-6fb708f77ab9','15c3473f-ffd5-58f9-a25c-dae47d3ce652','regression_fixture',0,'InternLM 是文本推理模型，部署时调整温度、上下文长度和批大小。','InternLM 推理参数 温度 上下文 部署',0.9,true,'{}'),
('94c73ae2-1d9e-5dc9-a1a6-bd55013d0e3c','ca0b0248-6ad2-5237-bd1b-3e8b46d59a51','regression_fixture',0,'InternVL、书生多模态和 internvl 是同一模型的正式名、中文名和仓库名。','InternVL 书生多模态 internvl 别名 简称',0.9,true,'{}'),
('88dfd99d-edcf-5740-8afd-21ae0555a786','6a1c1868-3ea2-55f8-92e2-210010681e8f','regression_fixture',0,'混合检索将全文排名和向量排名通过 RRF 融合，RRF 只使用排名位置。','混合检索 全文检索 向量检索 RRF 排名 融合',0.9,true,'{}'),
('034ec691-1de1-5479-ae42-7c5bfc9f4ee1','009a7494-8e91-5b77-80e6-ab5b1594e25d','regression_fixture',0,'InternVL 负责看图和文档版面，InternLM 负责文本推理。两者可以串联。','InternVL InternLM 多模态 文本推理 分工',0.9,true,'{}'),
('143c0d30-9ed6-5e5e-8d33-5864a0e36aab','86671b6e-f07e-54b4-942f-575f5f9a5bbd','regression_fixture',0,'今天沿着河边散步，天气晴朗。','实验室日志 河边 散步 天气',0.5,true,'{}')
ON CONFLICT (id, kb_type) DO UPDATE
SET content = EXCLUDED.content, search_text = EXCLUDED.search_text,
    quality_score = EXCLUDED.quality_score, index_ready = true, updated_at = now();
