# NATS JetStream 高可用（O-04）

## 配置

Core / Worker 建流时使用 `nats.jetstream.replicas`（环境变量 `NATS_REPLICAS`）：

| 环境 | 建议 replicas |
|------|----------------|
| dev | `1`（单节点 NATS） |
| prod | `3`（需 3 节点 NATS 集群） |

主流 `GRAYDRAFT_UGC` 与 DLQ 流 `GRAYDRAFT_UGC_DLQ` 使用相同副本数。

## 生产 NATS 集群（概要）

1. 部署 3 个 NATS 节点，启用 `-js`，配置 `cluster` 路由。
2. 将 `NATS_URL` 设为客户端连接串（可逗号分隔多地址）。
3. 设置 `NATS_REPLICAS=3`，重启 Core 与 Worker 以 `UpdateStream` 提升副本（需集群已就绪）。

## 验证

```bash
nats stream info GRAYDRAFT_UGC
# 查看 Replicas、Cluster 状态
```

单节点 dev 勿将 `replicas` 设为 3，否则建流会失败。
