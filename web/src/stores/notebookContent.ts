import { defineStore } from 'pinia'

/** 笔记 HTML 文档变更后 bump，供 NotebookHtmlPane 等重新从 localStorage 渲染 */
export const useNotebookContentStore = defineStore('notebookContent', {
  state: () => ({ revision: 0 }),
  actions: {
    bump() {
      this.revision++
    },
  },
})
