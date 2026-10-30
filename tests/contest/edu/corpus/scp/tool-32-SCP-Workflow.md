# SCP-Workflow
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：2
调用次数：221
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/32/SCP-Workflow
标签：通用, 数据库, 计算工具

## 介绍
SCP Workflow是SCP生态中面向工具调用与流程编排的智能管理服务。SCP Workflow 提供一套统一、可扩展的工具发现与调度接口，让用户（或上层应用与自动化流程）能够依据实验任务需求，灵活发现、选择并组合所需的SCP工具。SCP Workflow 作为SCP工具层的智能协调中枢，目前专注于实现工具列表的全量查询、按条件筛选与推荐，并持续演进以支持更动态、自适应的工具编排策略。

## 详细说明与使用方法
# SCP Workflow 智能工具编排服务

🔧 SCP Workflow 是 SCP 生态中面向工具调用与流程编排的智能管理服务，旨在为复杂科学实验提供**统一、可扩展**的工具发现与调度能力。该系统以大语言模型为智能核心，专注于从自然语言实验需求出发，自动**发现、筛选并推荐**最相关的 SCP 工具链，构建符合实验逻辑的可执行工作流。通过引入 LLM 驱动的智能工具推荐与条件筛选机制，SCP Workflow 能够理解用户自然语言描述的实验任务，自动匹配最相关的实验工具，为复杂实验流程的自动化构建提供关键支撑。作为 SCP 工具层的**智能协调中枢**，它正在从基础的工具发现服务，持续演进为支持动态、自适应工具编排策略的全流程调度引擎。

## ✨ Core Features

1. **统一工具发现接口：标准化查询与动态扩展**  
   SCP Workflow 提供了一套标准化、可扩展的工具发现与查询接口，支持用户或上层应用灵活访问整个 SCP 生态的工具资源。通过统一的 MCP 协议封装，屏蔽底层工具实现差异，实现跨领域、跨平台的工具资源聚合与调用。
   - **标准化查询**：提供统一 API 接口，支持工具全量查询、按条件筛选与元数据检索；
   - **动态扩展**：支持新工具服务的即插即用式接入，无需修改核心调度逻辑；
   - **元数据丰富**：每个工具附带详细的功能描述、输入输出格式、执行限制与领域标签，便于精准匹配。

2. **智能工具推荐：LLM 驱动的意图理解与精准匹配**  
   针对用户自然语言描述的复杂实验需求，SCP Workflow 引入 LLM 驱动的智能推荐机制，将模糊的需求描述转化为具体的工具调用序列，显著提升工具发现的准确性与效率。
   - **意图理解**：通过 LLM 分析用户查询，自动识别实验任务类型、关键步骤与约束条件；
   - **语义匹配**：基于工具功能描述与用户需求间的语义相似度，推荐最相关的工具组合；
   - **多维度筛选**：支持按实验领域、操作类型、资源限制等多维度条件进行工具过滤与排序。

3. **结构化任务分析：从需求分解到工具映射**  
   为避免传统关键字匹配的局限性，SCP Workflow 将工具发现过程结构化为**需求分析→任务拆解→工具映射**的多阶段推理流程，确保推荐结果的逻辑合理性与可执行性。
   - **需求结构化**：将用户自然语言需求解析为结构化的实验目标、步骤规划与参数约束；
   - **任务分解**：将复杂实验流程拆解为原子操作单元，明确各步骤的输入输出依赖；
   - **工具映射**：为每个原子操作匹配最合适的 SCP 工具，构建端到端的可执行工作流。

4. **可扩展的推荐框架：支持自定义规则与领域适配**  
   SCP Workflow 采用模块化设计，支持用户根据特定实验场景定制推荐策略，确保系统在不同科研领域的适用性与灵活性。
   - **规则引擎**：支持基于领域知识的自定义匹配规则，可针对特定实验类型优化推荐逻辑；
   - **反馈机制**：支持用户对推荐结果的评价与修正，持续优化推荐模型；
   - **多策略支持**：除 LLM 推荐外，同时支持基于规则、协同过滤、知识图谱等多种推荐策略的灵活切换与组合。

