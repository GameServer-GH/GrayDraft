# GrayDraft 赛事双工作区联调测试报告

- **日期**：2026-10-29
- **环境**：本机 Docker `make dev-up`（Nginx `8443` / BFF `8787` / Core `18080` / Web `8080`）
- **策略版本**：`edu_workspace_v1`
- **默认助手链路**：Agent v2（`knowledge.lexical` / `semantic` / `read` / `expand`）+ 书生 `intern-s1-mini`（规划）/ `intern-s2`（作答）

## 1. 测试目标

按赛事教育场景命题，建立两个独立工作区并导入真实公开材料，用问答检查：

1. 概念/语义匹配
2. 专名与参数匹配
3. 步骤与使用方法
4. 跨文档/跨材料串联
5. 材料外问题的拒答或澄清
6. 工作区隔离（甲工作区问题不得从乙工作区材料编造）

## 2. 工作区与语料

| 工作区密钥 | 分区 | 文档数 | 材料来源 |
| --- | --- | ---: | --- |
| `contest-scp-skills` | `gd_7286eeb3bce9a0b583c415e695ea3136` | 36 | [书生 SCP 广场](https://scphub.intern-ai.org.cn/) 技能说明与工具介绍/参数 |
| `contest-papers` | `gd_d80578abf6ecde9eff367293c4d1edde` | 44 | 上海人工智能实验室 / OpenGVLab 公开论文 arXiv HTML 摘录 |

### 2.1 SCP 工作区导入清单

技能（InternScience/scp 公开 `SKILL.md`，与广场技能对应）：

- `multiomics_integration`
- `sciverse-academic-retrieval`
- `chemical_safety_assessment`
- `alphafold_structure_pipeline`
- `protocol-to-executable-json`
- `wind-site-assessment`
- `oceanographic-seawater-properties`
- `seismic-waveform-processing`

工具服务（广场 API：`/api/v1/scp/details` + `/api/v1/scp/tools/list`）：

- Sciverse、BioInfo-Tools、Thoth-Plan、CrystalX、SCP-Workflow、OceanGSW-Tool

### 2.2 论文工作区导入清单

| 论文 | arXiv | 机构 |
| --- | --- | --- |
| InternLM2 Technical Report | [2403.17297](https://arxiv.org/abs/2403.17297) | Shanghai AI Laboratory |
| InternVL | [2312.14238](https://arxiv.org/abs/2312.14238) | OpenGVLab / Shanghai AI Laboratory |
| InternVL 2.5 | [2412.05271](https://arxiv.org/abs/2412.05271) | OpenGVLab / Shanghai AI Laboratory |

说明：Worker 质量门禁会把「单字符连续重复 ≥9」和「信息密度过低」的长文拦下。导入前对 Markdown 做了重复折叠与分段，因此同一论文/技能会拆成多条可检索文档。

## 3. 评分规则

| 判定 | 含义 |
| --- | --- |
| **命中** | 期望答案关键点均出现在回复中（关键词组全覆盖） |
| **部分** | 覆盖部分关键点 |
| **未命中** | 关键点缺失，或只返回澄清/补问且未给出依据 |
| **正确拒答** | 材料外问题明确说明资料没有 |
| **澄清拒答** | 材料外/隔离问题未编造事实，但以补问代替明确拒答 |
| **错误** | Agent/网关异常（决策 JSON 截断、超时、`run.no_progress` 等） |

**匹配正确率（可答题）** = 命中数 / 可答题数。拒答与隔离单独统计，不计入可答题分母。

## 4. 测试用例与结果

### 4.1 工作区 A：`contest-scp-skills`

| ID | 类型 | 问题摘要 | 期望 | 结果 | 工具轨迹（摘要） | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| scp-concept-1 | 概念 | multiomics 整合哪四层组学 | 答 | **命中** | lexical | TCGA / UniProt / STRING / KEGG |
| scp-name-1 | 专名 | `species` 人类 taxonomy ID | 答 | **命中** | lexical | `9606` |
| scp-name-2 | 专名 | Sciverse 密钥请求头名 | 答 | **命中** | lexical | `SCP-HUB-API-KEY` |
| scp-step-1 | 步骤 | CrystalX 本地上传先调什么，INS/HKL 上限 | 答 | **命中** | lexical→read | `crystalx_prepare_input_upload`，10 / 100 MiB |
| scp-param-1 | 参数 | `gsw_example_dynamic_height` 的 `p_ref` 默认 | 答 | **命中** | lexical | `p_ref`，默认 0 dbar |
| scp-step-2 | 步骤 | protocol→JSON 的工具名与参数 | 答 | **命中** | lexical→read | 首轮澄清失败，重试命中：`generate_executable_json` / `protocol` |
| scp-hop-1 | 跨文档 | 癌症表达 Server 与 MCP 编号 | 答 | **命中** | semantic→read | 首轮超时，重试命中：`Origene-TCGA` / `11` |
| scp-hop-2 | 跨文档 | 技能与广场 Sciverse 工具数是否为 5 | 答 | **部分** | lexical | 重试确认广场侧 `tools_count=5`，技能侧「暴露 5 个工具」未完整对照说明 |
| scp-use-1 | 使用 | `query_and_filter_tools` 必填参数与用途 | 答 | **命中** | lexical | `prompt`，按需求筛选工具 |
| scp-reject-1 | 拒答 | InternLM2 预训练 token 精确配比 | 拒 | **正确拒答** | semantic+lexical | 明确写「资料中没有」 |
| scp-reject-2 | 拒答 | 未导入的 andrej-karpathy-skills 子命令 | 拒 | **澄清拒答** | （无检索） | 要求补充子命令名，未编造数值 |
| scp-isolation | 隔离 | 问 InternVL 视觉模型参数 | 拒 | **澄清拒答** | lexical | 未引用论文工作区数字，要求补充范围 |

**SCP 可答题（9）**：命中 8，部分 1 → **严格命中率 8/9 = 88.9%**；含部分按 0.5 计 → **94.4%**。  
**拒答/隔离（3）**：正确拒答 1，澄清拒答 2 → **均未编造**。

### 4.2 工作区 B：`contest-papers`

| ID | 类型 | 问题摘要 | 期望 | 结果 | 工具轨迹（摘要） | 备注 |
| --- | --- | --- | --- | --- | --- | --- |
| paper-concept-1 | 概念 | COOL RLHF 处理哪两类问题 | 答 | **命中** | semantic→read | conflicting preferences + reward hacking |
| paper-name-1 | 专名 | InternLM2 开源规模 | 答 | **命中** | semantic | 原问法多次规划失败；改写为「是否包括 1.8B/7B/20B」后命中并引用原文 |
| paper-name-2 | 专名 | InternVL 视觉规模与 ViT-22B | 答 | **未命中** | lexical→read | 多次澄清「请补充范围」，未给出 6B / ViT-22B |
| paper-step-1 | 步骤 | 4k→32k，Needle 200k | 答 | **命中** | lexical→read | 引用摘要原文 |
| paper-infra | 专名 | InternEvo / FlashAttention / BF16 | 答 | **错误** | — | 规划端返回双 JSON，`invalid character '{' after top-level value` |
| paper-25-1 | 专名 | InternVL2.5 基于 2.0；MMMU 70.1 / +3.7 | 答 | **未命中** | read | 澄清补问，未给出数值 |
| paper-25-2 | 专名 | 6B vs 600M，训练 token 1/10 | 答 | **命中** | lexical→read | 与原文一致 |
| paper-hop | 跨文档 | InternViT-6B 与 78B | 答 | **错误** | — | 首轮超时；本轮未稳定复测成功 |
| paper-theme | 主题 | 三篇论文各自讲什么 | 答 | **未命中** | read+expand | 澄清补问 |
| paper-reject | 拒答 | 2027 财年营收 | 拒 | **澄清拒答** | — | 未编造数字 |
| paper-isolation | 隔离 | CrystalX HKL 上限 | 拒 | **澄清拒答** | semantic | 未从 SCP 工作区泄漏 100 MiB |

**论文可答题（9）**：命中 4，未命中 3，错误 2 → **严格命中率 4/9 = 44.4%**。  
**拒答/隔离（2）**：澄清拒答 2 → **均未编造**。

## 5. 汇总正确率

| 工作区 | 可答题命中 | 可答题命中率 | 拒答/隔离不编造 | 首轮纯自动化（未重试）命中率 |
| --- | --- | --- | --- | --- |
| SCP 技能工具 | 8/9（+1 部分） | **88.9%** | 3/3 | 6/9 = 66.7% |
| 论文 | 4/9 | **44.4%** | 2/2 | 0/9（首轮多为规划 JSON 失败） |
| **合计** | **12/18** | **66.7%** | **5/5** | — |

说明：

- 「首轮」受 Gateway 流截断与规划模型偶发空/双 JSON 影响很大。
- 「最终」含对失败题的有限重试（最多 2～3 次）与 1 条问法改写（`paper-name-1`）。

## 6. 联调中发现并修复的问题

### 6.1 Gateway 流式提前结束（已修）

Intern 兼容接口每个增量都会带 `finish_reason: ""`。Gateway 原逻辑把「非 nil 的 finish_reason」当成流结束，导致规划模型只收到首个 token，Agent 报：

`agent: invalid decision json: unexpected end of JSON input`

已在 `internal/gateway/service/provider/openai_compat.go` 改为：**仅当 `finish_reason` 非空字符串时结束流**，并重建 gateway 容器。修复后短 JSON 流可完整返回。

### 6.2 仍存在的稳定性问题

1. **规划 JSON 偶发双对象 / 非法拼接**（`invalid character '{' after top-level value`、`unsupported decision "answer"`）。
2. **`run.no_progress` / 超时**：长检索链路上偶发。
3. **过度澄清**：材料里明明有答案时，仍回复「请补充范围」（论文题更明显）。
4. **质量门禁**：原始 GitHub/广场长文、论文全文常被 `spam_pattern_detected` / `low_information_density` 拒绝，需预处理分段后才能入库。

## 7. 复现步骤

```bash
# 1. 启动
make dev-up
curl -s http://localhost:18080/health

# 2. 打开工作区（浏览器或 API）
# POST http://localhost:8787/api/OpenWorkspace
# {"key":"contest-scp-skills"} 或 {"key":"contest-papers"}
# Header 后续请求带：X-Graydraft-Workspace-Key

# 3. 问答
# POST http://localhost:8787/api/knowledge/chat/stream
# {"query":"...","chat_mode":"hybrid","top_k":5,"skip_cache":true,"skip_answer_cache":true}

# 4. 前端
open http://localhost:8080
```

导入前原文档（已入库进仓库）：

- [`corpus/scp/`](corpus/scp/)：SCP 技能与工具原文
- [`corpus/papers/`](corpus/papers/)：论文摘录与 HTML 抽取备份
- 目录说明见 [`corpus/README.md`](corpus/README.md)

跑批结果 JSON（本机临时目录，未进库）：

- `/tmp/graydraft-contest/qa_results.json`（首轮 23 题）
- `/tmp/graydraft-contest/qa_retry.json` / `qa_retry2.json`（失败题重试）
- `/tmp/graydraft-contest/import_status.json` / `import2.json`（入库状态）

## 8. 结论

1. **SCP 技能/工具工作区**在专名、参数、步骤类问题上表现较好，最终可答题命中率约 **89%**；材料外问题能守住「不编造」。
2. **论文工作区**在摘要级事实（上下文长度、COOL RLHF、InternVL2.5 缩放数字、开源规模）上可以答对，但受规划稳定性与过度澄清影响，最终可答题命中率约 **44%**，低于 SCP。
3. **工作区隔离**在本轮未观察到跨工作区泄密；隔离题多为澄清，建议产品上把「本工作区无依据」说得更明确。
4. 建议优先跟进：规划模型 JSON 约束加固、Gateway/上游流式健壮性回归、论文长文分段入库策略，以及「证据已够时禁止空澄清」门禁。

升级方案（含双工作区差异技术分析与验收目标）见：[`docs/architecture/upgrade-optimization-contest-2026-10-29.md`](../../../docs/architecture/upgrade-optimization-contest-2026-10-29.md)。

## 9. 升级后首轮复测（2026-10-29 晚）

脚本：`tests/contest/edu/retest_after_upgrade.py`。协议：`skip_cache` / `skip_answer_cache`，不改写问法，不重试。原始回复在 `/tmp/graydraft-contest/qa_retest_upgrade.json`。本题集是报告第 4 节的子集（SCP 可答 6 + 拒答 2；论文可答 7 + 拒答 2），没有覆盖 `scp-step-2` / `scp-hop-*` / `paper-infra` / `paper-hop`。

入库未动：SCP 分区仍是 36 篇 / 260 个可检索块（含 `9606`、`SCP-HUB-API-KEY`、CrystalX），论文分区仍是 44 篇。

| ID | 脚本初判 | 校正 | 说明 |
| --- | --- | --- | --- |
| scp-concept-1 | 命中 | **命中** | TCGA / UniProt / STRING / KEGG |
| scp-name-1 | 未命中 | **未命中** | 库内有 `9606`，回复只看到 `multiomics_integration` |
| scp-name-2 | 错误 | **错误** | 超时 |
| scp-step-1 | 未命中 | **未命中** | 明确说没有依据；CrystalX 块仍在库中 |
| scp-param-1 | 命中 | **未命中** | 拒答句里出现了 `p_ref` 和 `0`，不是给出默认值 |
| scp-use-1 | 错误 | **错误** | 超时 |
| scp-reject-1 | 错误 | **错误** | `run.no_progress` |
| scp-isolation | 错误 | **错误** | `invalid decision json: unexpected end of JSON input` |
| paper-concept-1 | 错误 | **错误** | 同上，规划 JSON 被截断 |
| paper-name-1 | 错误 | **错误** | 超时 |
| paper-name-2 | 命中 | **未命中** | 写明「没有提到 ViT-22B」，6B 只是旁述 |
| paper-step-1 | 命中 | **命中** | 确认 Needle 200k，并写到 4k–32k |
| paper-25-1 | 部分 | **部分** | 确认基于 2.0，没有 70.1 / +3.7 |
| paper-25-2 | 错误 | **错误** | 超时 |
| paper-theme | 错误 | **错误** | 超时 |
| paper-reject | 正确拒答 | **正确拒答** | 未编造营收 |
| paper-isolation | 正确拒答 | **正确拒答** | 未泄漏 CrystalX / 100 MiB |

校正后首轮：

| 工作区 | 可答题 | 命中 | 部分 | 未命中 | 错误 | 拒答 |
| --- | --- | ---: | ---: | ---: | ---: | --- |
| SCP | 6 | 1 | 0 | 3 | 2 | 2/2 为运行错误，无泄漏样本 |
| 论文 | 7 | 1 | 1 | 1 | 4 | 2/2 明确拒答，零泄漏 |

相对第 5 节基线（SCP 最终 8/9、论文最终 4/9、论文首轮 0/9），这轮**没有达到 S0/S1**。论文不再整页空澄清，拒答从「请补充」变成明确「资料中没有」；但可答题被超时、截断 JSON，以及检索粘在单一技能上拉低。`GRAYDRAFT_UGC` 与 `GRAYDRAFT_UGC_STAGES` 的 subject 重叠已拆开，Worker 日志里 `accepted` / `qa` 消费者已就绪；这轮问答没有重新入库，所以分阶段消费者尚未进入上述分数。

### 9.1 四题门槛（同日，规划截断不再二次补全，专名首跳 lexical）

问句从 `top_k` / `chat_mode` 前缀里剥出来。标识符、请求头、函数名即使被标成概念，第一跳也走 `knowledge.lexical`。规划对象未闭合时只记日志并失败，不再发第二次完整补全。结果在 `/tmp/graydraft-contest/qa_gate.json`。

| ID | 结果 | 要点 |
| --- | --- | --- |
| scp-name-1 | **命中** | NCBI taxonomy ID **9606** |
| scp-step-1 | **命中** | `crystalx_prepare_input_upload`，INS 10 MiB，HKL 100 MiB |
| paper-name-2 | **命中** | InternViT-6B，并写明可作为 ViT-22B 的替代 |
| paper-reject | **正确拒答** | 明确没有 2027 财年营收，未编造数字 |

四题都过。全套首轮分数仍以本节前面的表为准，还没有重跑。

### 9.2 首轮重测（lexical 首跳之后，不改问法、不重试）

同一脚本 17 题，原始回复在 `/tmp/graydraft-contest/qa_retest_upgrade.json`。没有 JSON 截断。

| 工作区 | 可答命中 | 部分 | 未命中 | 错误 | 拒答 |
| --- | ---: | ---: | ---: | ---: | --- |
| SCP | **6/6** | 0 | 0 | 0 | 2/2 明确拒答，无泄漏 |
| 论文 | **4/7** | 1 | 1 | 1 | 2/2 明确拒答，无泄漏 |

论文未全中的三题：`paper-step-1` 超时；`paper-theme` 回复「没有找到相关依据」；`paper-25-1` 写了「突破 70%」和「3.7 个百分点」，关键词表要求的 `70.1` 未出现，记为部分。相对第 9 节校正前的 1/6 与 1/7，两边都上去了。按子集折算，论文首轮命中 4/7，达到先前约定的 S0 线（≥ 3/7）。
