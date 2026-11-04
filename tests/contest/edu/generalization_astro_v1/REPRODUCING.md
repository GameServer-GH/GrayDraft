# 正确性基线和离线复核

冻结的天文题集见 [suite.json](suite.json)。完整基线按旧双工作区 24 题、中文天文 24 题、英文原始 PDF 4 题依次运行。每题保留首次结果。

完整运行需要本地开发服务和已导入的四个测试工作区。中文与英文 PDF 工作区的本地描述文件分别为 `workspace-remediation-accepted.json` 和 `workspace-remediation-pdf-preserved.json`。英文回归还需要 `sources/1602.03837v1.pdf`。工作区描述、PDF、语料快照和运行记录在本地维护。

从仓库根目录运行以下命令，将 `BASELINE_ID` 换成首次命令输出的基线目录名。

```sh
python3 tests/contest/edu/generalization_astro_v1/remediation/establish_baseline.py
python3 tests/contest/edu/generalization_astro_v1/remediation/collect_claim_reviews.py BASELINE_ID
```

离线纠错回放使用含原始候选、可见片段、摘要和 M87 运算记录的归档基线，复用本地开发 Gateway。边界检查和数值复算测试可独立运行。

```sh
cd tests/contest/edu/generalization_astro_v1/remediation
python3 -m unittest test_offline_repair test_numeric_review_replay
```
