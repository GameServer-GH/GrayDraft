import { useNotebookDocCacheStore } from '@/stores/notebookDocCache'
import type { NotebookDocument } from '@/utils/notebookDocDefaults'
import { sanitizeNotebookCss as sanitizeNotebookCssAgent } from '@/utils/notebookCssSanitize'

export type { NotebookDocument }
export { defaultNotebookDocument } from '@/utils/notebookDocDefaults'

/** 旧版 localStorage 键名（仅用于可选迁移）。 */
export function notebookHtmlStorageKey(notebookId: string) {
  return `graydraft:notebook:${notebookId}:html`
}

export function notebookDocStorageKey(notebookId: string) {
  return `graydraft:notebook:${notebookId}:doc`
}

/** open Shadow：样式仍与外部隔离，便于 DevTools 调试且与测量路径一致（见 docs/notebook-html-rendering-diff.md）。 */
export function getOrCreateNotebookShadowRoot(host: HTMLElement): ShadowRoot {
  return host.shadowRoot ?? host.attachShadow({ mode: 'open' })
}

/**
 * Agent / 提示词常用 CSS 变量名 → 应用 `tokens.css`（:root）里实际存在的变量。
 * Shadow 与外部样式隔离后，子树只会继承「宿主元素」上的自定义属性；宿主类名上又没有 --color-primary 等，
 * 若不在这里桥接，模型写的 var(--color-primary) 会大量落回 fallback，整体会像「没套上主题」一样发灰、发素。
 * 说明：--color-surface 等与 :root 同名，用 var(--color-surface, …) 读取的是继承链上的值，不会自引用死循环。
 */
const NB_AGENT_TOKEN_BRIDGE_ON_HOST = `
  --font-family-sans:ui-sans-serif,system-ui,-apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif;
  color:var(--color-text,#1c1c1e);
  font-family:var(--font-family-sans);
  --color-text-primary:var(--color-text,#1c1c1e);
  --color-text-secondary:var(--color-text-secondary,#8e8e93);
  --color-primary:var(--color-accent,#007aff);
  --color-primary-hover:#0066cc;
  --color-surface:var(--color-surface,rgba(255,255,255,.72));
  --color-border:var(--color-border,rgba(60,60,67,.12));
  --radius-md:12px;
  --spacing-3:var(--space-3,12px);
  --spacing-4:var(--space-4,16px);
  --shadow-sm:var(--shadow-float-sm,0 1px 2px rgba(0,0,0,.06));
  --shadow-glow:0 0 32px rgba(0,122,255,.2);
`.trim()

/**
 * Shadow 内首段 style：:host（含 token 桥）+ .nb-surface + 语义标签兜底排版，再拼接文档 CSS。
 */
const NB_SURFACE_BASE_TYPOGRAPHY = `
.nb-surface,:host{color:var(--color-text-primary,#1c1c1e);}
.nb-surface :where(h1,h2,h3,h4,h5,h6){font-weight:600;line-height:1.25;margin:0.35em 0;color:var(--color-text-primary,#1c1c1e);}
.nb-surface :where(p){margin:0.35em 0;line-height:1.55;color:inherit;}
.nb-surface :where(a){color:var(--color-primary,#007aff);}
.nb-surface :where(ul,ol){padding-left:1.25em;margin:0.35em 0;}
.nb-surface :where(code,pre){font-family:ui-monospace,SFMono-Regular,Menlo,Monaco,Consolas,monospace;font-size:0.9em;}
`

export function buildNotebookShadowShellCss(safeCss: string): string {
  return (
    `:host{display:block;width:100%;height:100%;overflow:hidden;box-sizing:border-box;${NB_AGENT_TOKEN_BRIDGE_ON_HOST}}
.nb-surface{width:100%;height:100%;max-width:100%;max-height:100%;box-sizing:border-box;overflow:hidden;padding:0;}
` +
      NB_SURFACE_BASE_TYPOGRAPHY +
      safeCss
  )
}

/** 与 canvas 卡片 clamp 对齐，用于 Agent 生成后按内容测量收紧 layout */
const NB_INTRINSIC_MIN_W = 240
const NB_INTRINSIC_MIN_H = 160
const NB_INTRINSIC_MAX_W = 2400
const NB_INTRINSIC_MAX_H = 1800

/**
 * 在离屏 Shadow 中按「宽 max-content」测量文档 intrinsic 尺寸（画布世界 px），
 * 供 Agent 返回后收紧笔记卡片 layout。
 */
export function measureNotebookDocumentIntrinsicPx(notebookId: string): { w: number; h: number } | null {
  if (typeof document === 'undefined') return null
  const { html, css } = readNotebookDocument(notebookId)
  const safeCss = sanitizeNotebookCssAgent(css)

  const host = document.createElement('div')
  host.setAttribute('data-nb-measure', notebookId)
  host.style.cssText =
    'position:fixed;left:0;top:0;visibility:hidden;pointer-events:none;z-index:-1;width:auto;height:auto;'
  document.body.appendChild(host)

  try {
    const sr = getOrCreateNotebookShadowRoot(host)
    while (sr.firstChild) sr.removeChild(sr.firstChild)
    const style = document.createElement('style')
    style.textContent =
      `:host{display:block;contain:content;${NB_AGENT_TOKEN_BRIDGE_ON_HOST}}
.nb-surface{box-sizing:border-box;display:flow-root;width:max-content;max-width:${NB_INTRINSIC_MAX_W}px;min-width:${NB_INTRINSIC_MIN_W}px;min-height:48px;height:auto;overflow:visible;padding:0;}
` +
      NB_SURFACE_BASE_TYPOGRAPHY +
      safeCss
    sr.appendChild(style)
    const wrap = document.createElement('div')
    wrap.className = 'nb-surface'
    wrap.innerHTML = html
    sr.appendChild(wrap)

    const rect = wrap.getBoundingClientRect()
    let w = Math.ceil(rect.width)
    let h = Math.ceil(rect.height)
    if (!Number.isFinite(w) || !Number.isFinite(h) || w < 1 || h < 1) return null
    w = Math.min(NB_INTRINSIC_MAX_W, Math.max(NB_INTRINSIC_MIN_W, w))
    h = Math.min(NB_INTRINSIC_MAX_H, Math.max(NB_INTRINSIC_MIN_H, h))
    return { w, h }
  } finally {
    host.remove()
  }
}

/** 供 CSS id / class 使用的安全片段（与 Go 侧一致） */
export function cssSafeNotebookScopeId(notebookId: string): string {
  return 'gd-nb-' + notebookId.replace(/[^a-zA-Z0-9_-]+/g, '_')
}

/** 从内存缓存读取（须先 {@link useNotebookDocCacheStore.ensureLoaded}）。 */
export function readNotebookDocument(notebookId: string): NotebookDocument {
  return useNotebookDocCacheStore().getReadable(notebookId)
}

/** 写入内存并异步同步到 Go 工作区。 */
export function writeNotebookDocument(notebookId: string, doc: NotebookDocument): void {
  const st = useNotebookDocCacheStore()
  st.setLocal(notebookId, doc)
  void st.flushRemote(notebookId)
}

/** 删除笔记后丢弃本地文档缓存（服务端在 DeleteNotebook 时已删文件）。 */
export function clearNotebookBrowserStorage(notebookId: string): void {
  useNotebookDocCacheStore().evict(notebookId)
}

export { sanitizeNotebookCss } from '@/utils/notebookCssSanitize'
