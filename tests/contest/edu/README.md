# 教育场景样例

`make dev-up` 后：

1. 把 `corpus/` 里的公开书生文档导入 `kb_type=tutorial`（论文页用 `paper`）。
2. 创建 Run，`policy_version` 可省略，服务端固定为 `edu_workspace_v1`。
3. `GET /api/v2/agent-runs/{run_id}/events/stream` 查看工具序列、引用或拒答。
4. `note.draft` 返回审批要求后，调用现有审批接口通过，笔记进入 `kb_type=note`，可被下一问检索。

环境变量见仓库 README。不要把密钥写进本目录。

评测条目在 `cases.json`。通过条件除答案外，还要求 `expect_tools` 出现在事件序列里。
