import type { KnowledgeDockMode } from '@/stores/agent'

function segment(s: string): string {
  return encodeURIComponent(s.trim()).slice(0, 160)
}

/** 本地历史 key：工作区_笔记(all|id)_import|chat|search */
export function buildAgentThreadKey(
  workspaceKey: string,
  notebookId: string | null | undefined,
  mode: KnowledgeDockMode,
): string {
  const ws = segment(workspaceKey)
  const nb = notebookId?.trim() ? segment(notebookId.trim()) : 'all'
  return `${ws}_${nb}_${mode}`
}

/** 服务端线程键只含笔记和模式。工作区由租户隔离，不把钥匙写入记录。 */
export function buildServerThreadKey(
  notebookId: string | null | undefined,
  mode: KnowledgeDockMode,
): string {
  const nb = notebookId?.trim() ? segment(notebookId.trim()) : 'all'
  return `${nb}_${mode}`
}
