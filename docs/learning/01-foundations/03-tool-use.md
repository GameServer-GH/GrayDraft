 # 工具调用 (Tool Use): Agent 的"双手"

 如果说记忆是 Agent 的"大脑"，那么工具调用就是 Agent 的"双手"和"感官"，让它不仅能思考，更能行动和感知外部世界。

 下面从工作流程、关键实现和进阶挑战三个维度展开。

 

 ## 1. 工作流程: 从意图到执行的完整链路

 工具调用的本质是将 LLM 的逻辑意图转化为可执行的代码操作。一个标准流程在工程上被拆分为更精细的 5 个阶段:

 ### 阶段一: 工具定义 -- 为 LLM 编写操作手册
 
 开发者使用 JSON Schema 或 Pydantic (Python 数据验证库) 模型来定义工具。关键要素:
 
 - 不仅包含函数名和参数，还必须包含清晰的功能描述和参数语义描述。例如，参数 `city` 的描述如果是"用户想要查询天气的城市名称，必须是中文"，能极大提高 LLM 填参的准确率。
 - 通过 `required` 字段标定必填参数，利用 `enum` (枚举) 约束可选值 (如天气单位只能是 `celsius` 或 `fahrenheit`)。

 ### 阶段二: Agent 决策 -- LLM 的内心独白
 
 在 Loop 的推理阶段，LLM 接收用户指令和工具 Schema 后，会进行零样本推理 (Zero-shot Reasoning)。它并不会直接执行代码，而是输出一个结构化的 JSON 对象。此时 LLM 实际上处于暂停生成文本的状态，现代的 Agent 框架 (如 LangChain、OpenAI Assistants API) 通过特殊的 `tool_calls` 字段捕获这个 JSON，而不会将其当作最终回答输出给用户。

 ### 阶段三: 执行工具 -- 框架层面的系统中断
 
 Agent 框架在解析到 `tool_calls` 指令后，会中断正常的文本生成流，转而执行本地函数或发起 HTTP 请求。需要处理同步与异步: 对于耗时操作 (如调用第三方 API)，通常采用 `async/await` 异步执行，避免阻塞整个 Agent 循环。同时必须设置超时机制 (Timeout)，防止工具卡死导致整个 Agent 崩溃。

 ### 阶段四: 返回结果 -- 注入观察值
 
 工具执行完毕后，返回值 (无论是成功的文本、JSON 数据，还是抛出的异常信息) 都会被封装起来。为了防止返回的 Token 过长撑爆上下文，框架通常会对超大返回结果进行截断 (Truncation) 或摘要 (Summarization)，然后再传递给 LLM。

 ### 阶段五: 结果处理 -- 闭环连接
 
 工具结果作为新的观察 (Observation) 被追加到记忆 (上下文) 中。此时，Agent Loop 进入下一轮迭代，LLM 会基于这个观察决定下一步是调用新工具，还是整理最终答案返回给用户。

 

 ## 2. 关键实现: 架构设计的五大支柱

 一个健壮的工具调用系统，光有流程还不够，必须有扎实的底层架构支撑。以下是四大支柱:

 ### 2.1 分发器 (Dispatcher) -- 智能路由中枢
 
 分发器不仅仅是简单的 `if-else` 或 `Map<String, Function>`，它解决了"如何将 LLM 生成的文本指令精确映射到内存中的函数"这一核心难题。

 - **动态注册机制**: 现代框架采用装饰器模式 (Decorator Pattern)。开发者只需在函数上方加一行 `@tool`，系统便会自动提取函数签名、参数类型和文档字符串，生成 JSON Schema 并加入路由表。这使得添加新工具无需修改主循环的一行代码。
 - **参数解析与校验**: 分发器收到 LLM 传来的 JSON 参数后，会利用 Pydantic 等库进行类型强制转换 (例如将字符串 `"123"` 转为整数 `123`) 和合法性校验 (如检查必填字段)。校验失败时，会生成一个明确的错误信息返回给 LLM，让其反思并修正参数。

 ### 2.2 沙箱环境 (Sandbox) -- 权限隔离带
 
 Agent 若拥有执行任意代码或系统命令的权限，极其危险。沙箱是安全性的最后一道防线，主要通过以下手段实现:

 - **进程级隔离**: 将工具执行放在独立的子进程 (Subprocess) 或 Docker 容器中运行，与主 Agent 进程隔离。
 - **资源限制**: 在沙箱内强制限制 CPU 时间 (防止死循环耗尽算力) 和内存大小 (防止 OOM 内存溢出)。例如，设定工具运行超过 10 秒即强制 kill。
 - **网络与文件系统屏蔽**: 限制工具只能访问特定的白名单 API 域名，或只能读写指定的临时文件夹，无法触碰系统核心文件。

 ### 2.3 工具选择 (Tool Selection) -- 海量工具中的检索
 
 当 Agent 拥有成百上千个工具时，不可能将所有 Schema 都塞进上下文 (会超过 Token 限制且容易产生幻觉)。此时需要引入向量检索 (RAG): 在决策前，先根据用户问题对工具库进行语义匹配，只将最相关的 Top-5 或 Top-10 个工具描述注入当前的系统提示词中，实现动态按需加载。

 ### 2.4 并行调用 (Parallel Function Calling) -- 多线程双手

 现代 LLM (如 GPT-4) 支持在一次推理中返回多个 `tool_calls`。例如，用户问"对比北京和上海的天气"，LLM 可以一次性生成两个 `get_weather` 调用指令。Agent 框架会并发发起这两个 API 请求，待全部返回后，再将结果汇总分析。这极大缩短了多步骤操作的响应时间。

 

 ## 3. 容错机制与优雅降级

 在实际落地中，工具调用远非一帆风顺，需要设计容错机制:

 1. **重试与降级**: 如果某工具调用失败 (如 API 超时)，分发器不应直接报错，而是尝试重试 (Retry) 1-2 次，或调用备用工具 (Fallback)。
 2. **无效调用修正**: 如果 LLM 生成的参数格式错误，框架会将错误信息返回给 LLM，并提示"请根据错误信息修正参数"，让 Agent 具备自我纠错 (Self-Correction) 能力。
 3. **拒绝执行**: 对于用户试图通过提示词注入诱导 Agent 调用危险工具 (如删除文件) 的行为，分发器应结合安全过滤规则，直接拒绝执行并终止循环。

 ## 4. 技术详解: 实现路径与工具链
 
 前面讲了工具调用的流程和架构，下面具体看看有哪些落地技术方案。
 
 ### 4.1 Function Calling 协议
 
 目前主流 LLM 提供商的 Function Calling 都遵循类似模式: 你传 JSON Schema 进去，模型返回一个 `tool_calls` 数组。差异在于细节:
 
 - **OpenAI**: 最早推出 Function Calling，目前已演进到支持 `parallel_tool_calls` (一次返回多个调用) 和 `strict` 模式 (强制参数类型匹配)。通过 Chat Completions API 的 `tools` 参数传入 Schema。
 - **Anthropic**: 使用 `tool_use` 块而非 JSON 模式。工具定义通过 `tools` 字段传入，模型返回的 content block 中 type 为 `tool_use`，包含 `id`、`name`、`input`。设计哲学上更强调工具是对话的一部分而非外部调用。
 - **开源模型 (Llama / Qwen 等)**: 通过 fine-tune 或提示词工程支持 Function Calling，通常采用与 OpenAI 兼容的 JSON 格式来降低迁移成本。
 
 ### 4.2 工具定义与分发
 
 - **Pydantic + @tool 装饰器**: Python 生态的主流做法。用 Pydantic BaseModel 定义参数 Schema，装饰器自动将 docstring 和类型注解转为 LLM 可用的工具描述。LangChain、LlamaIndex、CrewAI 都采用这个模式。
 - **MCP (Model Context Protocol)**: Anthropic 推出的开放协议。将工具实现为独立的 MCP Server 进程 (可运行在本地或远端)，Agent 通过 JSON-RPC 协议 `tools/list` 发现工具、`tools/call` 执行工具。好处是工具与 Agent 进程完全解耦，新增工具只需启动一个 Server，无需重启 Agent。
 - **JSON Schema 直接定义**: 最轻量的方式，适合简单场景。直接写一个 JSON 文件描述工具名称、描述和参数，不依赖任何框架。
 
 ### 4.3 沙箱与安全
 
 | 方案 | 隔离强度 | 启动速度 | 适用场景 |
 | :--- | :--- | :--- | :--- |
 | Subprocess | 低 | 极快 | 运行可信的本地脚本 |
 | Docker 容器 | 高 | 秒级 | 运行第三方代码，需要网络/文件系统隔离 |
 | Wasm 沙箱 | 高 | 毫秒级 | 运行用户上传的插件/脚本，轻量且安全 |
 | 远程沙箱 (如 Sandboxer) | 最高 | 秒级 | 执行不受信任的任意代码 |
 
 ### 4.4 推荐技术栈
 
 | 场景 | 推荐方案 |
 | :--- | :--- |
 | Python 单体 Agent | LangChain + Pydantic + OpenAI Function Calling |
 | 工具数量多 (> 50) | MCP Server + 向量检索 (RAG) 发现工具 |
 | 安全性要求高 | Docker 沙箱 + 参数白名单校验 |
 | 低延迟要求 | Wasm 沙箱 + 预编译工具二进制 |

 ## 5. 总结

 工具调用机制通过精细的流程编排与底层实现（动态分发、安全隔离、并行加速），让 Agent 从「只会说」进化为「能执行」。它与 Memory、Loop 构成 Agent 核心三角。

 ## 6. GrayDraft 落地对照

 | 概念 | GrayDraft 实现 | 代码 |
 |------|--------------|------|
 | ToolSpec | 名称白名单、风险等级 R0–R3、JSON Schema | `tool_registry` · Tool Governance |
 | 只读 R0 | `knowledge.lexical`、`knowledge.semantic`、`knowledge.read`、`knowledge.expand` | `edu_tools.go` |
 | 审批 R2 | `note.draft`，参数哈希绑定、`tool_approvals` | `tool_governance_repo.go` |
 | 沙箱执行 | Docker 只读 Runner + Egress Allowlist | `service/agent/runner.go` |
 | 审计 | `tool_invocations` 参数/输出哈希 | Governance Repo |

 详见 [agent-v2.md](../../api/agent-v2.md) · [教育场景 Agent 实现说明](../../architecture/Agent流程技术实现方案.md)。
