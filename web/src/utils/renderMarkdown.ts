import MarkdownIt from 'markdown-it'
import DOMPurify from 'dompurify'
import { normalizeKBMarkdown } from '@/utils/normalizeKBMarkdown'

const md = new MarkdownIt({
  html: false,
  linkify: true,
  breaks: true,
})

/** Agent 对话等场景：将 Markdown 转为可安全插入 DOM 的 HTML */
export function renderAgentMarkdown(source: string): string {
  const raw = md.render(source)
  return DOMPurify.sanitize(raw, { USE_PROFILES: { html: true } })
}

/** 知识库正文 / 问答：先规范化 KB Markdown，再渲染为 HTML */
export function renderKnowledgeMarkdown(source: string): string {
  const normalized = normalizeKBMarkdown(source)
  if (!normalized) return ''
  return renderAgentMarkdown(normalized)
}
