 # Loop 机制 (Agent Loop): Agent 的"发动机"

 如果说记忆是"经验库"，工具是"双手"，那么 Agent Loop 就是驱动整个系统运行的"发动机"和"决策大脑"。它让 Agent 从"被动响应"变为"主动求解"。

 下面从底层原理、工程落地挑战和高级演进形态三个维度展开。

 

 ## 1. ReAct 核心范式深度剖析: 不止是"三步走"

 ReAct (Reasoning + Acting) 看似是简单的"思考-行动-观察"循环，其精妙之处在于协同效应 (Synergy):

 - **思考 (Reasoning)**: 并不只是"决定做什么"，它还承担着任务拆解 (Task Decomposition) 和进度追踪的重任。LLM 在此阶段生成的"内心独白" (Chain-of-Thought) 实际上是在维护一张动态的待办清单，它将复杂目标 (如"帮我订一张去北京的机票") 拆解为"查航班-选座位-填信息-支付"等子目标。
 - **行动 (Acting)**: 是连接虚拟思维与物理/数字世界的桥梁，它的结果 (Observation) 通常偏离 LLM 的预期。这种预期与现实的偏差正是 Agent 智能的来源。例如，查询数据库返回空结果，会迫使 Agent 在下一轮思考中修正查询条件。
 - **观察 (Observing)**: 不仅是接收数据，更是状态更新 (State Update)。观察结果会覆盖或修正工作记忆中的某些假设，让 Agent 时刻感知环境变化。

 

 ## 2. 工作流程的工程化拆解: 从理想伪代码到现实代码

 在实际的高并发生产环境中，这个循环会被细化为四个物理步骤: 解析-规划-执行-校验。

 | 理想步骤 | 现实工程实现 | 关键难点 |
 | :--- | :--- | :--- |
 | 感知 (Perceive) | 上下文组装与压缩: 不仅要加载对话历史，还要根据 Token 预算对长期记忆进行 RAG 检索，并对过长的历史进行递归摘要 (Recursive Summarization)，防止上下文溢出。 | 如何在有限的上下文窗口 (如 128K) 中，保留最重要的决策链路，丢弃无效噪音。 |
 | 决策 (Decide) | 结构化输出约束: LLM 输出的不是纯文本，而是必须遵循特定 JSON Schema 的指令。工程上会使用 JSON Mode 或约束解码 (Constrained Decoding，如 Outlines 库)，确保 LLM 100% 输出合法的工具调用参数。 | 防止 LLM 幻觉出不存在的方法名，或参数类型错误 (如字符串传给整数类型)。 |
 | 行动 (Act) | 并行与依赖解析: 如果 LLM 决定调用多个工具，分发器 (Dispatcher) 会构建一个依赖图 (Dependency Graph)。对于互不依赖的工具 (如查天气和查新闻)，采用异步并发 (Async/Await)；对于有依赖的工具 (先查航班号，再查该航班的延误情况)，则串行执行。 | 协调异步任务的超时与取消 (Cancellation)，避免僵尸进程占用资源。 |
 | 反馈与记忆更新 | 记忆的写入触发器: 并非所有观察结果都值得存入长期记忆。工程上通过重要性评分 (如信息熵变化值) 来判断，只有高分值的信息才会触发向量数据库的写入操作。 | 频繁写入向量库会导致 IO 瓶颈，需设计批量写入缓冲池。 |

 

 ## 3. 关键要素与解决方案

 ### 3.1 停止条件 (Stop Condition) 的复杂性

 除了"模型给出最终答案"和"达到最大迭代次数"，现实系统中还必须包含:

 - **预算终止 (Budget Termination)**: 当 Token 消耗或 API 调用费用达到预设阈值时，强制终止 Loop 并返回已生成的部分结果。
 - **死循环检测 (Loop Detection)**: 如果 Agent 连续 3 次调用同一个工具且传入相同参数，说明陷入了逻辑死锁。系统需注入干预提示词 (如"你似乎卡住了，请尝试换个思路")，若仍无解则强制停止。
 - **无效操作终止**: LLM 决定调用工具但分发器找不到对应方法，或参数校验失败，连续发生 2 次即终止，避免无意义消耗。

 ### 3.2 上下文组装 (Context Assembly)

 这是工程上最难拿捏的部分，直接决定成败。

 - **滑动窗口 (Sliding Window)**: 为保证快速响应，通常只将最近 5-10 轮对话作为必选项。
 - **核心链路注入 (Critical Path Injection)**: 必须将系统提示词 (System Prompt)、当前任务目标 (Objective) 和上一步行动结果 (Last Observation) 强制固定在上下文顶部，绝对不能被滚动窗口挤出。
 - **记忆检索降级**: 如果长期记忆检索耗时超过 200ms，系统会走缓存 (Cache) 降级方案，返回近期热门结果，以保证 Loop 的 TTFT (首 Token 时间) 不超标。

 ### 3.3 控制权 (Control) 的正反两面

 Workflow (工作流) 是预定义的铁轨，数据只能按既定路线前进；Agent Loop 则是无人驾驶汽车，路径由 LLM 实时感知路况决定。

 - **优点**: 极高的不确定性适应能力。当用户中途改变需求 (如"算了，改成高铁吧")，Loop 可以动态调整后续所有步骤。
 - **致命弱点**: 不可解释性 (Black-box) 和不确定性。Workflow 哪怕出错，工程师也能快速定位到第 3 步；而 Agent Loop 出错 (比如删错了数据)，回溯原因极其困难。
 - **工程折衷**: 如今的高级 Agent 架构 (如 Plan-Execute 模式) 往往采用双循环机制: 外循环负责生成一个高层次的静态计划 (Plan)，内循环 (ReAct) 只负责执行当前计划步骤并纠偏。这既保留了灵活性，又增加了可控性。

 

 ## 4. 超越基础 ReAct: Agent Loop 的高级演进

 为了让 Loop 更智能，业界已发展出多种变体:

 - **Reflexion (反思机制)**: 在 Loop 中增加一个评价者 (Evaluator)。Agent 执行行动后，不直接进入下一轮，而是先让另一个 LLM 评判刚才的操作结果是好是坏。若结果不佳，生成一条反思摘要存入记忆，引导下一轮行动修正错误。
 - **Plan-and-Solve (计划与求解)**: Loop 启动时，先强制 LLM 花费 1-2 轮生成完整的步骤清单，再进入执行阶段。这解决了 ReAct 容易只见树木不见森林 (短视) 的问题。
 - **LLMCompiler (编译器模式)**: LLM 在一次推理中生成一个 DAG (有向无环图)，描述所有工具调用的依赖关系，调度器据此并行执行所有独立任务。这打破了 ReAct 必须步步串行的性能瓶颈，将多步骤任务的耗时缩短数倍。

 

 ## 5. 技术详解: 落地实现路径

 理论讲完了，Loop 到底怎么落地? 下面用通俗的语言介绍几种主流实现方式。

 ### 5.1 框架级的 Loop 实现

 大部分团队不会从零手写 Loop，而是直接使用成熟框架:

 - **LangChain AgentExecutor**: 最广为人知的实现。内部就是一个 `while` 循环，每轮调用 LLM、解析输出、执行工具、将结果喂回 LLM，直到 LLM 返回 `AgentFinish` 信号或达到 `max_iterations`。核心逻辑在 `AgentExecutor._take_next_step()` 方法中，开发者只需配置 Agent 类型 (ReAct / OpenAI Functions / Structured Chat) 和工具列表即可运行。
 - **OpenAI Assistants API**: OpenAI 把 Loop 做成了托管服务。你只需创建 Assistant、开启 Thread、提交 Message，服务端自动在后台跑完整个 ReAct 循环，最终返回 Run 结果。开发者可以通过 `Run Lifecycle` 事件流 (queued -> in_progress -> requires_action -> completed) 来观测 Loop 状态。
 - **LangGraph**: 把 Loop 建模为状态图 (StateGraph)。每个节点是一个处理步骤 (LLM 推理 / 工具执行)，边是条件路由 (如"是否需要调用工具?" -> 是则进入工具节点，否则进入输出节点)。相比 while 循环，图的优势在于可视化、可检查和可中断后恢复。

 ### 5.2 自建 Loop 的核心要点

 如果选择自建，有几个关键决策:

 - **异步协程 vs 同步循环**: Python 中推荐 `asyncio` 异步模型，因为 LLM 调用和工具执行都是 IO 密集型。Go 语言中天然适合 goroutine + channel 模式，用一个 select 多路复用 LLM 响应、工具执行结果和超时信号。
 - **结构化输出的保障**: 让 LLM 稳定输出可解析的 JSON 是 Loop 的生命线。可以用 OpenAI 的 `response_format: json_schema` 参数、Anthropic 的 `tool_use` 内置类型，或使用 Outlines / Guidance 这类开源库在 token 级别约束生成，保证输出 100% 符合预期 Schema。
 - **状态快照与恢复**: 生产环境 Loop 可能运行几分钟甚至更长。需要将每一步的 LLM 响应、工具调用结果持久化到硬盘 (PostgreSQL 或 Redis)，以便进程崩溃后可以从断点恢复，而不是从头再来。

 ### 5.3 推荐技术栈

 | 场景 | 推荐方案 |
 | :--- | :--- |
 | 快速原型 | LangChain AgentExecutor + OpenAI Function Calling |
 | 复杂多步任务 | LangGraph StateGraph + Pydantic 结构化输出 |
 | 全托管零运维 | OpenAI Assistants API |
 | 高性能自建 | Go + goroutine + channel + JSON Schema 约束 |


 ## 6. 总结: Loop 的灵魂三问

 在任何 Agent 系统的调试中，开发者都需要问自己三个问题，这正是 Loop 机制的核心:

 1. 它是否知道何时该停? (停止条件与自我认知)
 2. 它是否记得刚才说了什么? (上下文组装与记忆)
 3. 如果它做错了，能否自行纠正? (错误恢复与 ReAct 的闭环能力)

 理解 Agent Loop，就理解了 AI Agent 从"玩具"走向"工具"的底层逻辑。

 ## 7. GrayDraft 落地对照

 | 概念 | GrayDraft 实现 | 代码 |
 |------|--------------|------|
 | Loop 内核 | Planner → Tool/Final 决策，预算与无进展检测 | `service/agent/runtime.go` |
 | 异步执行 | JetStream 队列 + Run Actor | `service/agent/launcher.go` |
 | 状态快照 | append-only `agent_run_events`，`after_seq` 回放 | `repository/postgres/agent_run_repo.go` |
 | 结构化输出 | LLMPlanner JSON Schema（final / tool_calls / clarify） | `service/agent/planner.go` |
 | 教育工具 | `knowledge.lexical`、`knowledge.semantic`、`knowledge.read`、`knowledge.expand`、`note.draft` | `service/agent/edu_tools.go` · `planner.go` |

 详见 [Agent v2 对话与 SSE](../02-pipeline/03-agent-v2-sse.md) · [agent-v2.md](../../api/agent-v2.md)。
