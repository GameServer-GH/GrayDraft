/**
 * 通信抽象层 — 方案「所有对 Go 的调用必须通过 src/services/bridge.ts」
 * M1 锁定接口；禁止业务代码直接调用 Wails binding 或裸 fetch 调后端。
 */
import { apiUrl } from '@/config/env'
import { getWorkspaceContextKey, setWorkspaceContextKey } from '@/services/workspaceContext'

const isDesktop = typeof window.__WAILS__ !== 'undefined'

/** 桌面端尚无密钥 UI 时，使用固定占位密钥以便与 Web 共用工作区 API。 */
if (typeof window !== 'undefined' && isDesktop) {
  setWorkspaceContextKey('__wails_local_workspace__')
}

function abortable<T>(p: Promise<T>, signal?: AbortSignal): Promise<T> {
  if (!signal) return p
  if (signal.aborted) {
    return Promise.reject(new DOMException('Aborted', 'AbortError'))
  }
  return new Promise((resolve, reject) => {
    const onAbort = () => {
      signal.removeEventListener('abort', onAbort)
      reject(new DOMException('Aborted', 'AbortError'))
    }
    signal.addEventListener('abort', onAbort)
    p.then(
      (v) => {
        signal.removeEventListener('abort', onAbort)
        resolve(v)
      },
      (e) => {
        signal.removeEventListener('abort', onAbort)
        reject(e)
      },
    )
  })
}

export async function callBackend<T>(
  method: string,
  params?: unknown,
  init?: { signal?: AbortSignal },
): Promise<T> {
  const signal = init?.signal
  if (isDesktop) {
    const fn = (window as unknown as Record<string, (p?: unknown) => Promise<T>>)[method]
    if (typeof fn !== 'function') {
      throw new Error(`Wails binding missing: ${method}`)
    }
    return abortable(fn(params), signal)
  }
  const headers: Record<string, string> = {
    'Content-Type': 'application/json',
  }
  if (method !== 'OpenWorkspace') {
    const k = getWorkspaceContextKey()
    if (k) {
      headers['X-Graydraft-Workspace-Key'] = k
    }
  }
  const res = await fetch(apiUrl(method), {
    method: 'POST',
    body: JSON.stringify(params ?? {}),
    headers,
    signal,
  })
  if (!res.ok) {
    let detail = ''
    try {
      const ct = res.headers.get('content-type') ?? ''
      if (ct.includes('application/json')) {
        const j = (await res.json()) as { error?: string }
        if (j?.error) detail = ` — ${j.error}`
      } else {
        const t = await res.text()
        if (t.trim()) detail = ` — ${t.trim().slice(0, 240)}`
      }
    } catch {
      /* ignore body parse errors */
    }
    throw new Error(`API ${method} failed: ${res.status}${detail}`)
  }
  return res.json() as Promise<T>
}

export function getIsDesktop(): boolean {
  return isDesktop
}