## 🚀 Quick Start

#### 1. Dependence

推荐使用 Python 3.10+，只需安装 `mcp` 包即可：

```bash
pip install mcp
```

#### 2. Configuration

以下为 SCP Workflow 客户端示例代码，展示了如何连接服务、查询工具列表并进行智能筛选：

```python
import asyncio
import json
from datetime import datetime
from mcp.client.streamable_http import streamablehttp_client
from mcp import ClientSession


class SCPWorkflowClient:
    def __init__(self, server_url: str = "https://scp.intern-ai.org.cn/api/v1/mcp/32/SCP-Workflow"):
        self.server_url = server_url
        self.session = None

    async def connect(self):
        """建立连接并初始化会话"""
        print(f"\n{'='*80}")
        print("连接到 SCP Workflow 智能工具编排服务")
        print(f"{'='*80}")
        print(f"服务器地址: {self.server_url}")

        try:
            # 建立 streamable-http 传输连接
            self.transport = streamablehttp_client(
                url=self.server_url,
                headers={
                    "SCP-HUB-API-KEY": "sk-XXX"  # 替换为实际的API Key
                },
                sse_read_timeout=60 * 10,
                timeout=60 * 20
            )
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
            if hasattr(result, 'content') and result.content:
                content = result.content[0]
                if hasattr(content, 'text'):
                    return json.loads(content.text)
            return str(result)
        except Exception as e:
            import traceback
            traceback.print_exc()
            return {"error": f"解析结果失败: {e}", "raw": str(result)}
```

## 📊 Usage

以下是一个基于用户实验需求智能推荐相关工具的代码示例：

```python
async def main():
    # 初始化客户端
    client = SCPWorkflowClient()

    # 连接服务
    if not await client.connect():
        print("连接失败，请检查服务器地址与网络配置")
        return

    # await client.list_tools()

    try:
        user_requirement = """
        我需要计算药物分子的毒性和可成药性，请推荐相关的工具
        """

        print("用户需求:")
        print(user_requirement)
        print("\n正在智能筛选相关工具...")

        # 调用智能工具筛选服务
        result = await client.session.call_tool(
            "query_and_filter_tools",
            arguments={
                "prompt": user_requirement,
            }
        )

        # 解析返回结果
        result_data = client._parse_result(result)

        print("\n" + "="*60)
        print("智能推荐结果:")
        print("="*60)

        # 解析并展示推荐结果
        if "task_type" in result_data:
            print(f"分析的任务类型: {result_data['task_type']}")

        if "required_tools" in result_data and result_data["required_tools"]:
            print(f"\n推荐的SCP工具 (共{len(result_data['required_tools'])}个):")
            for i, tool_name in enumerate(result_data['required_tools'], 1):
                print(f"  {i}. {tool_name}")
        else:
            print("\n未找到匹配的工具，请尝试调整需求描述或联系管理员。")

        # 显示其他元数据
        if "note" in result_data:
            print(f"\n说明: {result_data['note']}")

    except Exception as e:
        print(f"工具调用过程中出错: {e}")
        import traceback
        traceback.print_exc()

    finally:
        # 断开连接
        await client.disconnect()


if __name__ == "__main__":
    import asyncio
    asyncio.run(main())
```

## 工具列表与参数

### generate_presigned_url
Generate pre-signed URLs for uploading and downloading objects from Alibaba Cloud OSS.

Args:
    key: The object name/path in the bucket.
    expires_seconds: URL expiration time in seconds. Defaults to 3600 (1 hour).
    
Returns:
    A dictionary containing pre-signed URLs for both upload and download operations.
参数：
- `expires_seconds`（integer，可选，默认 3600）
- `key`（string，必填）

### query_and_filter_tools
Query and filter tools based on user prompt

Args:
    prompt: User's query describing what tools they need
    
Returns:
    dict: {
        "task_type": "identified task type", # will be "error occurred" when internal error occurred
        "required_tools": {
            "tool_1_prefix_name": {
                "server": "server of tool 1",
                "original_name": "original name of tool 1",
                "description": "description of tool 1"
            },
            ...
        }
    }
参数：
- `prompt`（string，必填）