# Thoth-Plan
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：4
调用次数：4555
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/19/Thoth-Plan
标签：生命科学, 湿实验操作, 模型服务

## 介绍
一个统一的、通过强化学习微调的LLM智能体，用于科学实验流程（scientific protocol）的自动生成和执行，能够有效解决长期存在的准确性不足、逻辑结构混乱以及实验可执行性差等核心挑战。Thoth-Plan真正打通了从科研设想到实验动作的全链路，让湿实验从“人写文档、人手动操作”演进为“机器可理解、可调度、可验证的程序化实验”，成为下一代智能化自动实验室的核心执行引擎。

## 详细说明与使用方法
# Thoth湿实验智能编排 SCP Server

🧪 上海人工智能实验室正式推出面向生物实验流程设计与自动生成的LLM智能体——**Thoth 湿实验智能编排**。该系统以大模型为核心，专注于从自然语言科研需求出发，自动生成**精确、结构化且可执行的生物实验 protocol**，并通过结构化组件奖励机制显式建模“步骤粒度、逻辑顺序与语义一致性”，大幅提升实验方案的可复现性与可靠性。依托覆盖 27 个生物学子领域的 SciRecipe 协议数据集、全新的 “Sketch-and-Fill” 协议生成范式以及分阶段的 Knowledge-to-Action 训练流程，Thoth 在 SciRecipe-Eval 与多项公开科学问答基准上取得了显著领先的 SOTA 表现，为构建真正“懂实验”的科研智能体奠定了关键基础。

## ✨ Core Features

1. **大规模结构化协议语料库支撑，覆盖 27 个生物学子领域**  
   Thoth 由全新的 `SciRecipe` 数据集提供数据基础，该数据集包含超过 1.2 万条高结构化实验协议，系统性覆盖从实验Protocol理解VQA到实验Protocol生成的多种任务形态。 
   - **多学科覆盖**：囊括 27 个生物学子领域，涵盖分子生物学、细胞实验、组学分析等典型场景；  
   - **多任务范式**：同时支持协议 **理解（comprehension)** 与 **协议生成 / 改写（problem-solving）** 任务；  
   - **结构化标注**：对实验目的、步骤、参数、材料等关键要素进行显式结构化，天然适配下游推理与奖励设计。  

2. **“Sketch-and-Fill” 三阶段协议生成范式，显式拆解科学推理**  
   为避免传统 LLM 一步到位生成协议时出现的**逻辑跳步、信息缺失与结构混乱**问题，Thoth 引入了全新的 `Sketch-and-Fill` 协议生成流程，将生成过程拆解为三个可监督、可检查的阶段：
   - **Analysis（分析）**：从自然语言需求中抽取实验目标、变量与约束条件，明确问题边界；  
   - **Structuring（结构化）**：规划高层协议骨架，确定关键模块划分与步骤顺序；  
   - **Expression（表达）**：在固定骨架下补全每一步的具体操作细节、参数与注意事项。  
   通过将“想清楚”和“写清楚”解耦，Thoth 将科学推理过程显式暴露出来，便于人工审阅、自动校验以及后续工具链对接。  

3. **结构化组件奖励机制：直接对齐“可执行性”与“逻辑严谨性”**  
   针对实验协议的特殊结构，Thoth 设计了**结构化组件（component-based）奖励机制**，不再仅仅依赖语言模型的困惑度或粗粒度打分，而是从协议本身的结构属性出发进行细粒度评估：
   - **步骤粒度（Step Granularity）**：鼓励既不过度粗糙也不过度碎片化的合理步骤划分；  
   - **动作顺序（Action Order）**：显式检查关键操作的前后依赖关系，惩罚步骤错序与逻辑冲突；  
   - **语义一致性（Semantic Fidelity）**：衡量生成协议与原始任务意图之间的一致性与信息完整性。  
   该机制将训练信号与“是否真正能在实验台上执行”直接绑定，使得模型优化更贴近实际科研需求，而非仅追求文本层面的流畅与自然。  

4. **Knowledge-to-Action 分阶段训练：从知识掌握到操作推理**  
   Thoth 采用分阶段的 `Knowledge-to-Action` 训练流程，将模型从“会背书”逐步推向“会做实验”：  
   - **Stage 1：知识获取（Knowledge Acquisition）**  
     在 SciRecipe 及相关知识语料上进行指令微调，使模型掌握实验概念、术语与典型操作；  
   - **Stage 2：操作推理（Operational Reasoning）**  
     通过结构化任务，引导模型学习如何将实验目标拆解为合理的操作序列；  
   - **Stage 3：协议强化（Executable Protocol Generation）**  
     结合结构化组件奖励进行 RL 微调，使模型生成的协议在可执行性、逻辑严谨性和语义准确性上进一步提升。  
   这一“从知识到行动”的渐进式范式，使得 Thoth 不仅能回答“该做什么”，更能严谨地给出“如何一步一步做”。  

