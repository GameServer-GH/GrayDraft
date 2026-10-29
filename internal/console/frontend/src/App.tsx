import { useEffect, useState } from "react";
import {
  classifyPreview,
  discardDLQ,
  fetchAgentEvals,
  fetchAgentEvalQuality,
  fetchAgentRunTrace,
  fetchCostSummary,
  fetchToolApprovals,
  decideToolApproval,
  fetchDLQMessages,
  fetchDLQStats,
  fetchHealth,
  fetchOverview,
  fetchUGCTasks,
  fetchAdminChunks,
  replayDLQ,
  runAgentEval,
  search,
  streamChat,
  ugcNotify,
} from "./api";
import { clearToken, getToken, login, setToken } from "./auth";

type Tab = "dashboard" | "ugc" | "dlq" | "rag" | "chunks" | "agent-trace" | "cost-eval" | "approvals";

export default function App() {
  const [authed, setAuthed] = useState(() => Boolean(getToken()));
  const [tab, setTab] = useState<Tab>("dashboard");

  if (!authed) {
    return <LoginScreen onSuccess={() => setAuthed(true)} />;
  }

  return (
    <div className="layout">
      <nav className="nav">
        <h1>GrayDraft Console</h1>
        <button
          type="button"
          className="logout"
          onClick={() => {
            clearToken();
            setAuthed(false);
          }}
        >
          退出
        </button>
        <button className={tab === "dashboard" ? "active" : ""} onClick={() => setTab("dashboard")}>
          概览
        </button>
        <button className={tab === "ugc" ? "active" : ""} onClick={() => setTab("ugc")}>
          UGC 任务
        </button>
        <button className={tab === "dlq" ? "active" : ""} onClick={() => setTab("dlq")}>
          DLQ
        </button>
        <button className={tab === "rag" ? "active" : ""} onClick={() => setTab("rag")}>
          RAG 调试
        </button>
        <button className={tab === "chunks" ? "active" : ""} onClick={() => setTab("chunks")}>
          知识条目
        </button>
        <button className={tab === "agent-trace" ? "active" : ""} onClick={() => setTab("agent-trace")}>
          Agent Trace
        </button>
        <button className={tab === "cost-eval" ? "active" : ""} onClick={() => setTab("cost-eval")}>
          成本 / 评估
        </button>
        <button className={tab === "approvals" ? "active" : ""} onClick={() => setTab("approvals")}>
          工具审批
        </button>
      </nav>
      <main className="main">
        {tab === "dashboard" && <Dashboard />}
        {tab === "ugc" && <UGCPage />}
        {tab === "dlq" && <DLQPage />}
        {tab === "rag" && <RAGPage />}
        {tab === "chunks" && <ChunksPage />}
        {tab === "agent-trace" && <AgentTracePage />}
        {tab === "cost-eval" && <CostEvalPage />}
        {tab === "approvals" && <ApprovalsPage />}
      </main>
    </div>
  );
}

function LoginScreen({ onSuccess }: { onSuccess: () => void }) {
  const [username, setUsername] = useState("admin");
  const [password, setPassword] = useState("");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        await fetchOverview();
        if (!cancelled) onSuccess();
      } catch {
        /* JWT enabled — show login form */
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [onSuccess]);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoading(true);
    setErr("");
    try {
      const res = await login(username, password);
      setToken(res.token);
      onSuccess();
    } catch (ex) {
      setErr(String(ex));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="login-wrap">
      <form className="panel login-panel" onSubmit={submit}>
        <h1>GrayDraft Console</h1>
        <p className="muted">使用 Admin 账号登录（需 Core 配置 JWT_SECRET）</p>
        <label>用户名</label>
        <input value={username} onChange={(e) => setUsername(e.target.value)} autoComplete="username" />
        <label>密码</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />
        {err && <p className="error">{err}</p>}
        <button className="primary" type="submit" disabled={loading}>
          {loading ? "登录中…" : "登录"}
        </button>
      </form>
    </div>
  );
}

