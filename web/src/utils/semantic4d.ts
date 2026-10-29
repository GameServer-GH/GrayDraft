import type { KnowledgeExtensions } from '@/types/knowledge'

export interface SemanticTags4D {
  domain?: string
  entities?: string[]
  scene?: string
  style?: string
}

export interface QuerySemantic4D {
  domain?: string
  entities?: string[]
  scene?: string
  style?: string
  method?: string
}

export function parseSemanticTags4D(ext: KnowledgeExtensions): SemanticTags4D | null {
  const raw = ext.semantic_tags_4d
  if (!raw || typeof raw !== 'object' || Array.isArray(raw)) return null
  const o = raw as Record<string, unknown>
  const entities = Array.isArray(o.entities)
    ? o.entities.map((e) => String(e).trim()).filter(Boolean)
    : undefined
  const tags: SemanticTags4D = {
    domain: typeof o.domain === 'string' ? o.domain.trim() : undefined,
    scene: typeof o.scene === 'string' ? o.scene.trim() : undefined,
    style: typeof o.style === 'string' ? o.style.trim() : undefined,
    entities: entities?.length ? entities : undefined,
  }
  if (!tags.domain && !tags.scene && !tags.style && !tags.entities?.length) return null
  return tags
}

export function formatSemanticTags4D(tags: SemanticTags4D): string {
  const parts: string[] = []
  if (tags.domain) parts.push(`领域:${tags.domain}`)
  if (tags.scene) parts.push(`场景:${tags.scene}`)
  if (tags.style) parts.push(`形态:${tags.style}`)
  if (tags.entities?.length) parts.push(`实体:${tags.entities.join('、')}`)
  return parts.join(' · ')
}

export function formatQuerySemantic4D(q: QuerySemantic4D | undefined | null): string {
  if (!q) return ''
  return formatSemanticTags4D(q)
}

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

export function formatRetrievalTrace(trace: RetrievalTrace | undefined | null): string {
  if (!trace) return ''
  const parts: string[] = []
  if (trace.waterfall_tier != null && trace.waterfall_tier > 0) {
    parts.push(`瀑布 T${trace.waterfall_tier}`)
  } else if (trace.waterfall_suffix) {
    parts.push(`瀑布 ${trace.waterfall_suffix}`)
  }
  if (trace.query_4d_method) parts.push(`Q4D:${trace.query_4d_method}`)
  if (trace.scene_original || trace.scene_aligned) {
    const s = trace.scene_aligned || trace.scene_original
    if (trace.scene_original && trace.scene_aligned && trace.scene_original !== trace.scene_aligned) {
      parts.push(`场景 ${trace.scene_original}→${trace.scene_aligned}`)
    } else if (s) {
      parts.push(`场景:${s}`)
    }
  }
  if (trace.entity_align) parts.push(`实体对齐:${trace.entity_align}`)
  if (trace.entities_final?.length) {
    parts.push(`实体:${trace.entities_final.join('、')}`)
  }
  return parts.join(' · ')
}

export function hasSemanticTags4D(ext: KnowledgeExtensions): boolean {
  return parseSemanticTags4D(ext) != null
}