5. **面向湿实验协议的一站式智能服务节点**
   Thoth SCP Server 将复杂的协议生成能力封装为“开箱即用”的平台服务，面向不同层级的科研用户提供统一入口：  
   - **一站式 Protocol 服务**：支持从“需求描述 → 实验目标分析 → 协议草稿 → 结构化完善”的端到端协议生成与优化；  
   - **多场景适配**：支持新实验设计、现有 protocol 重写 / 标准化、参数调整、对照实验扩展等多种场景；  
   - **结构化输出**：返回包含实验目的、材料试剂、仪器设备、步骤序列、关键参数与注意事项的高结构化协议，便于直接落地到 ELN/LIMS 或机器人执行系统中。   

## 🚀Quick Start

#### 1. Dependence

使用 Server 推荐使用python 3.10+，仅需安装 `mcp` 即可，命令如下：

```
pip install mcp
```

#### 2. Configuration

请参考以下代码定义 Server 客户端，其中包含 `connect`、`disconnect`、`list_tools` 和 `_parse_result` 方法：

```python
import asyncio
import json
from datetime import datetime
from mcp.client.streamable_http import streamablehttp_client
from mcp import ClientSession


class ThothToolsClient:
    def __init__(self, server_url:str="https://scp.intern-ai.org.cn/api/v1/mcp/19/Thoth-Plan"):
        self.server_url = server_url
        self.session = None

    async def connect(self):
        """建立连接并初始化会话"""
        print(f"\n{'='*80}")
        print("连接到同步PCR工具MCP服务器")
        print(f"{'='*80}")
        print(f"服务器地址: {self.server_url}")

        try:
            # 建立streamable-http传输连接
            self.transport = streamablehttp_client(
                url=self.server_url, 
                headers={
                    "SCP-HUB-API-KEY": "sk-XXX"  # 替换为实际的API Key
                },
                sse_read_timeout=60 * 10)
            self.read, self.write, self.get_session_id = await self.transport.__aenter__()

            # 创建客户端会话
            self.session_ctx = ClientSession(self.read, self.write)
            self.session = await self.session_ctx.__aenter__()

            # 初始化会话
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
        print(f"\n{'='*80}")
        print("列出所有可用工具")
        print(f"{'='*80}")

        try:
            tools_list = await self.session.list_tools()
            print(f"\n可用工具 (共{len(tools_list.tools)}个):\n")

            for i, tool in enumerate(tools_list.tools, 1):
                print(f"{i:2d}. {tool.name}")
                if tool.description:
                    # 只显示描述的第一行
                    desc_line = tool.description
                    print(f"    {desc_line}")

            print(f"\n✓ 工具列表获取成功")
            return tools_list.tools

        except Exception as e:
            print(f"✗ 列出工具失败: {e}")
            return []

    def _parse_result(self, result):
        """解析SCP工具调用结果"""
        try:
            return result.content[0].text
        except Exception as e:
            import traceback
            traceback.print_exc()
            return {"error": f"解析结果失败: {e}", "raw": str(result)}
```

## 📊Usage

以下是一个基于用户需求生成“基于斑点的菌落PCR筛选三角褐指藻”实验方案的代码示例：

```python
async def main():
    client = ThothToolsClient()
    if not await client.connect():
        print("connection failed")
        return

    prompt = "Could you describe the full set of laboratory steps used in the patch based colony PCR protocol for screening Phaeodactylum tricornutum?" 
    print("Generating a protocol...")
    result = await client.session.call_tool(
        "protocol_generation",
        arguments={
            "user_prompt": prompt
        }
    )
    protocol = client._parse_result(result)
    print("=" * 20)
    print("Generated protocol:")
    print(protocol)
    await client.disconnect()
    
if __name__ == "__main__":
    import asyncio
    asyncio.run(main())

```

## 工具列表与参数

### protocol_generation
Generation experiment protocol given user prompt.
Params：
    user_prompt (str): requirements for the experiment

Returns：
    protocol (str): generated protocol text
参数：
- `user_prompt`（string，必填）

### generate_executable_json
Generate executable JSON file from protocol text.
Params:
    protocol (str): protocol text

Returns:
    json (str): executable JSON string
参数：
- `protocol`（string，必填）

### extract_protocol_from_pdf
Extract experimental protocol from a provided PDF
Params:
    pdf_url (str): PDF file URL

Returns:
    protocol (str): protocol text
参数：
- `pdf_url`（string，必填）

### execute_json
Execute JSON with PCR operation server
Params:
    json (str): executable JSON
    server_url (str): PCR operation server URL; default to Thoth-OP server URL

Returns:
    exec_info (str): execution information
参数：
- `executable_json`（string，必填）
- `server_url`（，可选，默认 http://101.126.42.198:8080）