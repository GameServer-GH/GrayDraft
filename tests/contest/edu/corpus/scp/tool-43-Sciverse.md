# Sciverse
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：5
调用次数：1511
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/43/Sciverse
标签：通用, 数据库

## 介绍
这是一款面向科学大模型与AI科学家的学术文献检索服务。该平台覆盖学术文献、图书、全球专利等多类科研数据，围绕多种真实科学问题组织数据，助推科研应用。

## 详细说明与使用方法
# Sciverse SCP Server

📚 由上海人工智能实验室（[OpenDataLab](https://github.com/opendatalab)）打造的 Sciverse SCP Server，是一款面向科学大模型与 AI 科学家的**全模态 AI-Ready 数据基座**的学术文献检索服务。该平台覆盖**学术文献、图书、全球专利**等多类科研数据，已积累 **516M+ 知识记录、814 种语言**，并围绕 CRISPR 基因编辑、蛋白质折叠预测、COVID-19 长期效应、逆合成路径规划、阿尔茨海默症靶点、锂电池固态电解质等真实科学问题组织数据。本 Server 把 Sciverse 的检索能力封装为 5 个标准化工具，帮助科研智能体快速完成文献调研、RAG 上下文构建、图表证据获取与可追溯引用。

## ✨ Core Features

1. **可引用级（citation-grade）的学术文献检索**

   每条返回结果都带稳定的 `doc_id` / `chunk_id`，智能体可直接在产出中标注来源，跨次调用结果可复现，便于人工与自动复核回追，避免「自信幻觉但无出处」的常见 LLM 输出问题。

2. **自然语言语义检索 + 结构化过滤**

   `semantic_search` 在论文片段（chunks）级别做语义匹配，针对 RAG 场景直接返回相关段落与 `score`；`search_papers` 提供作者、年份、期刊、学科等结构化过滤；`list_catalog` 暴露完整可过滤字段与枚举样本值，三者配合可精确定位到与问题真正相关的论文片段。

3. **原文上下文按需扩展 + 多模态图表证据**

   `read_content` 支持按字节偏移在论文原文中前后扩展上下文（最大 16KB），让 agent 按需读取更长窗口而无需一次性返回整篇全文；`get_resource` 把论文中引用的图、表图片以 MCP image content block 形式返回，多模态智能体（Claude / GPT-4V / Gemini 等）可直接读图像证据。

4. **标准化 MCP 协议，开箱即用**

   所有能力均通过标准化的 MCP（Model Calling Protocol）协议封装，提供一致的 API 调用方式。研究人员无需本地部署检索引擎或对接多家学术数据源，仅需几行 Python 代码即可远程访问完整服务，显著降低使用门槛，让使用者专注于科学探索而非工程实现。


## 🛠️ Tool List

| 工具名称 | 功能描述 | 参数 |
|----|------|------|
| **search_papers** | 按结构化条件检索学术文献元数据，返回论文列表（含 `doc_id`、`title`、`abstract`、`author`、期刊、年份等） | `query`（可选）: BM25 关键词，匹配标题/摘要/期刊/关键词<br>`authors`（可选）: 作者名列表，任一命中即可<br>`year_from` / `year_to`（可选）: 发表年范围（含）<br>`journals`（可选）: 期刊名列表<br>`subjects`（可选）: 学科分类<br>`title_contains` / `abstract_contains`（可选）: 标题/摘要必含词<br>`sort_by_year`（可选，默认 `desc`）<br>`page` / `page_size`（可选，默认 1 / 10，最大 50） |
| **semantic_search** | 自然语言语义检索，返回论文片段（chunks），适合 RAG 场景 | `query`（必选）: 自然语言查询，1-200 字最佳<br>`top_k`（可选，默认 10，最大 30）<br>`mode`（可选，默认 `balanced`）: `fast`（~200ms）/ `balanced`（~600ms）/ `quality`（2-4s，LLM 改写 + 混合检索）<br>`source_types`（可选）: `web` / `pdf` |
| **read_content** | 按字节偏移读取论文原文片段，扩展上下文窗口 | `doc_id`（必选）: 论文 ID（来自 search_papers / semantic_search）<br>`offset`（可选，默认 0）: 起始字节偏移<br>`limit`（可选，默认 4096，最大 16384）: 读取字节数 |
| **get_resource** | 获取论文中引用的图 / 表图片（read_content 返回的 Markdown 中 `![alt](file_name)` 占位） | `file_name`（必选）: 图片相对路径（禁止 `..` 或以 `/` 开头） |
| **list_catalog** | 列出 search_papers 可过滤字段、操作符、枚举样本值；构造精确过滤前可调用 | `include_sample_values`（可选，默认 `false`）: 是否拉取 enum-like 字段取值样本 |


## 🚀 Quick Start

#### 1. Dependence

推荐使用 Python 3.10+，仅需安装 `mcp` 即可：

```bash
pip install mcp
```

#### 2. Configuration

请参考以下代码定义 Server 客户端，其中包含 `connect`、`disconnect`、`list_tools` 和 `parse_result` 方法（注意替换 SCP-HUB-API-KEY）：

```python
import asyncio
import json
import base64
from mcp.client.streamable_http import streamablehttp_client
from mcp import ClientSession


class SciverseClient:

    def __init__(self, server_url: str = "https://scp.intern-ai.org.cn/api/v1/mcp/43/Sciverse"):
        self.server_url = server_url
        self.session = None

    async def connect(self):
        """建立连接并初始化会话"""
        print(f"\n{'='*80}")
        print("连接到 Sciverse SCP Server")
        print(f"{'='*80}")
        print(f"服务器地址: {self.server_url}")

        try:
            self.transport = streamablehttp_client(
                url=self.server_url,
                headers={"SCP-HUB-API-KEY": "sk-xxx"}
            )
            self.read, self.write, self.get_session_id = await self.transport.__aenter__()

            self.session_ctx = ClientSession(self.read, self.write)
            self.session = await self.session_ctx.__aenter__()

            await self.session.initialize()
            session_id = self.get_session_id()

            print(f"✓ 连接成功")
            print(f"✓ 会话ID: {session_id}")
            print(f"{'='*80}\n")
            return True

        except Exception as e:
            print(f"✗ 连接失败: {e}")
            import traceback
            traceback.print_exc()
            return False

    async def disconnect(self):
        """断开连接"""
        try:
            if self.session:
                await self.session_ctx.__aexit__(None, None, None)
            if hasattr(self, 'transport'):
                await self.transport.__aexit__(None, None, None)
            print("\n✓ 已断开连接\n")
        except Exception as e:
            print(f"✗ 断开连接时出错: {e}")

    async def list_tools(self):
        """列出所有可用工具"""
        tools_list = await self.session.list_tools()
        print(f"\n可用工具 (共{len(tools_list.tools)}个):\n")
        for i, tool in enumerate(tools_list.tools, 1):
            print(f"{i:2d}. {tool.name}")
            if tool.description:
                desc_line = tool.description.strip().split('\n')[0]
                print(f"    {desc_line}")
        return tools_list.tools

    def parse_result(self, result):
        """解析 SCP 工具调用结果（文本类工具：search_papers / semantic_search / read_content / list_catalog）"""
        try:
            if hasattr(result, 'content') and result.content:
                content = result.content[0]
                if hasattr(content, 'text'):
                    return json.loads(content.text)
            return str(result)
        except Exception as e:
            return {"error": f"解析结果失败: {e}", "raw": str(result)}

    def parse_image_result(self, result):
        """解析 get_resource 返回的图片 content block"""
        if isinstance(result, dict):
            content_list = result.get("content") or []
        else:
            content_list = getattr(result, "content", []) or []
        for item in content_list:
            data = item.get("data") if isinstance(item, dict) else getattr(item, "data", None)
            mime = item.get("mimeType") if isinstance(item, dict) else getattr(item, "mimeType", None)
            type_ = item.get("type") if isinstance(item, dict) else getattr(item, "type", None)
            if type_ == "image" and data:
                return {"mime_type": mime, "bytes": base64.b64decode(data)}
        return None
```

## 📊 Usage

以下展示 5 个工具的典型调用方法：

```python
async def main():
    ## 客户端创建和连接
    client = SciverseClient()
    if not await client.connect():
        print("连接失败")
        return

    try:
        ## 示例 1：结构化检索 —— 近期 transformer 相关论文
        result = await client.session.call_tool(
            "search_papers",
            arguments={
                "query": "transformer attention",
                "year_from": 2023,
                "page_size": 5,
            }
        )
        papers = client.parse_result(result)
        print(f"search_papers hits: {len(papers.get('hits', []))}")

        ## 示例 2：自然语言语义检索（RAG 入口）
        result = await client.session.call_tool(
            "semantic_search",
            arguments={
                "query": "How does transformer attention work?",
                "top_k": 3
            }
        )
        chunks = client.parse_result(result)
        for hit in chunks.get("hits", []):
            print(f"  - {hit['title']}  (doc_id={hit['doc_id']}, score={hit['score']:.2f})")

        ## 示例 3：按字节偏移读取原文上下文（向后扩展 4KB）
        if chunks.get("hits"):
            first = chunks["hits"][0]
            result = await client.session.call_tool(
                "read_content",
                arguments={
                    "doc_id": first["doc_id"],
                    "offset": first["offset"],
                    "limit": 4096
                }
            )
            text_window = client.parse_result(result)
            print(f"read_content next_offset={text_window.get('next_offset')} more={text_window.get('more')}")

        ## 示例 4：发现可过滤字段（构造精确过滤前用）
        result = await client.session.call_tool(
            "list_catalog",
            arguments={"include_sample_values": False}
        )
        catalog = client.parse_result(result)
        print(f"available filter fields: {len(catalog.get('fields', []))}")

        ## 示例 5：获取论文图 / 表图片（read_content Markdown 中引用的占位）
        # result = await client.session.call_tool(
        #     "get_resource",
        #     arguments={"file_name": "figures/fig-3.png"}
        # )
        # image = client.parse_image_result(result)
        # if image:
        #     from pathlib import Path
        #     Path("fig-3.png").write_bytes(image["bytes"])
    finally:
        ## 客户端断开
        await client.disconnect()


if __name__ == '__main__':
    asyncio.run(main())
```

## 工具列表与参数

### search_papers
Search academic papers by structured filters (title, authors, journal,
year, subjects, etc.).
Use when: "find Hinton's papers from 2020-2023", "Nature papers on
CRISPR".
Not for: natural-language Q&A retrieval (use semantic_search) or
full-text snippets (use read_content).
Returns: list of papers; each entry has unique_id (always present),
doc_id (only when full text exists), title, author, abstract,
publication_venue_name_unified, publication_published_year.
参数：
- `abstract_contains`（string，可选）：摘要中必须包含的词（仅匹配 abstract 字段）。
- `authors`（array，可选）：作者名（任一命中即可）。SDK 内部映射到后端 `author` 字段（FILTER_OP_IN）。
- `collection`（string，可选，默认 papers）：Entity collection to search. papers (default) / authors / sources. Each collection has its own field schema — call list_catalog(collection=<name>). The convenience fields (authors/journals/year_from/subjects) apply to papers only; for authors/sources use filters_advanced with that collection's field names.
- `filters_advanced`（array，可选）：高级过滤逃生舱（仅当上述字段不够用时使用）。可用字段见 get_field_catalog。  引文反查（常用）：field="references_unique_id" 查「谁引用了某篇论文」， value 填目标论文的 unique_id。相比 list_paper_relations 的 CITATIONS， 它支持深翻页与任意排序，适合超高被引论文。可叠加条件， 例如「引用了 ResNet 且 2023 年后发表」：   [{"field":"references_unique_id","value":"paper:10.1109/cvpr.2016.90"},    {"field":"publication_published_year","operator":"FILTER_OP_GTE","value":2023}] 该字段仅支持过滤，不能排序/聚合，也不能放进 fields 返回。
- `freshness_boost`（string，可选，默认 NONE）：模糊搜索新鲜度加权：结果偏向新文献（仅 query 非空时生效；传排序 （sort_by_year 非 none / sort_advanced）时被忽略，硬排优先）。 MILD: 近 10 年加权，适合日常查文献；STRONG: 近 3 年加权，适合跟踪 研究方向 / 追最新进展。与 impact_boost / language_affinity 可叠加 （均为乘法因子）。boost 生效时为浅翻页：不产 next_cursor、不支持 cursor 深翻页。
- `impact_boost`（string，可选，默认 NONE）：模糊搜索影响力加权：高被引文献在保留相关性的前提下上浮（仅 query 非空时生效；传排序时被忽略）。MILD: 轻度上浮，相关性仍主导； STRONG: 明显偏向高被引。引用因子有界、零被引中性（不会归零）。 与 freshness_boost / language_affinity 可叠加；boost 生效时为浅翻页。
- `journals`（array，可选）：期刊名（任一命中即可）。SDK 内部映射到后端 `publication_venue_name_unified` 字段（FILTER_OP_IN，规范化后的载体名）。
- `language_affinity`（string，可选，默认 NONE）：模糊搜索语言亲和加权：非 query 语言的结果降序、但不排除（仅 query 非空时生效；传排序时被忽略）。目标语言由服务端从 query 文本判定 （假名→ja / 谚文→ko / 汉字→zh / 拉丁→en，其他书写系统不生效）； 语言未知的文献保持中性不降权。MILD: 非目标语言 ×0.5，跨语言强相关 结果仍可上浮；STRONG: ×0.2，几乎只看目标语言。与 freshness_boost / impact_boost 可叠加；boost 生效时为浅翻页。要硬排除某语言请改用 filters_advanced 的 language 字段（如 {"field":"language","value":"en"}， 软硬两层语义不同：本参数只调序，filter 直接排除）。
- `page`（integer，可选，默认 1）
- `page_size`（integer，可选，默认 25）：每页条数。省略时为服务端默认 25（SDK / MCP 不注入默认值）；工具层上限 50。
- `query`（string，可选）：BM25 full-text keywords over title/abstract/venue/keywords; leave empty for pure structured filtering. Plain keywords match loosely (any word, ranked by relevance).  Boolean syntax is also supported: UPPERCASE AND / OR / NOT, ( ) grouping and "quoted phrases"; precedence NOT > AND > OR, adjacent words are implicitly AND. Example:   (histopathology OR pathology) AND ("deep learning" OR "machine learning") AND (prognosis OR survival) In boolean mode every term is a hard requirement and nothing is relaxed — 0 hits means 0. - Put only search terms in query: labels or instructions such as   "检索式1（预后预测）" become required terms too. - Lowercase and/or are ordinary words; quote "OR" to search the literal   (e.g. odds ratio). - Quotes only take effect alongside an operator: a query that is just   "spread through air spaces" is treated as plain keywords, whereas   "spread through air spaces" AND lung is an exact phrase match. - Max 64 terms and 10 nesting levels (400 otherwise); split complex searches   into several calls.
- `sort_advanced`（array，可选）：高级排序逃生舱（按任意可排序字段）。papers 用 sort_by_year 即可； authors/sources 想按 h-index / 被引 / works_count 排序时用本字段。 与 query 互斥（query 走相关性排序）。
- `sort_by_year`（string，可选，默认 auto）：按发表年份排序。默认 auto：传了 query（或 sort_advanced）时不加年份排序 ——保留 BM25 相关性排序，且 freshness/impact/language_affinity 软加权可用； 纯结构化筛选（无 query）时按年份降序（否则后端默认序是 unique_id，实质乱序）。 ⚠️ 不要用 query + desc 求「最相关且最新」：显式排序会让 query 退化为命中 过滤（OR 语义、无相关性排序）、三个软加权全部失效——返回的是「含任一关键词 的最新文档」。要「相关且偏新」请用 freshness_boost。
- `subjects`（array，可选）：学科分类，如 "computer science"、"biology"。
- `title_contains`（string，可选）：标题中必须包含的词（仅匹配 title 字段）。
- `year_from`（integer，可选）：起始发表年（含）。
- `year_to`（integer，可选）：结束发表年（含）。

### semantic_search
Natural-language semantic search returning relevant paper chunks for
RAG-style answering.
Use when: "How does Transformer attention work?", "What are recent
methods for protein structure prediction?".
Not for: precise field filtering (use search_papers) or fetching full
original text (use read_content).
Returns: list of chunks; each entry has chunk_id, doc_id, abstract,
chunk, score, title, offset.
Typical chain: semantic_search → pick chunk → read_content(doc_id,
offset).
参数：
- `filters`（object，可选）：结构化过滤（可选）。在召回阶段与语义检索同时生效（ES+Milvus 双引擎下推， 不是结果后过滤）；多个字段之间 AND，同一字段传数组时数组内 OR。 ⚠️ 宽松（软）语义：chunk 侧元数据缺失的文档不会被排除——例如按年份过滤时， 缺年份信息的 chunk 仍可能返回。需要严格范围保证时勿当硬约束使用， 表述结论时注明范围为"近似过滤"。 数值/日期字段支持区间：{"gte":2020,"lte":2025} 或 [min,max]（null 表示一侧不限）； 日期接受 YYYY / YYYY-MM / YYYY-MM-DD。 实际可用字段受账号字段权限约束；未知字段服务端返回 400。 例：{"author":["Hinton"],"publication_published_year":{"gte":2023},     "topics":{"dimensions":{"primary_topic_domain":"Health Sciences"}}}
- `mode`（string，可选，默认 balanced）：fast = 仅关键词召回 (~200ms)；balanced = 混合检索 (~600ms)；quality = LLM 改写 + 混合 (~2-4s)。
- `query`（string，必填）：自然语言查询，1-200 字最佳。
- `source_types`（array，可选）
- `top_k`（integer，可选，默认 10）：返回命中条数上限，合法 1-100（服务端校验，超出报 400）。 实际条数还受 mode 影响：balanced 单路混合召回在服务端固定截到约 50 条， top_k 超过 50 时多出的部分不会返回；fast 与 quality 可取到 top_k。 另外同一篇论文最多返回约 3 个 chunk，因此高 top_k 需要命中足够多的不同论文。

### list_catalog
Returns the schema catalog for search_papers: every field name, type,
whether it's filterable / sortable, default-return status, human
description, and applicable FilterOperators.
Use when: "Which field do I filter by DOI?", "What values can
access_oa_status take?", "What's the right enum for metadata_type?".
Not for: actually searching papers (use search_papers / semantic_search).
Typical pattern: call once when first encountering Sciverse or facing
an ambiguous field need, then construct precise search_papers filters
from the returned schema.
Pass include_sample_values=true to also fetch top-20 values for
enum-like fields (OpenSearch terms aggregation, 24h cached).
参数：
- `collection`（string，可选，默认 papers）：字段 catalog 所属实体集合。papers（默认）/ authors / sources，各 collection 字段不同。
- `include_field_stats`（boolean，可选，默认 False）：是否返回字段统计（keyword 字段基数 + 数值字段 min/max/avg/p50/p95）。触发 OpenSearch 聚合，缓存 24h。
- `include_sample_values`（boolean，可选，默认 False）：是否拉取 enum-like 字段的取值样本。false 仅返回静态 schema（毫秒级）；true 触发 OpenSearch terms agg（首次几百毫秒，之后 24h 走缓存）。

### read_content
Read a range of a paper's original text addressed in Unicode code
points (offset/limit count characters like Python len(), not bytes).
Typically used with a doc_id/offset returned by semantic_search to
expand context (read more text before or after a chunk).
Returns: text fragment, bytes_returned (UTF-8 byte length of text, for
reference only), next_offset (code-point offset of the next fragment —
page with it, never with bytes_returned), more (boolean).
Server behaviour: limit above 524288 is silently clamped; omitting
offset returns the whole document ignoring limit — the SDKs / MCP
server send offset=0 and limit=4096 by default, so pass offset
explicitly when calling the HTTP API directly.
参数：
- `doc_id`（string，必填）：文献 ID（来自 search_papers / semantic_search）。
- `limit`（integer，可选，默认 4096）：最多返回的 Unicode 码点数。默认 4096；服务端上限 524288（超出静默钳制）。LLM 场景建议 ≤ 16384，避免撑爆上下文。
- `offset`（integer，可选，默认 0）：起始 Unicode 码点偏移，直接使用 semantic_search 返回的 offset。默认 0。

### get_resource
Returns the binary bytes of a paper figure / table image referenced
inside read_content's Markdown via `![alt](file_name)` placeholders.
Use when the user asks to see / display / describe a figure and
read_content output contains an image reference.
Input file_name comes from the Markdown URL part (relative path,
no `\\` or `..`).
Returns: raw image stream + image/* Content-Type. The SDK / MCP
server wraps the bytes as base64 + mimeType so Claude (multimodal)
can read the image directly.
参数：
- `file_name`（string，必填）：图片相对路径，来自 read_content Markdown 中的 `![alt](file_name)` 占位。禁止 `\\` 与 `..`，不能以 `/` 开头。