import { callBackend } from '@/services/bridge'
import type { NotebookMeta } from '@/types/notebook'

export async function listNotebooks(): Promise<NotebookMeta[]> {
  return callBackend<NotebookMeta[]>('ListNotebooks')
}

export async function createNotebook(body?: { title?: string }): Promise<NotebookMeta> {
  return callBackend<NotebookMeta>('CreateNotebook', body ?? {})
}

export async function deleteNotebook(body: { id: string }): Promise<void> {
  await callBackend<{ ok?: boolean }>('DeleteNotebook', body)
}

export async function renameNotebook(body: { id: string; title: string }): Promise<NotebookMeta> {
  return callBackend<NotebookMeta>('RenameNotebook', body)
}
