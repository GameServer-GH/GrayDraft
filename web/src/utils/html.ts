/** 将 HTML 转为纯文本，用于 Agent 气泡展示（避免直接渲染未消毒 HTML） */
export function htmlToPlain(html: string | null | undefined): string {
  if (html == null || !String(html).trim()) return ''
  try {
    const doc = new DOMParser().parseFromString(String(html), 'text/html')
    return doc.body.textContent?.trim() ?? ''
  } catch {
    return String(html).replace(/<[^>]+>/g, ' ').trim()
  }
}
