# BioInfo-Tools
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：3
调用次数：899
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/17/BioInfo-Tools
标签：生命科学, 数据库, 计算工具, 模型服务

## 介绍
BioInfo-Tools MCP Server 是面向蛋白质序列分析的专业工具服务，集成 InterProScan 和 BLAST，支持结构域识别、GO 注释和序列相似性搜索。基于 MCP 协议封装，提供统一 API 接口，开箱即用。

## 详细说明与使用方法
# BioInfo-Tools MCP Server

🧬 上海人工智能实验室推出 BioInfo-Tools MCP Server，这是一款面向蛋白质序列分析的专业工具服务，集成 InterProScan 和 BLAST 核心生物信息学工具，支持蛋白质结构域识别、功能注释、序列相似性搜索等操作，为生物学研究提供高效的蛋白质分析能力。

## ✨ Core Features

1. **工具功能丰富，覆盖蛋白质分析核心环节**

   BioInfo-Tools MCP Server 整合了业界主流的生物信息学分析工具，全面支持蛋白质序列的结构与功能解析。工具链包括：基于 InterProScan 的蛋白质结构域识别（支持 Pfam、Gene3D、SUPERFAMILY、PANTHER 等数据库）、GO 功能注释、基于 BLAST 的序列相似性搜索、同源蛋白鉴定以及物种与基因信息获取等，覆盖从序列输入到功能注释的完整分析流程。

2. **统一接口，一键调用，开箱即用**

   所有工具通过标准化的 MCP（Model Context Protocol）协议封装，提供一致的 API 调用方式，无需用户本地安装 InterProScan、BLAST 等复杂依赖或手动配置运行环境。研究人员仅需几行 Python 代码即可远程调用任意功能模块，实现从输入到结果的一键式分析，显著降低技术门槛，提升研发效率。

## 🚀 Quick Start

#### 1. Dependence

推荐使用 Python 3.10+，仅需安装 `mcp` 即可：

```
pip install mcp
```

#### 2. Configuration

请参考以下代码定义 Server 客户端，其中包含 `connect`、`disconnect`、`list_tools` 和 `parse_result` 方法：

```python
import asyncio
import json
from mcp.client.streamable_http import streamablehttp_client
from mcp import ClientSession

class BioInfoToolsClient:
    
    def __init__(self, server_url: str="https://scp.intern-ai.org.cn/api/v1/mcp/17/BioInfo-Tools"):
        self.server_url = server_url
        self.session = None
        
    async def connect(self):
        """建立连接并初始化会话"""
        print(f"\n{'='*80}")
        print("连接到 BioInfo-Tools MCP Server")
        print(f"{'='*80}")
        print(f"服务器地址: {self.server_url}")
        
        try:
            # 建立streamable-http传输连接
            self.transport = streamablehttp_client(
                url=self.server_url, 
                headers={"SCP-HUB-API-KEY": "sk-xxx"}
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
                    desc_line = tool.description.split('\n')[0]
                    print(f"    {desc_line}")
            
            print(f"\n✓ 工具列表获取成功")
            return tools_list.tools
            
        except Exception as e:
            print(f"✗ 列出工具失败: {e}")
            return []
    
    def parse_result(self, result):
        """解析SCP工具调用结果"""
        try:
            if hasattr(result, 'content') and result.content:
                content = result.content[0]
                if hasattr(content, 'text'):
                    return json.loads(content.text)
            return str(result)
        except Exception as e:
            return {"error": f"解析结果失败: {e}", "raw": str(result)}
```

## 📊 Usage

以下是 BLAST 序列相似性搜索的示例代码：

```python
async def main():
    ## 客户端创建和连接
    client = BioInfoToolsClient()
    if not await client.connect():
        print("连接失败")
        return
    
    ## 输入参数
    protein_sequence = "MVHLTPEEKSAVTALWGKVNVDEVGGEALGRLLVVYPWTQRFFESFGDLSTPDAVMGNPKVKAHGKKVLGAFSDGLAHLDNLKGTFATLSELHCDKLHVDPENFRLLGNVLVCVLAHHFGKEFTPPVQAAYQKVVAGVANALAHKYH"
    
    ## 调用工具
    result = await client.session.call_tool(
        "blast_search",
        arguments={
            "sequence": protein_sequence,
            "sequence_id": "query_protein",
            "evalue": 0.01,
            "max_hits": 10
        }
    )
    
    ## 结果解析
    result_data = client.parse_result(result)
    if result_data.get("success"):
        for hit in result_data["hits"][:5]:
            print(f"UniProt ID: {hit['uniprot_id']}")
            print(f"Identity: {hit['identity_percent']}%")
            print(f"E-value: {hit['evalue']}")
            print(f"Organism: {hit['organism']}")
            print("-" * 80)
    
    ## 客户端断开
    await client.disconnect()

if __name__ == '__main__':
    asyncio.run(main())
```

