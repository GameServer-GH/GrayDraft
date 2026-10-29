import { defineStore } from 'pinia'
import { defaultNotebookDocument, type NotebookDocument } from '@/utils/notebookDocDefaults'
import { fetchNotebookDocument, pushNotebookDocument } from '@/services/workspace'
import { useNotebookContentStore } from '@/stores/notebookContent'

/** 笔记正文缓存 + 与 Go 工作区同步 */
export const useNotebookDocCacheStore = defineStore('notebookDocCache', {
  state: () => ({
    cache: {} as Record<string, NotebookDocument>,
    loaded: {} as Record<string, boolean>,
  }),
  actions: {
    clearAll() {
      this.cache = {}
      this.loaded = {}
    },
    evict(notebookId: string) {
      delete this.cache[notebookId]
      delete this.loaded[notebookId]
    },
    getReadable(notebookId: string): NotebookDocument {
      return this.cache[notebookId] ?? defaultNotebookDocument(notebookId)
    },
    setLocal(notebookId: string, doc: NotebookDocument) {
      this.cache[notebookId] = { html: doc.html, css: doc.css }
      this.loaded[notebookId] = true
      useNotebookContentStore().bump()
    },
    async ensureLoaded(notebookId: string) {
      if (!notebookId) return
      if (this.loaded[notebookId]) return
      const row = await fetchNotebookDocument(notebookId)
      const def = defaultNotebookDocument(notebookId)
      const html = typeof row.html === 'string' && row.html.trim() ? row.html : def.html
      const css = typeof row.css === 'string' ? row.css : ''
      this.cache[notebookId] = { html, css }
      this.loaded[notebookId] = true
      useNotebookContentStore().bump()
    },
    async flushRemote(notebookId: string) {
      const doc = this.cache[notebookId] ?? defaultNotebookDocument(notebookId)
      await pushNotebookDocument(notebookId, doc)
    },
    async saveLocalAndRemote(notebookId: string, doc: NotebookDocument) {
      this.setLocal(notebookId, doc)
      await this.flushRemote(notebookId)
    },
  },
})
