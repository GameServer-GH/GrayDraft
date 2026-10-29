/**
 * 笔记 HTML 内联样式消毒（遗留本地 HTML 预览仍使用）。
 * 修改时请同时更新 Go 侧与 src/utils/notebookCssSanitize.ts。
 */

const reCSSImport = /@import[^;]+;/gi
const reCSSFontFace = /@font-face\s*\{[^}]*\}/gi
const reCSSPositionFixed = /\bposition\s*:\s*(?:fixed|sticky)\b/gi
const reCSSZIndex = /\bz-index\s*:\s*([+-]?\d+)/gi
const reCSSContentURL = /\bcontent\s*:\s*[^;]*url\s*\([^)]*\)[^;]*/gi
const reCSSURLJS = /url\s*\(\s*javascript[^)]*\)/gi
const reCSSURLDataFont = /url\s*\(\s*data:\s*font[^)]*\)/gi
const reCSSJSProto = /javascript\s*:/gi
const reCSSExpression = /expression\s*\(/gi
const reCSSMozBinding = /-moz-binding\s*:[^;};]+;?/gi

function capZIndexInCss(css: string): string {
  return css.replace(reCSSZIndex, (full, numStr: string) => {
    const n = parseInt(numStr, 10)
    if (!Number.isFinite(n) || n <= 9999) return full
    return 'z-index:9999'
  })
}

/** 与 Go SanitizeAgentCSS 对齐：拦截 @import、@font-face、expression、moz-binding、危险 url、裸 javascript: 协议、视觉劫持等。 */
export function sanitizeNotebookCss(css: string): string {
  let s = css
  s = s.replace(reCSSImport, '/*@import removed*/')
  s = s.replace(reCSSFontFace, '/*font-face block removed*/')
  s = s.replace(reCSSExpression, '/*expr*/(')
  s = s.replace(reCSSMozBinding, '/*-moz-binding removed*/')
  s = s.replace(reCSSURLJS, 'url("")')
  s = s.replace(reCSSURLDataFont, 'none')
  s = s.replace(reCSSJSProto, '/*js*/:')
  s = s.replace(reCSSPositionFixed, 'position:relative')
  s = s.replace(reCSSContentURL, 'content:none')
  s = capZIndexInCss(s)
  return s.trim()
}
