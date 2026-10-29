import { computed, ref, watch } from 'vue'
import { retrievalSearch } from '@/services/knowledge'
import { buildRelatedQuery, parseExtensions } from '@/utils/knowledgeLibrary'
import { formatNexusError } from '@/utils/nexusError'
import type { KnowledgeEntryDetail } from '@/types/knowledge'

export interface RelatedDocument {
  docId: string
  title: string
  score: number
}

export function useKnowledgeRelated(activeDocId: () => string | null, detail: () => KnowledgeEntryDetail | null) {
  const loading = ref(false)
  const error = ref<string | null>(null)
  const related = ref<RelatedDocument[]>([])

  async function load() {
    const id = activeDocId()
    const d = detail()
    if (!id || !d) {
      related.value = []
      return
    }
    loading.value = true
    error.value = null
    try {
      const ext = parseExtensions(d.extensions)
      const query = buildRelatedQuery(d.title, d.content, ext)
      const res = await retrievalSearch({ query, top_k: 8, skip_cache: true })
      const byDoc = new Map<string, RelatedDocument>()
      for (const c of res.chunks ?? []) {
        if (c.doc_id === id) continue
        const title = (c.doc_title || c.doc_id).trim()
        const prev = byDoc.get(c.doc_id)
        if (!prev || c.score > prev.score) {
          byDoc.set(c.doc_id, { docId: c.doc_id, title, score: c.score })
        }
      }
      related.value = [...byDoc.values()].sort((a, b) => b.score - a.score).slice(0, 5)
    } catch (e) {
      error.value = formatNexusError(e)
      related.value = []
    } finally {
      loading.value = false
    }
  }

  watch(
    () => [activeDocId(), detail()?.id, detail()?.updated_at] as const,
    () => {
      void load()
    },
    { immediate: true },
  )

  const hasRelated = computed(() => related.value.length > 0)

  return { loading, error, related, hasRelated, reload: load }
}
