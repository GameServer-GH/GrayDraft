import { authHeaders, clearToken } from "./auth";

const API = "";

async function apiFetch(input: string, init?: RequestInit): Promise<Response> {
  const headers = new Headers(init?.headers);
  for (const [k, v] of Object.entries(authHeaders())) {
    headers.set(k, v);
  }
  const r = await fetch(input, { ...init, headers });
  if (r.status === 401) {
    clearToken();
  }
  return r;
}

export type Health = {
  status: string;
  database?: string;
  nats?: string;
  cache?: string;
  answer_cache?: string;
};

export async function fetchHealth(): Promise<Health> {
  const r = await apiFetch(`${API}/api/health`);
  return r.json();
}

export async function fetchAdminChunks(kbType = "", limit = 50, offset = 0) {
  const q = new URLSearchParams({ limit: String(limit), offset: String(offset) });
  if (kbType) q.set("kb_type", kbType);
  const r = await apiFetch(`${API}/api/v1/admin/chunks?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchOverview() {
  const r = await apiFetch(`${API}/api/v1/admin/overview`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchUGCTasks(status = "", limit = 50) {
  const q = new URLSearchParams({ limit: String(limit) });
  if (status) q.set("status", status);
  const r = await apiFetch(`${API}/api/v1/admin/ugc/tasks?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function classifyPreview(query: string) {
  const r = await apiFetch(`${API}/api/v1/classifier/preview`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ query }),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function search(body: Record<string, unknown>) {
  const r = await apiFetch(`${API}/api/v1/retrieval/search`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchDLQStats() {
  const r = await apiFetch(`${API}/api/v1/admin/dlq/stats`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchDLQMessages(limit = 50) {
  const r = await apiFetch(`${API}/api/v1/admin/dlq/messages?limit=${limit}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function replayDLQ(sequences: number[]) {
  const r = await apiFetch(`${API}/api/v1/admin/dlq/replay`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sequences }),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function discardDLQ(sequences: number[]) {
  const r = await apiFetch(`${API}/api/v1/admin/dlq/discard`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ sequences }),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function ugcNotify(body: Record<string, unknown>) {
  const r = await apiFetch(`${API}/api/v1/ugc/notify`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchAgentRunTrace(tenantID: string, runID: string) {
  const q = new URLSearchParams({ tenant_id: tenantID });
  const r = await apiFetch(`${API}/api/v1/admin/agent-runs/${encodeURIComponent(runID)}/trace?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchCostSummary(tenantID = "", hours = 24) {
  const q = new URLSearchParams({ hours: String(hours) });
  if (tenantID) q.set("tenant_id", tenantID);
  const r = await apiFetch(`${API}/api/v1/admin/cost/summary?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchAgentEvals(tenantID = "", limit = 20) {
  const q = new URLSearchParams({ limit: String(limit) });
  if (tenantID) q.set("tenant_id", tenantID);
  const r = await apiFetch(`${API}/api/v1/admin/agent-evals?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchToolApprovals(tenantID: string, status = "pending") {
  const q = new URLSearchParams({ tenant_id: tenantID });
  if (status) q.set("status", status);
  const r = await apiFetch(`${API}/api/v1/admin/tools/approvals?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function decideToolApproval(
  tenantID: string,
  runID: string,
  callID: string,
  status: "approved" | "rejected"
) {
  const q = new URLSearchParams({ tenant_id: tenantID });
  const r = await apiFetch(`${API}/api/v1/admin/tools/approvals/${encodeURIComponent(runID)}/${encodeURIComponent(callID)}/decide?${q}`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ status, actor_id: "console-admin" }),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function fetchAgentEvalQuality(tenantID = "", days = 7) {
  const q = new URLSearchParams({ days: String(days) });
  if (tenantID) q.set("tenant_id", tenantID);
  const r = await apiFetch(`${API}/api/v1/admin/agent-evals/quality?${q}`);
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function runAgentEval(body: Record<string, unknown> = {}) {
  const r = await apiFetch(`${API}/api/v1/admin/agent-evals/run`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) throw new Error(await r.text());
  return r.json();
}

export async function streamChat(
  body: Record<string, unknown>,
  onEvent: (ev: Record<string, unknown>) => void
) {
  const r = await apiFetch(`${API}/api/v1/chat/stream`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
  });
  if (!r.ok) throw new Error(await r.text());
  const reader = r.body?.getReader();
  if (!reader) throw new Error("no stream body");
  const dec = new TextDecoder();
  let buf = "";
  while (true) {
    const { done, value } = await reader.read();
    if (done) break;
    buf += dec.decode(value, { stream: true });
    const parts = buf.split("\n\n");
    buf = parts.pop() ?? "";
    for (const part of parts) {
      for (const line of part.split("\n")) {
        if (line.startsWith("data: ")) {
          try {
            onEvent(JSON.parse(line.slice(6)));
          } catch {
            /* ignore */
          }
        }
      }
    }
  }
}