function Dashboard() {
  const [health, setHealth] = useState<Record<string, string>>({});
  const [overview, setOverview] = useState<Record<string, unknown> | null>(null);
  const [err, setErr] = useState("");

  useEffect(() => {
    (async () => {
      try {
        const [h, o] = await Promise.all([fetchHealth(), fetchOverview()]);
        setHealth(h as Record<string, string>);
        setOverview(o);
      } catch (e) {
        setErr(String(e));
      }
    })();
  }, []);

  const catalog = (overview?.catalog as Record<string, unknown>[]) ?? [];
  const docs = (overview?.documents as Record<string, unknown>[]) ?? [];
  const ugc = (overview?.ugc_by_status as Record<string, number>) ?? {};

  return (
    <>
      <div className="panel">
        <h2>服务健康</h2>
        {err && <p className="error">{err}</p>}
        <div className="grid">
          {["status", "database", "nats", "cache", "answer_cache"].map((k) => (
            <div key={k} className={`stat ${health[k] === "ok" || health[k] === "jetstream" ? "ok" : health[k] === "unavailable" ? "err" : ""}`}>
              <div className="label">{k}</div>
              <div className="value">{health[k] ?? "—"}</div>
            </div>
          ))}
        </div>
      </div>
      <div className="panel">
        <h2>目录版本（缓存失效）</h2>
        <table>
          <thead>
            <tr>
              <th>kb_type</th>
              <th>version</th>
              <th>latest_doc_updated_at</th>
            </tr>
          </thead>
          <tbody>
            {catalog.map((c) => (
              <tr key={String(c.kb_type)}>
                <td>{String(c.kb_type)}</td>
                <td>{String(c.version)}</td>
                <td>{String(c.latest_doc_updated_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <h2>文档统计</h2>
        <table>
          <thead>
            <tr>
              <th>kb_type</th>
              <th>文档数</th>
              <th>index_ready</th>
              <th>chunks</th>
            </tr>
          </thead>
          <tbody>
            {docs.map((d) => (
              <tr key={String(d.kb_type)}>
                <td>{String(d.kb_type)}</td>
                <td>{String(d.total_docs)}</td>
                <td>{String(d.index_ready_docs)}</td>
                <td>{String(d.total_chunks)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="panel">
        <h2>UGC 任务分布</h2>
        <div className="grid">
          {Object.entries(ugc).map(([st, n]) => (
            <div key={st} className="stat">
              <div className="label">{st}</div>
              <div className="value">{n}</div>
            </div>
          ))}
        </div>
      </div>
    </>
  );
}

function UGCPage() {
  const [status, setStatus] = useState("");
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [err, setErr] = useState("");
  const [form, setForm] = useState({
    kb_type: "gd_console_demo",
    title: "",
    content: "",
  });
  const [submitResult, setSubmitResult] = useState("");

  const load = async () => {
    try {
      const data = await fetchUGCTasks(status);
      setItems(data.items ?? []);
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  useEffect(() => {
    load();
  }, [status]);

  const submit = async () => {
    try {
      const res = await ugcNotify(form);
      setSubmitResult(JSON.stringify(res, null, 2));
      load();
    } catch (e) {
      setSubmitResult(String(e));
    }
  };

  return (
    <>
      <div className="panel">
        <h2>投递 UGC</h2>
        <div className="row">
          <div>
            <label>kb_type（工作区分区 gd_*）</label>
            <input
              value={form.kb_type}
              onChange={(e) => setForm({ ...form, kb_type: e.target.value })}
              placeholder="gd_console_demo"
            />
          </div>
        </div>
        <label>标题</label>
        <input value={form.title} onChange={(e) => setForm({ ...form, title: e.target.value })} />
        <label>正文</label>
        <textarea rows={4} value={form.content} onChange={(e) => setForm({ ...form, content: e.target.value })} />
        <button className="primary" onClick={submit}>
          提交 notify
        </button>
        {submitResult && <pre>{submitResult}</pre>}
      </div>
      <div className="panel">
        <h2>任务列表</h2>
        <div className="row">
          <div>
            <label>状态筛选</label>
            <select value={status} onChange={(e) => setStatus(e.target.value)}>
              <option value="">全部</option>
              <option value="pending">pending</option>
              <option value="processing">processing</option>
              <option value="completed">completed</option>
              <option value="failed">failed</option>
            </select>
          </div>
          <div style={{ alignSelf: "flex-end" }}>
            <button className="primary" onClick={load}>
              刷新
            </button>
          </div>
        </div>
        {err && <p className="error">{err}</p>}
        <table>
          <thead>
            <tr>
              <th>task_id</th>
              <th>kb_type</th>
              <th>title</th>
              <th>status</th>
              <th>chunks</th>
              <th>updated</th>
            </tr>
          </thead>
          <tbody>
            {items.map((t) => (
              <tr key={String(t.task_id)}>
                <td title={String(t.task_id)}>{String(t.task_id).slice(0, 8)}…</td>
                <td>{String(t.kb_type)}</td>
                <td>{String(t.title)}</td>
                <td>
                  <span className={`badge ${String(t.status)}`}>{String(t.status)}</span>
                </td>
                <td>{String(t.chunk_count)}</td>
                <td>{String(t.updated_at)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </>
  );
}

function DLQPage() {
  const [stats, setStats] = useState<Record<string, unknown> | null>(null);
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [selected, setSelected] = useState<Set<number>>(new Set());
  const [actionResult, setActionResult] = useState("");
  const [err, setErr] = useState("");

  const load = async () => {
    try {
      const [s, m] = await Promise.all([fetchDLQStats(), fetchDLQMessages(50)]);
      setStats(s);
      setItems(m.items ?? []);
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  useEffect(() => {
    load();
  }, []);

  const toggle = (seq: number) => {
    setSelected((prev) => {
      const next = new Set(prev);
      if (next.has(seq)) next.delete(seq);
      else next.add(seq);
      return next;
    });
  };

  const runReplay = async () => {
    const seqs = [...selected];
    if (!seqs.length) return;
    try {
      const res = await replayDLQ(seqs);
      setActionResult(JSON.stringify(res, null, 2));
      setSelected(new Set());
      load();
    } catch (e) {
      setActionResult(String(e));
    }
  };

  const runDiscard = async () => {
    const seqs = [...selected];
    if (!seqs.length) return;
    if (!confirm(`确认丢弃 ${seqs.length} 条 DLQ 消息？`)) return;
    try {
      const res = await discardDLQ(seqs);
      setActionResult(JSON.stringify(res, null, 2));
      setSelected(new Set());
      load();
    } catch (e) {
      setActionResult(String(e));
    }
  };

  return (
    <>
      <div className="panel">
        <h2>JetStream DLQ（GRAYDRAFT_UGC_DLQ）</h2>
        {err && <p className="error">{err}</p>}
        {stats && (
          <div className="grid">
            <div className="stat">
              <div className="label">messages</div>
              <div className="value">{String(stats.messages)}</div>
            </div>
            <div className="stat">
              <div className="label">stream</div>
              <div className="value" style={{ fontSize: "0.85rem" }}>
                {String(stats.stream)}
              </div>
            </div>
            <div className="stat">
              <div className="label">seq range</div>
              <div className="value">
                {String(stats.first_seq)} – {String(stats.last_seq)}
              </div>
            </div>
          </div>
        )}
        <button className="primary" onClick={load} style={{ marginTop: "0.5rem" }}>
          刷新
        </button>
      </div>
      <div className="panel">
        <h2>死信消息</h2>
        <div className="row" style={{ marginBottom: "0.75rem" }}>
          <button className="primary" onClick={runReplay} disabled={selected.size === 0}>
            重放选中 ({selected.size})
          </button>
          <button className="primary" onClick={runDiscard} disabled={selected.size === 0}>
            丢弃选中
          </button>
        </div>
        <table>
          <thead>
            <tr>
              <th></th>
              <th>seq</th>
              <th>reason</th>
              <th>task_id</th>
              <th>kb_type</th>
              <th>title</th>
              <th>time</th>
            </tr>
          </thead>
          <tbody>
            {items.map((m) => {
              const seq = Number(m.sequence);
              return (
                <tr key={seq}>
                  <td>
                    <input type="checkbox" checked={selected.has(seq)} onChange={() => toggle(seq)} />
                  </td>
                  <td>{seq}</td>
                  <td>{String(m.dlq_reason || "—")}</td>
                  <td title={String(m.task_id)}>{m.task_id ? String(m.task_id).slice(0, 8) + "…" : "—"}</td>
                  <td>{String(m.kb_type || "—")}</td>
                  <td>{String(m.title || "—")}</td>
                  <td>{String(m.time || "")}</td>
                </tr>
              );
            })}
          </tbody>
        </table>
        {items.length === 0 && <p style={{ color: "var(--muted)" }}>暂无 DLQ 消息</p>}
      </div>
      {actionResult && (
        <div className="panel">
          <h2>操作结果</h2>
          <pre>{actionResult}</pre>
        </div>
      )}
    </>
  );
}

function ChunksPage() {
  const [kbType, setKbType] = useState("");
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [total, setTotal] = useState(0);
  const [err, setErr] = useState("");

  const load = async () => {
    try {
      const data = await fetchAdminChunks(kbType, 50, 0);
      setItems(data.items ?? []);
      setTotal(data.total ?? 0);
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  useEffect(() => {
    load();
  }, [kbType]);

  return (
    <div className="panel">
      <h2>知识条目（chunks）</h2>
      <div className="row">
        <div>
          <label>kb_type 筛选</label>
          <input value={kbType} onChange={(e) => setKbType(e.target.value)} placeholder="gd_console_demo" />
        </div>
        <div style={{ alignSelf: "flex-end" }}>
          <button className="primary" onClick={load}>
            刷新
          </button>
        </div>
      </div>
      {err && <p className="error">{err}</p>}
      <p className="muted">共 {total} 条</p>
      <table>
        <thead>
          <tr>
            <th>kb</th>
            <th>doc</th>
            <th>idx</th>
            <th>preview</th>
            <th>score</th>
            <th>ready</th>
          </tr>
        </thead>
        <tbody>
          {items.map((c) => (
            <tr key={String(c.id)}>
              <td>{String(c.kb_type)}</td>
              <td title={String(c.doc_id)}>{String(c.doc_id).slice(0, 8)}…</td>
              <td>{String(c.chunk_index)}</td>
              <td>{String(c.preview)}</td>
              <td>{String(c.quality_score)}</td>
              <td>{String(c.index_ready)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function AgentTracePage() {
  const [tenantID, setTenantID] = useState("");
  const [runID, setRunID] = useState("");
  const [trace, setTrace] = useState<Record<string, unknown> | null>(null);
  const [err, setErr] = useState("");

  const load = async () => {
    if (!tenantID.trim() || !runID.trim()) {
      setErr("tenant_id 与 run_id 均为必填");
      return;
    }
    try {
      const data = await fetchAgentRunTrace(tenantID.trim(), runID.trim());
      setTrace(data);
      setErr("");
    } catch (e) {
      setErr(String(e));
      setTrace(null);
    }
  };

  const spans = (trace?.spans as Record<string, unknown>[]) ?? [];

  return (
    <>
      <div className="panel">
        <h2>Agent Run Trace 排障</h2>
        <p className="muted">按 tenant_id + run_id 查看 Run/Step/Tool span 时间线与耗时。</p>
        {err && <p className="error">{err}</p>}
        <label>tenant_id</label>
        <input value={tenantID} onChange={(e) => setTenantID(e.target.value)} placeholder="tn_..." />
        <label>run_id</label>
        <input value={runID} onChange={(e) => setRunID(e.target.value)} placeholder="uuid" />
        <button className="primary" onClick={load}>
          查询 Trace
        </button>
      </div>
      {trace && (
        <div className="panel">
          <h2>
            Trace {String(trace.trace_id || "—")} · Run {String(trace.run_id)}
          </h2>
          <p className="muted">policy_version: {String(trace.policy_version || "—")}</p>
          <table>
            <thead>
              <tr>
                <th>seq</th>
                <th>kind</th>
                <th>event</th>
                <th>tool</th>
                <th>latency</th>
                <th>status</th>
                <th>error</th>
              </tr>
            </thead>
            <tbody>
              {spans.map((s) => (
                <tr key={String(s.seq)}>
                  <td>{String(s.seq)}</td>
                  <td>{String(s.kind)}</td>
                  <td>{String(s.event_type)}</td>
                  <td>{String(s.tool_name || "—")}</td>
                  <td>{s.latency_ms != null ? `${String(s.latency_ms)}ms` : "—"}</td>
                  <td>
                    <span className={`badge ${String(s.status) === "error" ? "failed" : "completed"}`}>
                      {String(s.status)}
                    </span>
                  </td>
                  <td title={String(s.error || "")}>{String(s.error || "—")}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {spans.length === 0 && <p className="muted">暂无 span 事件</p>}
        </div>
      )}
    </>
  );
}

function ApprovalsPage() {
  const [tenantID, setTenantID] = useState("tenant_edu");
  const [items, setItems] = useState<Record<string, unknown>[]>([]);
  const [err, setErr] = useState("");

  const load = async () => {
    try {
      const data = await fetchToolApprovals(tenantID.trim(), "pending");
      setItems((data.items as Record<string, unknown>[]) ?? []);
      setErr("");
    } catch (e) {
      setErr(String(e));
      setItems([]);
    }
  };

  const decide = async (runID: string, callID: string, status: "approved" | "rejected") => {
    try {
      await decideToolApproval(tenantID.trim(), runID, callID, status);
      await load();
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <>
      <div className="panel">
        <h2>R2 工具审批队列</h2>
        <p className="muted">处理 Agent Runtime 触发的 `tool.approval_required` 待办。</p>
        {err && <p className="error">{err}</p>}
        <label>tenant_id</label>
        <input value={tenantID} onChange={(e) => setTenantID(e.target.value)} />
        <button className="primary" onClick={load}>
          刷新待审批
        </button>
      </div>
      <div className="panel">
        <table>
          <thead>
            <tr>
              <th>run_id</th>
              <th>call_id</th>
              <th>tool</th>
              <th>actor</th>
              <th>expires</th>
              <th>actions</th>
            </tr>
          </thead>
          <tbody>
            {items.map((it) => (
              <tr key={`${String(it.run_id)}-${String(it.call_id)}`}>
                <td title={String(it.run_id)}>{String(it.run_id).slice(0, 8)}…</td>
                <td>{String(it.call_id)}</td>
                <td>{String(it.tool_name)}</td>
                <td>{String(it.actor_id)}</td>
                <td>{String(it.expires_at)}</td>
                <td>
                  <button onClick={() => decide(String(it.run_id), String(it.call_id), "approved")}>批准</button>
                  <button onClick={() => decide(String(it.run_id), String(it.call_id), "rejected")}>拒绝</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {items.length === 0 && <p className="muted">暂无待审批项</p>}
      </div>
    </>
  );
}

function CostEvalPage() {
  const [tenantID, setTenantID] = useState("");
  const [hours, setHours] = useState("24");
  const [cost, setCost] = useState<Record<string, unknown> | null>(null);
  const [evals, setEvals] = useState<Record<string, unknown> | null>(null);
  const [quality, setQuality] = useState<Record<string, unknown> | null>(null);
  const [useOracle, setUseOracle] = useState(false);
  const [err, setErr] = useState("");
  const [running, setRunning] = useState(false);

  const loadCost = async () => {
    try {
      const data = await fetchCostSummary(tenantID.trim(), Number(hours) || 24);
      setCost(data);
      setErr("");
    } catch (e) {
      setErr(String(e));
      setCost(null);
    }
  };

  const loadEvals = async () => {
    try {
      const [list, qual] = await Promise.all([
        fetchAgentEvals(tenantID.trim()),
        fetchAgentEvalQuality(tenantID.trim()),
      ]);
      setEvals(list);
      setQuality(qual);
      setErr("");
    } catch (e) {
      setErr(String(e));
      setEvals(null);
      setQuality(null);
    }
  };

  const triggerEval = async () => {
    setRunning(true);
    try {
      await runAgentEval({
        tenant_id: tenantID.trim() || undefined,
        use_oracle: useOracle,
      });
      await loadEvals();
      setErr("");
    } catch (e) {
      setErr(String(e));
    } finally {
      setRunning(false);
    }
  };

  const rows = (cost?.rows as Record<string, unknown>[]) ?? [];
  const evalItems = (evals?.items as Record<string, unknown>[]) ?? [];
  const categories = (quality?.categories as Record<string, unknown>[]) ?? [];
  const metrics = (quality?.metrics as Record<string, unknown>) ?? {};

  return (
    <>
      <div className="panel">
        <h2>模型成本看板</h2>
        <p className="muted">按 tenant / operation / model 汇总 LLM、Embedding、Rerank token 消耗。</p>
        {err && <p className="error">{err}</p>}
        <label>tenant_id（可选）</label>
        <input value={tenantID} onChange={(e) => setTenantID(e.target.value)} placeholder="tn_..." />
        <label>统计窗口（小时）</label>
        <input value={hours} onChange={(e) => setHours(e.target.value)} />
        <label className="inline">
          <input type="checkbox" checked={useOracle} onChange={(e) => setUseOracle(e.target.checked)} />
          Oracle 冒烟（跳过真实 Agent 回放）
        </label>
        <button className="primary" onClick={loadCost}>
          刷新成本
        </button>
        <button onClick={loadEvals}>
          刷新评估记录
        </button>
        <button className="primary" onClick={triggerEval} disabled={running}>
          {running ? "评估中…" : "运行 Golden 评估"}
        </button>
      </div>
      {cost && (
        <div className="panel">
          <h2>
            成本汇总 · 今日 tokens: {String(cost.daily_tokens ?? 0)}
            {Number(cost.daily_token_limit) > 0 && (
              <span className="muted"> / 限额 {String(cost.daily_token_limit)}</span>
            )}
          </h2>
          {cost.budget_exceeded === true && <p className="error">今日 token 预算已超限</p>}
          <table>
            <thead>
              <tr>
                <th>operation</th>
                <th>provider</th>
                <th>model</th>
                <th>tenant</th>
                <th>agent</th>
                <th>in</th>
                <th>out</th>
                <th>calls</th>
                <th>avg_ms</th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r, i) => (
                <tr key={`${String(r.operation)}-${i}`}>
                  <td>{String(r.operation)}</td>
                  <td>{String(r.provider)}</td>
                  <td>{String(r.model)}</td>
                  <td>{String(r.tenant_id || "—")}</td>
                  <td>{String(r.agent_id || "—")}</td>
                  <td>{String(r.input_tokens)}</td>
                  <td>{String(r.output_tokens)}</td>
                  <td>{String(r.calls)}</td>
                  <td>{Number(r.avg_latency_ms).toFixed(1)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {rows.length === 0 && <p className="muted">暂无成本数据</p>}
        </div>
      )}
      {quality && (
        <div className="panel">
          <h2>
            质量指标 · 近 7 天平均通过率 {(Number(quality.avg_pass_rate) * 100).toFixed(1)}%
          </h2>
          <p className="muted">
            工具成功率 {(Number(metrics.tool_success_rate) * 100).toFixed(1)}% · 拒答准确率{" "}
            {(Number(metrics.refusal_accuracy) * 100).toFixed(1)}% · 最新评估{" "}
            {String(quality.latest_eval_id || "—").slice(0, 8)}
          </p>
          <table>
            <thead>
              <tr>
                <th>category</th>
                <th>passed</th>
                <th>total</th>
                <th>pass_rate</th>
              </tr>
            </thead>
            <tbody>
              {categories.map((c) => (
                <tr key={String(c.category)}>
                  <td>{String(c.category)}</td>
                  <td>{String(c.passed)}</td>
                  <td>{String(c.total)}</td>
                  <td>{(Number(c.pass_rate) * 100).toFixed(1)}%</td>
                </tr>
              ))}
            </tbody>
          </table>
          {categories.length === 0 && <p className="muted">暂无分类质量数据</p>}
        </div>
      )}
      {evals && (
        <div className="panel">
          <h2>Agent 评估记录</h2>
          <table>
            <thead>
              <tr>
                <th>eval_id</th>
                <th>tenant</th>
                <th>pass_rate</th>
                <th>passed</th>
                <th>total</th>
                <th>dataset</th>
                <th>created</th>
              </tr>
            </thead>
            <tbody>
              {evalItems.map((e) => (
                <tr key={String(e.eval_id)}>
                  <td title={String(e.eval_id)}>{String(e.eval_id).slice(0, 8)}…</td>
                  <td>{String(e.tenant_id)}</td>
                  <td>{(Number(e.pass_rate) * 100).toFixed(1)}%</td>
                  <td>{String(e.passed)}</td>
                  <td>{String(e.total)}</td>
                  <td title={String(e.dataset_path)}>{String(e.dataset_path).split("/").pop()}</td>
                  <td>{String(e.created_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
          {evalItems.length === 0 && <p className="muted">暂无评估记录</p>}
        </div>
      )}
    </>
  );
}

function RAGPage() {
  const [query, setQuery] = useState("什么是 index_ready");
  const [kbType, setKbType] = useState("gd_console_demo");
  const [classifyOut, setClassifyOut] = useState("");
  const [searchOut, setSearchOut] = useState("");
  const [chatLog, setChatLog] = useState<string[]>([]);
  const [err, setErr] = useState("");

  const runClassify = async () => {
    try {
      const res = await classifyPreview(query);
      setClassifyOut(JSON.stringify(res, null, 2));
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  const runSearch = async () => {
    try {
      const res = await search({ query, kb_type: kbType, top_k: 5 });
      setSearchOut(JSON.stringify(res, null, 2));
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  const runChat = async () => {
    setChatLog([]);
    try {
      await streamChat({ query, kb_type: kbType, top_k: 5 }, (ev) => {
        setChatLog((prev) => [...prev, JSON.stringify(ev)]);
      });
      setErr("");
    } catch (e) {
      setErr(String(e));
    }
  };

  return (
    <>
      <div className="panel">
        <h2>RAG 调试台</h2>
        {err && <p className="error">{err}</p>}
        <label>query</label>
        <textarea rows={2} value={query} onChange={(e) => setQuery(e.target.value)} />
        <label>kb_type（工作区分区 gd_*）</label>
        <input value={kbType} onChange={(e) => setKbType(e.target.value)} placeholder="gd_console_demo" />
        <div className="row">
          <button className="primary" onClick={runClassify}>
            分类预览
          </button>
          <button className="primary" onClick={runSearch}>
            检索
          </button>
          <button className="primary" onClick={runChat}>
            Chat 流式
          </button>
        </div>
      </div>
      {classifyOut && (
        <div className="panel">
          <h2>分类结果</h2>
          <pre>{classifyOut}</pre>
        </div>
      )}
      {searchOut && (
        <div className="panel">
          <h2>检索结果</h2>
          <pre>{searchOut}</pre>
        </div>
      )}
      {chatLog.length > 0 && (
        <div className="panel">
          <h2>Chat SSE 事件</h2>
          <pre>{chatLog.join("\n")}</pre>
        </div>
      )}
    </>
  );
}
