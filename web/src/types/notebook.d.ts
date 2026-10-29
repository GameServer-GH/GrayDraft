/** 知识条目元数据（画布卡片与侧栏共用） */
export interface KnowledgeEntryMeta {
  id: string
  title: string
  updatedAt: string
  /** 笔记预览截图 URL；缺省时侧栏使用占位块 */
  thumbnailUrl?: string
  /** GrayDraft 正文预览（知识库面板） */
  preview?: string
  indexReady?: boolean
  qualityScore?: number
  chunkCount?: number
  /** GrayDraft extensions（分组、过滤、面包屑） */
  extensions?: import('@/types/knowledge').KnowledgeExtensions
}

/** @deprecated 使用 KnowledgeEntryMeta */
export type NotebookMeta = KnowledgeEntryMeta
