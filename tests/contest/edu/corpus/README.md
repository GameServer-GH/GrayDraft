# 赛事双工作区测试语料

对应报告：[`../REPORT-scp-papers-2026-10-29.md`](../REPORT-scp-papers-2026-10-29.md)。

本目录保存 2026-10-29 联调时使用的**公开原文**（导入前版本）。不含密钥。

| 子目录 | 用途 | 导入工作区密钥 | 说明 |
| --- | --- | --- | --- |
| `scp/` | 书生 SCP 广场技能说明与工具介绍/参数 | `contest-scp-skills` | 技能来自 InternScience/scp 公开 `SKILL.md`；工具来自广场公开 API |
| `papers/` | 上海人工智能实验室 / OpenGVLab 论文摘录 | `contest-papers` | `*.md` 为实际入库摘录；`*.full.txt` 为 arXiv HTML 抽取的更长原文备份 |

## 文件一览

### `scp/`

- `skill-*.md`：`multiomics_integration`、`sciverse-academic-retrieval`、`chemical_safety_assessment`、`alphafold_structure_pipeline`、`protocol-to-executable-json`、`wind-site-assessment`、`oceanographic-seawater-properties`、`seismic-waveform-processing`
- `tool-*.md`：Sciverse、BioInfo-Tools、Thoth-Plan、CrystalX、SCP-Workflow、OceanGSW-Tool

### `papers/`

| 文件 | 论文 | arXiv |
| --- | --- | --- |
| `internlm2.md` / `.full.txt` | InternLM2 Technical Report | [2403.17297](https://arxiv.org/abs/2403.17297) |
| `internvl.md` / `.full.txt` | InternVL | [2312.14238](https://arxiv.org/abs/2312.14238) |
| `internvl25.md` / `.full.txt` | InternVL 2.5 | [2412.05271](https://arxiv.org/abs/2412.05271) |

## 导入提示

Worker 质量门禁可能拒绝「字符连续重复过长」或「信息密度过低」的长文。联调时对正文做了重复折叠与分段后再入库；复现时可直接用本目录 `.md`，若被拒再按报告第 2、6 节同样处理。

广场来源：https://scphub.intern-ai.org.cn/
