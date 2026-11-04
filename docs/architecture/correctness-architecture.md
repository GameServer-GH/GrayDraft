# 正确性架构与验收边界

更新于 2026-11-04。当前优先建立可重复的正确性基线，尾延迟优化暂缓。

| 环节 | 当前实现 | 代码入口 |
| --- | --- | --- |
| PDF 与导入 | Core 支持 PDF 文本层提取。导入保留原文定位，分段与质量规则区分页面标签、有效正文和失败状态。Worker 对不可重试错误保留明确终态。 | [PDF 提取](../../internal/core/service/multimodal/pdf_text.go)、[来源完整性](../../internal/core/service/smartimport/source_integrity.go)、[Worker 消费](../../internal/worker/worker/pipeline/stage_consumer.py) |
| 检索与读窗口 | 首跳按问题选择查法。跨分段定位在授权范围内展开，引用回到具体原文切片。 | [首跳](../../internal/core/service/agent/first_hop.go)、[文档组窗口](../../internal/core/repository/postgres/group_span.go) |
| 请求 | BFF 将用户问题和运行配置编码为独立字段。Core 解码后以问题驱动规划与检索，兼容旧版纯文本输入。 | [BFF](../../internal/graydraft/pkg/nexus/chat_stream.go)、[Core 输入](../../internal/core/service/agent/run_input.go) |
| 成文与门禁 | Runtime 在成文前构建有界证据包。成文与门禁消费同一份可见证据，门禁独立复算摘要，摘要不一致时终止提交。 | [证据包](../../internal/core/service/agent/answer_evidence.go)、[Runtime](../../internal/core/service/agent/runtime.go) |
| 声明复核 | 明确等价且同单位的数量级矛盾可硬拒绝。记录系数、单位、倍率、归一化有理数及阈值，支持独立复算。近似值和绑定歧义进入复核。 | [数值规则](../../internal/core/service/agent/numeric_consistency.go)、[角色复核](../../internal/core/service/agent/claim_review.go) |

角色复核区分计数对象及范围、参数名与请求头名、SNR 与检测统计量等统计量，以及假设、解释、估计与确定结论。这些规则提供有界诊断。未覆盖表示规则没有给出判断。复核证据限定在包内实际可见片段，拒绝审计保留候选摘要和必要运算依据。

## 离线纠错与上线条件

复核 Agent 决定保留、修订一次或报告证据不足。修订引文由程序按可见片段和行号提取，再由门禁和独立断言复验。必须依赖人工才能完成的环节按设计理念标记为项目技术不可触达领域。[设计理念](设计理念.md)

固定 52 题最新一次运行有 51 题正常返回，49 次门禁摘要全部一致。两条新答案通过定向离线复核。完整运行验收的是现有问答路径，自动纠错仍在离线阶段。线上旁路暂缓，待独立留出题验证误改、漏放和证据不足时的收窄行为。[52 题复跑与决策](correctness-rebaseline-shadow-decision-2026-11-04.md) · [离线纠错](offline-answer-repair-2026-11-04.md)
