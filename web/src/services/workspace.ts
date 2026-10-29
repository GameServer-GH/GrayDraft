import { callBackend } from '@/services/bridge'
import type { NotebookMeta } from '@/types/notebook'
import type { CanvasViewState } from '@/types/canvas'
import type { ConfiguredVendor } from '@/stores/agentModelConfig'
import type { InstalledPluginRow } from '@/types/plugin'

export type WorkspaceSettingsPayload = {
  vendors: ConfiguredVendor[]
  locale: {
    usePluginUiLanguage: boolean
    activeLanguagePluginId: string | null
  }
  canvasView: Pick<CanvasViewState, 'panX' | 'panY' | 'viewScale' | 'notebookLayouts'>
  plugins: { installed: InstalledPluginRow[] }
  /** 4.0 Agent 配置（推荐） */
  agentConfig?: {
    unstructuredPrompt?: string
    contextProfile?: WorkspaceContextProfile
  }
  /** 兼容旧版扁平字段 */
  multimodalWorkspacePrompt?: string
}

export type WorkspaceContextProfile = {
  description?: string
  entities?: string[]
  rules?: string[]
  negative_context?: string[]
}

export type OpenWorkspaceResponse = {
  created: boolean
  notebooks: NotebookMeta[]
  settings: Record<string, unknown>
  /** 1 工作区 = 1 GrayDraft kb_type；有值时启用知识库面板 API */
  nexusKbType?: string
}

export async function openWorkspace(key: string): Promise<OpenWorkspaceResponse> {
  return callBackend<OpenWorkspaceResponse>('OpenWorkspace', { key })
}

export async function saveWorkspaceSettings(settings: WorkspaceSettingsPayload): Promise<void> {
  await callBackend<{ ok?: boolean }>('SaveWorkspaceSettings', { settings })
}

export type NotebookDocumentDTO = { html: string; css: string }

export async function fetchNotebookDocument(notebookId: string): Promise<NotebookDocumentDTO> {
  return callBackend<NotebookDocumentDTO>('GetNotebookDocument', { id: notebookId })
}

export async function pushNotebookDocument(notebookId: string, doc: NotebookDocumentDTO): Promise<void> {
  await callBackend<{ ok?: boolean }>('SaveNotebookDocument', {
    id: notebookId,
    html: doc.html,
    css: doc.css,
  })
}
