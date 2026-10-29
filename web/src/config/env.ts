/**
 * 运行时环境变量（Vite：以 VITE_ 为前缀的键会注入 import.meta.env）。
 * 见项目根目录 `.env.development`、`.env.production`。
 */

type PublicEnvKey =
  | 'VITE_API_BASE_URL'
  | 'VITE_WS_URL'
  | 'VITE_OSS_BASE_URL'
  | 'VITE_JWT_KEY'
  | 'VITE_DEFAULT_LOCALE'
  | 'VITE_LANGUAGE_PACK_HELP_URL'

function str(key: PublicEnvKey, fallback = ''): string {
  const v = import.meta.env[key]
  if (v == null) return fallback
  const s = String(v).trim()
  return s || fallback
}

/** 是否使用同域相对路径调 API（开发时常配合 Vite 代理 /api） */
export function isApiSameOrigin(): boolean {
  return str('VITE_API_BASE_URL') === ''
}

/** POST JSON-RPC 风格：`POST {origin}/api/{method}` */
export function apiUrl(method: string): string {
  const base = str('VITE_API_BASE_URL').replace(/\/+$/, '')
  if (!base) return `/api/${method}`
  return `${base}/api/${method}`
}

/** 知识库 RAG 问答 SSE（BFF 代理 GrayDraft /api/v1/chat/stream） */
export function knowledgeChatStreamUrl(): string {
  const base = str('VITE_API_BASE_URL').replace(/\/+$/, '')
  if (!base) return '/api/knowledge/chat/stream'
  return `${base}/api/knowledge/chat/stream`
}

/** WebSocket 根地址（完整 ws:// 或 wss:// URL，未启用可留空） */
export function wsUrl(): string {
  return str('VITE_WS_URL')
}

/** OSS / 静态资源 CDN 根（无尾部斜杠；未启用可留空） */
export function ossBaseUrl(): string {
  return str('VITE_OSS_BASE_URL').replace(/\/+$/, '')
}

/** 拼接 OSS 对象路径 */
export function ossUrl(path: string): string {
  const base = ossBaseUrl()
  const p = path.startsWith('/') ? path : `/${path}`
  if (!base) return p
  return `${base}${p}`
}

/**
 * 前端可用的 JWT 相关配置（若仅为服务端密钥，请勿写入前端 env）。
 * 常见用途：Authorization 头前缀后的 token 名、或公钥指纹等非机密标识。
 */
export function jwtKey(): string {
  return str('VITE_JWT_KEY')
}

/** 应用默认语言 BCP 47，如 zh-CN、en-US */
export function defaultLocale(): string {
  const v = str('VITE_DEFAULT_LOCALE', 'zh-CN')
  return v || 'zh-CN'
}

/** 语言包规范与帮助页（绝对 URL 或站内路径，如 `/language-pack-help.html`） */
export function languagePackHelpUrl(): string {
  const v = str('VITE_LANGUAGE_PACK_HELP_URL', '/language-pack-help.html').trim()
  return v || '/language-pack-help.html'
}

/** 图片识别 + 清洗端点 */
export function imageRecognizeUrl(): string {
  const base = str('VITE_API_BASE_URL').replace(/\/+$/, '')
  if (!base) return '/api/image/recognize'
  return `${base}/api/image/recognize`
}

/** 语音转写端点 */
export function speechTranscribeUrl(): string {
  const base = str('VITE_API_BASE_URL').replace(/\/+$/, '')
  if (!base) return '/api/speech/transcribe'
  return `${base}/api/speech/transcribe`
}
