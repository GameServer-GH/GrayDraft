 # AI Agent 核心机制全景图

 本文档以 Mermaid 流程图将 Loop、工具调用、记忆、关键模块四者的关系与内部流程可视化。


 ## 1. 总览: 四大机制关系图

 Loop 是发动机，驱动机器运转；工具调用是双手，操作外部世界；记忆是经验库，存下所见所闻；关键模块是辅助器官，支撑系统稳定运行。

 ```mermaid
 flowchart LR
     User["用户请求"] --> Loop
 
     subgraph Core["Agent 核心"]
         Loop["Loop 循环
         (发动机)"]
         Tools["工具调用
         (双手与感官)"]
         Memory["记忆系统
         (经验库)"]
     end
 
     subgraph Support["辅助系统"]
         Planner["规划模块"]
         Guardrails["安全护栏"]
         State["状态管理"]
         Orchestration["编排协调"]
         HITL["人机协同"]
     end
 
     Loop -->|"决策输出"| Tools
     Tools -->|"执行结果"| Loop
     Loop -->|"读取上下文"| Memory
     Loop -->|"写入经验"| Memory
     Tools -->|"记录调用"| Memory
     Memory -->|"检索结果"| Loop
 
     Loop -.->|"拆解目标"| Planner
     Tools -.->|"安全检查"| Guardrails
     Loop -.->|"持久化进度"| State
     Orchestration -.->|"调度"| Loop
     HITL -.->|"审批中断"| Loop
 
     Loop --> Response["最终回答"]
 ```


 ## 2. Agent Loop 执行流程 (ReAct 循环)

 ```mermaid
 flowchart TD
     Start["接收用户请求"] --> Perceive["感知: 上下文组装"]
     Perceive --> Decide["决策: LLM 推理"]
 
     Decide --> Check{"需要调用
     工具?"}
 
     Check -->|"是"| Act["行动: 执行工具调用"]
     Act --> Observe["观察: 接收工具结果"]
     Observe --> Update["更新记忆"]
     Update --> Guard{"停止条件
     检查"}
 
     Guard -->|"达到最大迭代"| Final["生成最终回答"]
     Guard -->|"死循环检测"| Abort["强制终止 + 降级"]
     Guard -->|"预算耗尽"| Partial["返回部分结果"]
     Guard -->|"继续"| Perceive
 
     Check -->|"否"| Final
 ```


 ## 3. 工具调用: 五阶段流水线

 ```mermaid
 sequenceDiagram
     participant LLM as LLM 推理引擎
     participant Framework as Agent 框架
     participant Dispatcher as 分发器
     participant Sandbox as 沙箱环境
     participant Tool as 外部工具/API
 
     Note over LLM: 阶段一: 工具定义 (开发时)
     Framework->>Framework: 注册工具 Schema (JSON Schema / Pydantic)
 
     Note over LLM: 阶段二: Agent 决策
     LLM->>Framework: 输出 tool_calls JSON
     Framework->>Dispatcher: 解析工具名和参数
 
     Note over Dispatcher: 阶段三: 分发与校验
     Dispatcher->>Dispatcher: 参数类型校验
     alt 校验失败
         Dispatcher-->>LLM: 返回错误信息，请求修正
     else 校验通过
         Dispatcher->>Sandbox: 在沙箱中执行工具
         Sandbox->>Tool: 调用外部 API / 本地函数
         Tool-->>Sandbox: 返回结果
         Sandbox-->>Dispatcher: 返回结果 (截断/摘要)
 
         Note over Framework: 阶段四: 返回结果
         Dispatcher-->>Framework: 封装为 Observation
 
         Note over Framework: 阶段五: 闭环
         Framework->>LLM: 注入 Observation 到上下文
         LLM->>LLM: 基于新信息继续推理
     end
 ```


 ## 4. 记忆系统: 四层架构与读写循环

 ```mermaid
 flowchart TB
     subgraph Write["写入流程"]
         Raw["原始对话/事件"] --> Judge["价值判断"]
         Judge -->|"高价值"| Extract["LLM 摘要提炼"]
         Extract --> Store["写入存储层"]
         Judge -->|"低价值"| Discard["丢弃"]
     end
 
     subgraph Storage["存储层"]
         WM["工作记忆
         上下文窗口 / 缓存
         临时 | 单次会话"]
         EM["情景记忆
         向量数据库 / 外部存储
         持久化 | 跨会话"]
         SM["语义记忆
         知识图谱 / 向量库
         持久化 | 跨会话"]
         PM["程序记忆
         模型权重 / System Prompt
         稳定 | 长期"]
     end
 
     subgraph Read["读取流程"]
         Query["当前任务触发"] --> Hybrid["混合检索"]
         Hybrid --> Vec["向量检索 (语义)"]
         Hybrid --> KW["关键词 BM25 (精确)"]
         Hybrid --> Graph["图遍历 (关系)"]
         Hybrid --> Time["时序筛选"]
         Vec & KW & Graph & Time --> Merge["RRF 融合排序"]
         Merge --> Inject["注入 LLM 上下文"]
     end
 
     subgraph Manage["管理流程"]
         Cons["整合去重"] --> Upd["更新"]
         Upd --> Forget["遗忘/衰减/归档"]
     end
 
     Store --> Storage
     Storage --> Read
     Manage -.-> Storage
 ```


 ## 5. 关键模块协同工作全景

 ```mermaid
 flowchart TD
     User["用户输入"] --> GuardIn["输入护栏
     屏蔽恶意提示"]
 
     GuardIn --> Planner["规划模块
     拆解大目标为步骤清单"]
 
     Planner --> LoopEntry["进入 Agent Loop"]
 
     subgraph LoopBox["Agent Loop 内部"]
         direction TB
         CTX["上下文组装
         上下文工程负责精准检索"]
         LLM["LLM 推理决策"]
         Dispatch["工具分发"]
         Exec["沙箱执行"]
         Obs["观察结果"]
 
         CTX --> LLM
         LLM -->|"tool_calls"| Dispatch
         Dispatch -->|"安全检查"| GuardOut["输出护栏"]
         GuardOut -->|"放行"| Exec
         GuardOut -->|"拦截"| Reject["拒绝执行 + 终止"]
         Exec --> Obs
         Obs -->|"状态更新"| State["状态管理
         持久化进度"]
         Obs -->|"写入记忆"| MemWrite["记忆写入"]
         State & MemWrite --> CTX
     end
 
     LoopEntry --> CTX
 
     LoopBox --> HITL{"需要人类
     决策?"}
 
     HITL -->|"是"| Human["人机协同
     暂停等待审批"]
     Human -->|"通过"| LoopBox
     Human -->|"拒绝"| Abort["终止任务"]
 
     HITL -->|"否"| Orchestration{"需要其他
     Agent 协作?"}
 
     Orchestration -->|"是"| SubAgent["编排协调层
     调度子 Agent"]
     SubAgent --> LoopBox
 
     Orchestration -->|"否"| CheckStop{"停止条件?"}
     CheckStop -->|"继续"| LoopBox
     CheckStop -->|"完成"| Output["输出最终回答"]
 ```


 ## 6. 技术栈总览

 ```mermaid
 flowchart LR
     subgraph Lang["LangChain 生态"]
         LC["AgentExecutor"]
         LG["LangGraph"]
         LS["LangSmith"]
     end
 
     subgraph API["LLM API"]
         OAI["OpenAI Assistants"]
         Anth["Anthropic Claude"]
     end
 
     subgraph Tools["工具 / 协议"]
         MCP["MCP 协议"]
         PyD["Pydantic"]
         JSON["JSON Schema"]
     end
 
     subgraph Memory["记忆 / 存储"]
         PG["pgvector"]
         Mil["Milvus"]
         Chr["Chroma"]
         Neo["Neo4j"]
     end
 
     subgraph Sandbox["安全 / 沙箱"]
         Docker["Docker"]
         Wasm["Wasm"]
         NS["NeMo Guardrails"]
     end
 
     subgraph Orchestration["编排 / 协同"]
         AG["AutoGen"]
         CA["CrewAI"]
         TM["Temporal"]
     end
 
     Lang --> API
     API --> Tools
     Tools --> Sandbox
     Tools --> Memory
     Lang --> Orchestration
 ```
