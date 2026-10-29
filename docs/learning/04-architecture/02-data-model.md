 # 数据模型

 > **定位**：理解 PostgreSQL schema 设计、pgvector 索引策略、分区表与版本管理
 > **代码依据**：`deploy/migrations/` 迁移脚本 + `internal/core/repository/postgres/`

 ---

 ## 0. ER 关系图

 ```mermaid
 erDiagram
   tenants ||--o{ knowledge_bases : owns
   knowledge_bases ||--|| kb_types : "storage_partition"
   kb_types ||--o{ kb_documents : contains
   kb_documents ||--o{ kb_chunks : "LIST 分区"
   kb_documents ||--o| ugc_ingest_tasks : "via ingest"
   kb_types ||--o| kb_catalog_version : "缓存失效版本"
 ```

 ---

 ## 1. 核心表

 ### 1.1 tenants / knowledge_bases（PaaS 多租户）

 | 表 | 关键字段 | 说明 |
|----|----------|------|
| `tenants` | `tenant_id` (PK), `display_name` | 租户注册 |
| `knowledge_bases` | `tenant_id` (FK), `kb_id` (PK), `storage_partition` | 知识库 → 分区映射 |

 GrayDraft 映射：
 ```
 workspace_key → tenant_id = "tn_" + SHA256(key)[:16]
              → kb_id = "default"
              → storage_partition = "gd_" + SHA256(key)[:12]
 ```

 ### 1.2 kb_types（知识库分区注册）

 | 字段 | 类型 | 说明 |
|------|------|------|
| `name` | text (PK) | 分区名称（即 storage_partition） |
| `context_profile` | jsonb | 工作区背景（用于检索/对话 Prompt 注入） |

 ### 1.3 kb_documents（文档）

 | 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid (PK) | 文档 ID |
| `kb_type` | text | 所属分区 |
| `title` | text | 标题 |
| `content` | text | 原始正文 |
| `content_hash` | text | MD5 去重哈希 |
| `kb_version` | int | 同标题更新版本号（单调递增） |
| `index_ready` | bool | 是否可检索（默认 false） |
| `quality_score` | float | 质量分（feedback 驱动） |
| `extensions` | jsonb | 业务自定义扩展 |

 ### 1.4 kb_chunks（切片 — LIST 分区表）

 | 字段 | 类型 | 说明 |
|------|------|------|
| `id` | uuid (PK) | 切片 ID |
| `doc_id` | uuid (FK) | 所属文档 |
| `kb_type` | text | 分区键 |
| `position` | int | 在原文档中的位置 |
| `text` | text | 切片正文 |
| `embedding` | vector(1024) | 常规向量 |
| `embedding_structured` | vector(1024) | 结构化向量（可选） |
| `search_text` | text | 检索优化文本 |
| `semantic_tags_4d` | jsonb | 4D 语义标签 |

 ---

 ## 2. pgvector 索引

 ### 2.1 IVFFlat 索引

 ```sql
 CREATE INDEX ON kb_chunks
   USING ivfflat (embedding vector_cosine_ops)
   WITH (lists = 100);
 ```

 | 参数 | 说明 | 建议值 |
|------|------|--------|
| `lists` | 聚类中心数 | 行数开方（10K 行 → 100 lists） |
| `probes` | 查询时探测的聚类数 | `sqrt(lists)` 起步（查询时设置） |

 **调 probes**：
 ```sql
 SET ivfflat.probes = 20;  -- 越大越精确越慢
 ```

 ### 2.2 索引维护

 - **何时重建**：数据量增长 2-3 倍后，lists 数应相应调整
 - **重建命令**：`REINDEX INDEX CONCURRENTLY idx_name;`
 - **Embedding 模型切换**：必须重建索引（向量语义空间变化）

 ---

 ## 3. LIST 分区

 `kb_chunks` 按 `kb_type` 做 LIST 分区：

 ```sql
 CREATE TABLE kb_chunks (...) PARTITION BY LIST (kb_type);

 CREATE TABLE kb_chunks_tutorial PARTITION OF kb_chunks
   FOR VALUES IN ('tutorial');

 CREATE TABLE kb_chunks_paper PARTITION OF kb_chunks
   FOR VALUES IN ('paper');
 ```

 **优势**：
- 查询自动裁剪：`WHERE kb_type = 'tutorial'` 只扫描 tutorial 分区
- 数据隔离：按分区独立维护（VACUUM、REINDEX）
- 删除高效：DROP PARTITION 比 DELETE 快数百倍

 ---

 ## 4. catalog_version 版本管理

 ```sql
 CREATE TABLE kb_catalog_version (
   kb_type     text PRIMARY KEY,
   version     int NOT NULL DEFAULT 0,
   updated_at  timestamptz NOT NULL DEFAULT now()
 );
 ```

 **何时 bump**：

| 事件 | bump 原因 |
|------|----------|
| 新文档入库 | 内容变化，缓存需刷新 |
| Blueprint 变更 | 标签体系变化，检索结果可能不同 |
| 影子回填完成 | 存量标签已更新 |

 **如何影响缓存**：

 ```
 catalog_version = 15
   → 缓存 key 含 "v15"
   → bump → 16
   → 旧 key "v15" 不再命中
   → 新查询自动生成 "v16" key
 ```

 ---

 ## 5. 迁移脚本体系

 路径：`deploy/migrations/`

 ```
 deploy/migrations/
 ├── 000001_init.up.sql
 ├── 000001_init.down.sql
 ├── 000002_seed_edu.up.sql
 └── 000002_seed_edu.down.sql
 ```

 **约定**：
- 文件名：`NNNNNN_description.{up,down}.sql`
- 所有 DDL 使用 `IF NOT EXISTS` / `IF EXISTS` 保证幂等
- 启动时 `RunMigrations` 按序号顺序执行所有 up 脚本
- down 脚本不自动执行（仅手动回滚用）

 ---

 ## 6. 学习练习

 1. 画出 tenants → knowledge_bases → kb_types → kb_documents → kb_chunks 的完整 ER 图。
 2. 如果 `kb_chunks` 不做分区，多个 kb_type 各 10 万行，`WHERE kb_type = 'tutorial'` 查询会有多慢？
 3. 模拟 catalog_version bump：写一条 SQL 更新 version，然后验证新查询是否生成新 key。
 4. 阅读 `000001_init.up.sql`，解释为什么要引入 `knowledge_bases` 表。

 ---

 ## 7. 相关文档

 - [系统逻辑总览 §3](../../architecture/系统逻辑总览.md)
 - [Core 服务架构](../04-architecture/01-core-service.md)
 - [embedding.md](../../api/embedding.md)
 - [workspace-background.md](../../features/workspace-background.md)
