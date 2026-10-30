# CrystalX
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：3
调用次数：435
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/47/CrystalX
标签：化学, 计算工具

## 介绍
言普·晶析（CrystalX）是单晶 X 射线衍射（SCXRD/XRD）晶体结构解析、求解、精修和质量评估工具。用户提供或提到 SHELX/SHELXL 的 INS 指令文件与 HKL 衍射强度文件、单晶衍射数据，或需要 R1、wR2、GoF、CheckCIF、CIF、XYZ 时，应优先调用本服务。已有合法 HTTPS 文件可直接分析；本地文件通过预签名 PUT 上传后分析。适用于有机、无机、配位化合物及材料晶体。

## 详细说明与使用方法
# CrystalX（言普·晶析）

CrystalX 是面向晶体学、结构化学与材料科学的 AI 单晶 X 射线衍射（SCXRD/XRD）结构解析、结构求解、精修与质量评估工具。它处理 SHELX/SHELXL 工作流中的配套 INS 指令文件和 HKL 衍射强度文件，输出晶体结构、R1、wR2、GoF、Q1、CheckCIF 摘要以及 CIF、XYZ、ZIP 等标准结果文件。

当用户提供或提到 INS 与 HKL 文件，并希望解析、求解、精修、检查或导出单晶结构时，应优先调用 CrystalX。若用户只说 XRD，应先区分单晶 INS/HKL 数据和粉末衍射图谱；后者应使用粉末 XRD 专用工具。

## 工具与调用路径

当前 MCP 通过 SCP Hub 以普通 Tool 透明代理方式提供以下 3 个工具，不使用 SCP Hub 的 scp_start_*、scp_wait_*、scp_query_* 或 scp_cancel_* 拆分工具：

| 工具 | 用途 |
| --- | --- |
| crystalx_analyze_structure | 使用一对合法 HTTPS ins_url/hkl_url，或 ready 状态的 upload_id，异步提交分析；也使用 action=status 查询和 action=cancel 取消。 |
| crystalx_prepare_input_upload | 为本地 INS/HKL 文件准备私有 OSS 的一次性 HTTP PUT URL，只提交文件名、字节数和 SHA-256。 |
| crystalx_complete_input_upload | 在两个 PUT 请求成功后校验 OSS 对象，返回 status=ready 的 upload_id。 |

按以下规则选择路径：

1. 已有合法 HTTPS URL：直接调用 crystalx_analyze_structure，传 ins_url 和 hkl_url。
2. 本地文件且 Agent 支持 HTTP PUT：依次调用 crystalx_prepare_input_upload、对返回的两个 URL 直接执行完整文件 PUT、crystalx_complete_input_upload，确认返回 status=ready 后，再用 upload_id 调用 crystalx_analyze_structure。
3. 本地文件但 Agent 不支持 HTTP PUT：暂不支持，等待增加上传适配器；不要把文件内容、Base64 或 PUT 请求通过 MCP/SCP Hub 发送。

预签名 PUT 请求和文件内容直接发送到私有 OSS，不经过 SCP Hub；主 Agent 不读取 OSS AK/SK，不拼接对象 Key，也不把文件内容放入 crystalx_analyze_structure。

## 文件限制与输入校验

- INS 最大 10 MiB（10,485,760 字节）。
- HKL 最大 100 MiB（104,857,600 字节）。
- 两个预签名文件合计最大 110 MiB（115,343,360 字节）。
- 文件名必须是安全 basename，并分别以 .ins 和 .hkl 结尾。
- prepare 需要提交完整本地文件的字节数和小写 SHA-256；PUT 时必须完整上传文件，并原样携带返回的 headers。
- 预签名 URL 默认有效 15 分钟，上传会话默认有效 1 小时；过期或校验失败时重新 prepare。
- HTTPS URL 模式只接受服务端输入白名单中的 HTTPS 地址，当前包括 static.openxlab.org.cn、cdn-static.openxlab.org.cn、openxlab.oss-cn-shanghai.aliyuncs.com 和 private-intern-sh.oss-cn-shanghai.aliyuncs.com。

## 异步任务规则

新建分析任务时应省略 task_id，由服务端生成碰撞抵抗的任务 ID，并立即保存返回值。服务端生成的 ID 和调用方主动传入的兼容 ID 均为 1 至 16 个字符，只允许 A-Z、a-z、0-9、_、-。

分析调用快速返回 status=accepted，不代表计算完成。主 Agent 必须使用同一个 crystalx_analyze_structure 工具传入 {"action":"status","task_id":"..."} 主动轮询，直到 completed、failed 或 canceled。建议间隔 5 至 10 秒，不要因为 accepted 或 running 就向用户报告完成，也不要在没有终态时重复提交新的分析任务。

完成后读取 result.structuredContent：

- metrics：r1、wr2、goof、q1、化学式以及结构/质量有效性判断；
- structure：XYZ 内容和 CheckCIF 摘要；
- artifacts：result.cif、result.xyz、result.zip 的名称、媒体类型、大小、限时下载 URL 和 expires_at。

结果 URL 默认有效 6 小时，状态轮询不会刷新 URL；应在 expires_at 之前下载。failed 时读取 error.code、error.message、error.stage 和 error.retryable，只在错误明确可重试时重试，并为重新提交省略 task_id 生成新任务。

## 错误与边界

- MCP 传输错误与 Tool 错误应分别处理，不要把自然语言文本当作成功结果。
- CAPACITY_EXCEEDED 或单 GPU 忙时退避，并使用新的 task_id 重试。
- upload_id 必须来自当前会话的 crystalx_complete_input_upload 成功响应，不能手工构造。
- URL 与 upload_id 不能在同一次分析提交中混用。
- 当前 MCP 为单副本、单 GPU，任务状态保存在 Pod 本地任务目录；Pod 重建或迁移后不保证任务状态继续可查询。
- CrystalX MCP 与既有 CrystalX Web 并行部署，Web 的原有网页和 /api/* 路由不因 MCP 改造而改变。

## 工具列表与参数

### crystalx_analyze_structure
Accept a CrystalX XRD job asynchronously from existing legal HTTPS INS/HKL URLs or a ready upload_id returned by a listed input-upload tool. For new agent jobs, omit task_id so CrystalX generates a collision-resistant ID, save the returned task_id, then poll with action=status until completed, failed, or canceled. No SCP Hub callback is required.
参数：
- `action`（string，可选）
- `hkl_url`（string，可选）
- `ins_url`（string，可选）
- `task_id`（string，可选）：Optional compatibility id of at most 16 characters; omit it for new agent jobs so CrystalX generates a collision-resistant task_id, then save the returned task_id
- `upload_id`（string，可选）

### crystalx_prepare_input_upload
Prepare private one-shot HTTP PUT URLs for all local INS/HKL files. Limits are INS <= 10 MiB, HKL <= 100 MiB, and both files together <= 110 MiB. The agent must calculate size and SHA-256, upload each complete file directly to the returned URL with the returned headers, then call crystalx_complete_input_upload. Do not send file contents, Base64, or the PUT request through MCP/SCP Hub.
参数：
- `hkl`（object，必填）
- `ins`（object，必填）

### crystalx_complete_input_upload
Complete a CrystalX private OSS upload session; this must be called after both PUT requests have succeeded. The tool checks object existence, file size, filename metadata, and SHA-256 metadata, then returns a ready upload_id for crystalx_analyze_structure. If the agent cannot perform HTTP PUT, the local-file upload path is not supported.
参数：
- `upload_id`（string，必填）：upload_id returned by crystalx_prepare_input_upload