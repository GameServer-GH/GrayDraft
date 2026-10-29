import { defineStore } from 'pinia'
import { getKnowledgeEntry } from '@/services/knowledge'
import type { KnowledgeEntryDetail } from '@/types/knowledge'
import { formatNexusError } from '@/utils/nexusError'

type Slot = {
  detail: KnowledgeEntryDetail | null
  loading: boolean
  error: string | null
  revision: number
}

function emptySlot(): Slot {
  return { detail: null, loading: false, error: null, revision: 0 }
}

export type CitationOpen = {
  nonce: number
  docId: string
  chunkId: string | null
  mediaUrl: string | null
}

export const useKnowledgeDetailStore = defineStore('knowledgeDetail', {
  state: () => ({
    slots: {} as Record<string, Slot>,
    citationOpen: null as CitationOpen | null,
  }),
  actions: {
    openCitation(docId: string, chunkId?: string | null, mediaUrl?: string | null) {
      const id = docId.trim()
      if (!id) return
      this.citationOpen = {
        nonce: Date.now(),
        docId: id,
        chunkId: chunkId?.trim() || null,
        mediaUrl: mediaUrl?.trim() || null,
      }
    },

    slotFor(docId: string): Slot {
      if (!this.slots[docId]) {
        this.slots[docId] = emptySlot()
      }
      return this.slots[docId]
    },

    invalidate(docId: string) {
      const s = this.slotFor(docId)
      s.detail = null
      s.revision++
    },

    invalidateAll() {
      for (const id of Object.keys(this.slots)) {
        this.invalidate(id)
      }
    },

    async load(docId: string, opts?: { force?: boolean }): Promise<KnowledgeEntryDetail | null> {
      const s = this.slotFor(docId)
      if (s.loading) return s.detail
      if (s.detail && !opts?.force) return s.detail

      s.loading = true
      s.error = null
      try {
        const detail = await getKnowledgeEntry(docId)
        s.detail = detail
        s.revision++
        return detail
      } catch (e) {
        s.error = formatNexusError(e)
        s.detail = null
        return null
      } finally {
        s.loading = false
      }
    },
  },
})
