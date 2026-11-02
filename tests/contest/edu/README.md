# 教育场景样例

`make dev-up` 后：

1. 把 `corpus/` 里的公开书生文档导入 `kb_type=tutorial`（论文页用 `paper`）。
2. 创建 Run，`policy_version` 可省略，服务端固定为 `edu_workspace_v1`。
3. `GET /api/v2/agent-runs/{run_id}/events/stream` 查看工具序列、引用或拒答。
4. `note.draft` 返回审批要求后，调用现有审批接口通过，笔记进入 `kb_type=note`，可被下一问检索。

环境变量见仓库 README。不要把密钥写进本目录。

评测条目在 `cases.json`。通过条件除答案外，还要求 `expect_tools` 出现在事件序列里。

## 双工作区升级回归

`retest_after_upgrade.py` 使用独立的冻结清单 `upgrade_regression_v1.json`（24 题，非原题集 S1）。默认跳过缓存、不改问法、不重试，结果写入唯一命名的 `runs/` 文件。

```bash
python3 tests/contest/edu/retest_after_upgrade.py --only paper-theme,paper-infra
python3 tests/contest/edu/retest_after_upgrade.py
python3 tests/contest/edu/retest_after_upgrade.py --max-retries 2
python3 tests/contest/edu/retest_after_upgrade.py --replay tests/contest/edu/runs/20261102T071837583148Z_full.json
```

首轮与重试后成绩分别保留；源码、清单或本地镜像在运行中变化会使该轮无效。退出码 0 表示所选回归全部通过，1 表示存在失败，2 表示参数错误或无效轮次。离线重评不调用 API、不覆盖原记录。

完整步骤与边界见[优化与验证工作流](../../../docs/architecture/optimization-workflow-2026-11-02.md)。关键词评分不能代替事实关系验证、原题集 S1 或新入库验收。

## 总览延迟重复对照

```bash
python3 tests/contest/edu/benchmark_tail.py --baseline \
  tests/contest/edu/runs/20261102T083302030088Z_gate.json \
  tests/contest/edu/runs/20261102T083306110835Z_gate.json \
  tests/contest/edu/runs/20261102T083314374554Z_gate.json --samples 5
```

只对相同问题、有效运行和单次客户端尝试做对照，保留所有失败样本及原始记录哈希；报告平均值、中位数和观测最大值，小样本不推导总体 P95。每次独立运行的指纹校验不能证明多个轮次之间环境相同，比较时仍需检查记录中的配置、镜像与测试条件。实现、失败实验和回退方式见[尾延迟优化记录](../../../docs/architecture/tail-latency-optimization-2026-11-02.md)。