## 🛠️ Tool List

| 工具名称 | 功能描述 |
|---------|---------|
| **blast_search** | 在 UniProt Swiss-Prot 数据库中搜索相似蛋白质序列 |
| **interproscan_analyze** | 使用 InterProScan 识别蛋白质结构域和功能注释 |
| **analyze_protein** | 综合分析，同时运行 InterProScan 和 BLAST |

### blast_search

**参数：**

- `sequence` (string, 必需): 蛋白质氨基酸序列，最少10个残基
- `sequence_id` (string, 可选): 查询序列的标识符，默认 "query"
- `evalue` (number, 可选): E-value 阈值，默认 0.01
- `max_hits` (integer, 可选): 返回的最大结果数，默认 10

**返回结果包含：** uniprot_id, description, organism, gene_name, identity_percent, coverage_percent, evalue, bit_score, alignment_length 等字段

### interproscan_analyze

**参数：**

- `sequence` (string, 必需): 蛋白质氨基酸序列，最少10个残基
- `sequence_id` (string, 可选): 查询序列的标识符，默认 "query"
- `databases` (array, 可选): 要搜索的数据库列表，默认 ["Pfam"]。支持：Pfam, Gene3D, SUPERFAMILY, PANTHER, CDD, SMART 等
- `goterms` (boolean, 可选): 是否包含 GO 注释，默认 true

**返回结果包含：** domains (结构域列表), go_terms (GO 注释列表), pathways (通路信息)

### analyze_protein

**参数：** 结合上述两个工具的所有参数

**返回结果：**
```json
{
  "interproscan": { /* interproscan_analyze 的完整结果 */ },
  "blast": { /* blast_search 的完整结果 */ }
}
```

## 📝 Notes

**序列要求：** 长度范围 10 - 50,000 个氨基酸，支持标准氨基酸字母及特殊字符 (X, U, B, Z, J, O, *, -)

**执行时间：** blast_search 约 10-60 秒，interproscan_analyze 约 2-10 分钟，analyze_protein 约 2-10 分钟（并行执行）

**错误处理：** 所有工具返回结果包含 `success` 字段，建议使用 `result_data.get("success")` 检查执行状态

## 🙏 Acknowledgements

感谢上海人工智能实验室 AI4S 中心对 BioInfo-Tools MCP Server 整理、部署和发布所提供的大力支持。

## 工具列表与参数

### interproscan_analyze
Analyze protein sequence using InterProScan to identify functional domains, families, and GO terms.

Use this tool when you need to:
- Identify protein domains and families (Pfam, Gene3D, SUPERFAMILY, etc.)
- Get Gene Ontology (GO) annotations
- Understand protein function and structure

Returns: domains found, GO terms, and InterPro annotations.
Note: Analysis may take 2-10 minutes depending on sequence length.
参数：
- `databases`（array，可选，默认 ['Pfam']）：Databases to search. Options: Pfam, Gene3D, SUPERFAMILY, PANTHER, CDD, SMART, etc.
- `goterms`（boolean，可选，默认 True）：Whether to include GO term annotations
- `sequence`（string，必填）：Protein amino acid sequence (e.g., 'MKTAYIAKQRQISFVKSHFSRQ...'). Minimum 10 residues.
- `sequence_id`（string，可选，默认 query）：Optional identifier for the sequence

### blast_search
Search for similar protein sequences in UniProt Swiss-Prot database using BLAST.

Use this tool when you need to:
- Find homologous proteins
- Identify the protein by sequence similarity
- Get organism and gene name information
- Check if a sequence matches known proteins

Returns: matching proteins with identity%, coverage%, E-value, organism, and alignment details.
参数：
- `evalue`（number，可选，默认 0.01）：E-value threshold. Lower values = more stringent (e.g., 1e-10 for high confidence)
- `max_hits`（integer，可选，默认 10）：Maximum number of hits to return (1-100)
- `sequence`（string，必填）：Protein amino acid sequence to search. Minimum 10 residues.
- `sequence_id`（string，可选，默认 query）：Optional identifier for the query sequence

### analyze_protein
Comprehensive protein analysis combining InterProScan (domain analysis) and BLAST (similarity search).

Use this tool when you need a complete analysis of an unknown protein sequence, including:
- Domain and family identification
- GO term annotations
- Similar proteins in UniProt
- Organism and gene information

This runs both analyses in parallel for efficiency.
Note: May take 2-10 minutes to complete.
参数：
- `databases`（array，可选，默认 ['Pfam']）：InterProScan databases to use
- `evalue`（number，可选，默认 0.01）：BLAST E-value threshold
- `max_hits`（integer，可选，默认 10）：Maximum BLAST hits to return
- `sequence`（string，必填）：Protein amino acid sequence for comprehensive analysis
- `sequence_id`（string，可选，默认 query）：Optional identifier for the sequence