# OceanGSW-Tool
来源：书生 SCP 广场工具服务 https://scphub.intern-ai.org.cn/
发布方：上海人工智能实验室
工具数量：7
调用次数：242
接入端点：https://scp.intern-ai.org.cn/api/v1/mcp/34/OceanGSW-Tool
标签：地球科学, 计算工具

## 介绍
OceanGSW-Tool 是面向海洋科学与物理海洋学的专业化计算工具集，集成了国际海水热力学方程标准（TEOS-10）实现库 gsw-python 的常用核心功能，通过吉布斯海水状态方程，支持海水热力学参数计算、衍生海洋学参数估算、温标与盐度单位转换等关键操作。

## 详细说明与使用方法
# OceanGSW-Tool SCP Server

🌏上海人工智能实验室打造了OceanGsw-Tool，这是一款面向海洋科学、物理海洋学研究与工程应用的专业化计算工具集，集成 gsw-python 开源库的核心算法（严格遵循国际海水状态方程 TEOS-10 标准）。该工具集秉承 “精准高效、开箱即用” 的理念，融合了海水基础热力学计算、海洋参数标准化转换、衍生动力参数分析等核心板块。平台支持批量观测数据处理、自动化参数设置和智能结果输出，极大提高了海洋观测数据诊断分析、海洋工程水文评估的效率，为科研人员、工程师及教学用户在海洋环流研究、气候变化分析、海洋热力学探索等场景提供了更高效的技术保障。

## **✨Core Features**

1. **工具种类丰富，覆盖海洋科学数据分析关键环节**
    OceanGsw-Tool是一款面向海洋科学、物理海洋学研究与工程应用的专业化计算工具集，集成 gsw-python 开源库的核心算法（严格遵循国际海水状态方程 TEOS-10 标准），支持海水热力学参数（海水冰点、位势密度、声速等）精准计算、温标与盐度单位标准化转换、海洋衍生动力参数（海洋动力高度）智能估算等关键操作。
2. **统一接口，一键调用，开箱即用**
    所有工具通过标准化的 MCP（Model Calling Protocol）协议封装，提供一致的 API 调用方式，无需用户本地安装依赖或手动配置复杂环境。研究人员仅需几行 Python 代码即可远程调用任意功能模块，实现从输入到结果的一键式分析，显著降低技术门槛，提升研发效率，让使用者专注于科学探索而非工程实现。

## 🚀Quick Start

#### 1. Dependence

推荐使用python 3.10+，仅需安装 `mcp` 即可，命令如下：

```
pip install mcp
```

#### 2. Configuration

请参考以下代码定义 Server 客户端，其中包含 `connect`、`disconnect`、`list_tools` 和 `parse_result` 方法（注意替换SCP-HUB-API-KEY）：

```python
Ocean_SERVER_URL = "https://scp.intern-ai.org.cn/api/v1/mcp/34/OceanGSW-Tool"  ## Earthquake-Model Server地址

class OceanGSWToolClient:
    """使用MCP SDK的客户端"""
    
    def __init__(self, server_url: str):
        self.server_url = server_url
        self.session = None

        
    async def connect(self):
        """建立连接并初始化会话"""
        print(f"\n{'='*80}")
        print("连接到SeisOBS-Tool MCP服务器")
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
                    # 只显示描述的第一行
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

## Usage 

以下是一个计算海水绝对盐度的代码示例：

```python
async def main():
    ## client创建和连接
    client = OceanGSWToolClient(Ocean_SERVER_URL)
    if not await client.connect():
        print("connection failed")
        return
   	
   	##输入参数
    input_params = {'SP': [35.0,5.0], 
                    'p': [1000.0, 1000.0], 
                    'lon': [120.0, 165.0], 
                    'lat': [30.0, 45.0]}
    
    ## 调用工具
    result = await client.session.call_tool(
        "gsw_example_absolute_salinity",
        arguments={
            "SP": input_params['SP'],
            'p': input_params['p'],
            'lon':input_params['lon'],
            'lat':input_params['lat'],
        }
    )
    
    ## 结果解析
    result_data = client.parse_result(result)
    SA_result = result_data["st"]
    for i, item in enumerate(SA_result):
    	print ("SP: ", input_params['SP'][i], ", SA: ", item)
    
    ## client断开
    await client.disconnect()
    
if __name__ == '__main__':
    await main()
```

## 工具列表与参数

### gsw_example_absolute_salinity
GSW Example Absolute Salinity
Args:
    SP: 实用盐度 (PSU)
    p: 压力 (dbar)
    lon: 经度 (°E)
    lat: 纬度 (°N)
Returns:
    {'gsw_example_absolute_salinity': Dict[str, Any]}
参数：
- `SP`（array，必填）
- `lat`（array，必填）
- `lon`（array，必填）
- `p`（array，必填）

### gsw_example_dynamic_height
GSW Example Dynamic Height
Args:
    SA: 绝对盐度列表 (g/kg)
    CT: 保守温度列表 (°C)
    p: 压力列表 (dbar)
    p_ref: 参考压力 (dbar)，默认 0 dbar
Returns:
    {'gsw_example_dynamic_height': Dict[str, Any]}
参数：
- `CT`（array，必填）
- `SA`（array，必填）
- `p`（array，必填）
- `p_ref`（number，必填）

### gsw_example_freezing_temp
GSW Example Freezing Temp
Args:
    SA: 绝对盐度列表 (g/kg)
    p: 压力，单位：dbar（分巴）
    saturation_fraction: 气体饱和分数（默认 0.0，表示无气体饱和）
Returns:
    {'gsw_example_freezing_temp': Dict[str, Any]}
参数：
- `SA`（array，必填）
- `p`（array，必填）
- `saturation_fraction`（array，必填）

### gsw_example_potential_density_sigma0
GSW Example Potential Density Sigma0
Args:
    SA: 绝对盐度列表 (g/kg)
    CT: 保守温度列表 (°C)
Returns:
    {'gsw_example_potential_density_sigma0': Dict[str, Any]}
参数：
- `CT`（number，必填）
- `SA`（number，必填）

### gsw_example_sound_speed
GSW Example Sound Speed
Args:
    SA: 绝对盐度数组，g/kg
    CT: 保守温度数组，°C
    p: 压力数组，单位：dbar（分巴）
Returns:
    {'gsw_example_sound_speed': Dict[str, Any]}
参数：
- `CT`（array，必填）
- `SA`（array，必填）
- `p`（array，必填）

### gsw_example_SA_CT_density
GSW Example SA CT Density
Args:
    SA: 绝对盐度数组，g/kg
    CT: 保守温度数组，°C
    p: 压力数组，单位：dbar（分巴）
Returns:
    {'gsw_example_SA_CT_density': Dict[str, Any]}
参数：
- `CT`（array，必填）
- `SA`（array，必填）
- `p`（array，必填）

### gsw_example_conservative_temperature
服务端调用接口：从SA、t、p计算保守温度（CT）

Args:
    SA: 绝对盐度列表（g/kg）
    t: 原位温度列表（°C）
    p: 压力列表（dbar）

Returns:
    Dict[str, Any]：包含核心函数返回结果的字典，键为函数名
参数：
- `SA`（array，必填）
- `p`（array，必填）
- `t`（array，必填）