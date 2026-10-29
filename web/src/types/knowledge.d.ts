/** 文档级 4D 语义标签（入库 extensions.semantic_tags_4d） */
export interface SemanticTags4D {
  domain?: string
  entities?: string[]
  scene?: string
  style?: string
}

/** 检索侧从 query 提取的 4D（与文档标签结构一致） */
export interface QuerySemantic4D {
  domain?: string
  entities?: string[]
  scene?: string
  style?: string
  method?: string
}

/** GrayDraft 检索瀑布与实体对齐轨迹（Search / Chat meta） */
export interface RetrievalTrace {
  waterfall_tier?: number
  waterfall_suffix?: string
  query_4d_method?: string
  scene_original?: string
  scene_aligned?: string
  style_original?: string
  style_aligned?: string
  entity_align?: string
  entities_final?: string[]
}

/** GrayDraft 文档 extensions（smart-import 锚点 + 业务自定义） */
export interface KnowledgeExtensions {
  subject?: string
  parent_title?: string
  version?: string
  doc_type?: string
  context_enhancement?: string
  search_intent?: string
  semantic_summary?: string
  global_summary?: string
  semantic_tags_4d?: SemanticTags4D
  semantic_pivot?: string
  segment_index?: number
  segment_total?: number
  segment_tags?: string[]
  line_range?: number[]
  [key: string]: unknown
}

/** GrayDraft 文档列表项（BFF ListKnowledgeEntries） */
export interface KnowledgeDocumentItem {
  id: string
  title: string
  preview: string
  index_ready: boolean
  quality_score: number
  chunk_count: number
  extensions?: KnowledgeExtensions
  updated_at: string
}

export interface KnowledgeListResponse {
  kb_type: string
  total: number
  items: KnowledgeDocumentItem[]
}

export interface CreateKnowledgeResponse {
  task_id: string
  doc_id?: string
  accepted: boolean
  channel?: string
  message?: string
}

export interface IngestTaskResponse {
  task_id?: string
  doc_id?: string
  kb_type?: string
  title?: string
  status: 'pending' | 'processing' | 'completed' | 'failed' | string
  error_message?: string
  chunk_count?: number
}

export interface SmartImportPreview {
  kb_type?: string
  title?: string
  content?: string
  action?: string
  confidence?: number
  import_allowed?: boolean
  reason?: string
}

export interface SmartImportDocAnalysis {
  subject?: string
  version?: string
  doc_type?: string
  parent_title?: string
}

export interface SmartImportItem {
  index: number
  preview: SmartImportPreview
  import_allowed?: boolean
  line_range?: [number, number]
  context_enhancement?: string
}

export interface SmartImportTaskRef {
  task_id: string
  doc_id?: string
}

export interface KnowledgeGraphNode {
  id: string
  title: string
  extensions?: KnowledgeExtensions
  embedding?: number[]
}

export interface KnowledgeGraphResponse {
  kb_type?: string
  total: number
  nodes: KnowledgeGraphNode[]
}

export interface KnowledgeChunk {
  id: string
  chunk_index: number
  content: string
  index_ready: boolean
  quality_score: number
}

export interface KnowledgeQAPair {
  question: string
  answer: string
}

export interface KnowledgeEntryDetail {
  id: string
  kb_type?: string
  title: string
  content: string
  source_url?: string
  index_ready?: boolean
  quality_score?: number
  kb_version?: number
  updated_at?: string
  extensions?: KnowledgeExtensions
  chunks?: KnowledgeChunk[]
  qa_pairs?: KnowledgeQAPair[]
}

export type KnowledgeChatMode = 'hybrid' | 'kb_only'

export interface RetrievalChunk {
  id: string
  doc_id: string
  kb_type?: string
  content: string
  score: number
  chunk_index: number
  quality_score?: number
  doc_title?: string
  source?: string
  extensions?: KnowledgeExtensions
}

export interface RetrievalSearchResponse {
  query: string
  kb_type?: string
  top_k?: number
  total: number
  strategy?: string
  cache_hit?: string
  query_intent?: string
  catalog_version?: number
  query_semantic_4d?: QuerySemantic4D
  semantic_4d_filter_applied?: boolean
  retrieval_trace?: RetrievalTrace
  chunks: RetrievalChunk[]
}

export interface ChatSource {
  chunk_id: string
  doc_id: string
  kb_type?: string
  doc_title?: string
  quality_score?: number
  extensions?: KnowledgeExtensions
}

export interface ChatCitation {
  doc_id: string
  doc_title?: string
  kb_type?: string
  chunk_ids?: string[]
  media_url?: string
  extensions?: KnowledgeExtensions
}

export interface ChatStreamEvent {
  type: 'meta' | 'thinking' | 'chunk' | 'done' | 'error'
  delta?: string
  done?: boolean
  strategy?: string
  confidence?: number
  chunks_used?: number
  chat_mode?: string
  message?: string
  citations?: ChatCitation[]
  sources?: ChatSource[]
  query_semantic_4d?: QuerySemantic4D
  semantic_4d_filter_applied?: boolean
  retrieval_trace?: RetrievalTrace
  /** Agent Loop 进度：run | step | tool | validate */
  thinking_phase?: string
  tool_name?: string
  step_id?: number
}

export interface SimilarDocument {
  doc_id: string
  title: string
  kb_type: string
  similarity: number
}

export interface SmartImportSegmentOverride {
  index: number
  line_range: [number, number]
}

export interface SmartImportResponse {
  confidence?: number
  min_confidence_required?: number
  import_allowed?: boolean
  split?: boolean
  message?: string
  doc_analysis?: SmartImportDocAnalysis
  splitting_strategy?: 'single' | 'multi_segment'
  split_method?: string
  preview?: SmartImportPreview
  items?: SmartImportItem[]
  tasks?: SmartImportTaskRef[]
  task?: SmartImportTaskRef
  skipped?: { index: number; reason: string; confidence?: number }[]
  similar_documents?: SimilarDocument[]
}
