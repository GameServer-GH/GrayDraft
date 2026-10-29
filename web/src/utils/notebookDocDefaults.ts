/** 笔记 HTML 文档默认值与类型（避免 notebook-html ↔ doc cache 循环依赖）。 */
export type NotebookDocument = {
  html: string
  css: string
}

export function escapeAttrForNotebook(s: string): string {
  return s.replace(/&/g, '&amp;').replace(/"/g, '&quot;').replace(/</g, '&lt;')
}

export function defaultNotebookDocument(notebookId: string): NotebookDocument {
  return {
    html: `<div data-notebook-id="${escapeAttrForNotebook(notebookId)}" class="gd-nb-root"></div>`,
    css: `.gd-nb-root{min-height:48px;box-sizing:border-box;}`,
  }
}
