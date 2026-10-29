 # 智能导入详解

 > **定位**：从产品与全链路视角理解 Smart Import 的多阶段处理
 > **前置**：[算法总览](../03-algorithms/README.md) §1

 ---

 ## 1. 链接口：POST /api/v1/ugc/smart-import

 ### 请求

 ```json
 {
   "raw_text": "InternVL的 Q 技能叫斩钢闪...",
   "attachments": [{"url": "...", "mime_type": "image/png"}],
   "kb_type": "edu_tutorial",
   "use_llm": true,
   "split": true,
   "submit": false
 }
 ```

 | 字段 | 说明 |
|------|------|
| `raw_text` | 原始正文 |
| `attachments` | 附件列表 |
| `use_llm` | 是否启用 LLM 标题/拆分/分析 |
| `split` | 是否拆分多段内容 |
| `submit` | false → 仅预览（不实际入库） |

 ### 响应（submit=false，拆分模式）

 ```json
 {
   "items": [
     {"title": "InternVL Q 技能", "content": "...", "line_range": [0, 5], "confidence": 0.82},
     {"title": "InternVL W 技能", "content": "...", "line_range": [6, 10], "confidence": 0.78}
   ],
   "confidence": 0.80,
   "import_allowed": true
 }
 ```

 **422 仍带预览**：置信度不足时 HTTP 422，但 response 仍含 preview。前端可展示「为什么不能提交」。

 ---

 ## 2. 多阶段流水线

 ```
 raw_text + attachments
   │
   ├── 1. 多模态预处理
   │     ├─ VL/OCR 识图
   │     ├─ MD5/pHash 去重缓存
   │     └─ 图片解析结果 → 融入正文
   │
   ├── 2. LLM 标题推断
   │     ├─ InferTitleLLM → 标题
   │     └─ title_validate → 拒绝不合规标题
   │
   ├── 3. 拆分（可选）
   │     ├─ 规则拆分 → segments[]
   │     ├─ LLM 拆分 → 补充
   │     └─ 每段含 line_range 定位
   │
   ├── 4. 分析
   │     ├─ 规则分析 → kb_type
   │     └─ LLM 纠错（可选）
   │
   ├── 5. 去重
   │     ├─ content_hash 查重
   │     └─ target_doc_id 强制更新
   │
   └── 6. 置信度五检
         ├─ 结构（至少 1 个 h1–h4）
         ├─ 实体密度（≥ 0.06）
         ├─ 噪声（bigram 重复率 ≤ 0.5）
         ├─ 标题（子串/编辑距离≤3/LCS≥0.70）
         ├─ 实体溯源（≥50% 可追溯）
         └─ → Pass / Warning / Reject
 ```

 ---

 ## 3. 多模态处理

 代码：`internal/core/service/multimodal/`

 ### 图片去重

 ```
 导入图片 → 计算 MD5 + pHash
   → MD5 命中 → 完全相同 → 复用缓存结果
   → pHash 汉明距离 ≤ 10 → 视觉相似 → 三级校验链（颜色 + SSIM）
   → 都不命中 → 触发 VL/OCR 识图 → 结果写入缓存
 ```

 ### VL Plus 分流

 根据图片类型选择识别策略：

| 图片类型 | 识别方式 |
|----------|----------|
| 截图/UI | OCR 文本提取 |
| 照片/插画 | VL 模型描述 |
| 图表 | 结构化提取 |

 ---

 ## 4. 标题校验规则

| 校验 | 拒绝 |
|------|------|
| 纯 JSON | `{"title": "xxx"}` |
| 导入意图句 | "帮我把这段导入知识库" |
| 过短 | < 2 字 |
| 过长 | > 100 字 |
| 纯特殊字符 | "======" |

 ---

 ## 5. 学习练习

 1. 用 `submit=false` 测试同一段 raw_text 在 `use_llm=true` 和 `use_llm=false` 下的差异。
 2. 构造一篇「噪声过高」的文章（大量重复的 "测试测试测试..."），观察置信度五检的结果。
 3. 导入同一张图片两次，第二次导入是否触发了缓存命中？

 ---

 ## 6. 相关文档

 - [自研算法深度学习 §5](../03-algorithms/02-custom-algorithms.md)
 - [smart-import.md](../../features/smart-import.md)
 - [smart-import.md](../../api/smart-import.md)
 - [multimodal.md](../../features/multimodal.md)
