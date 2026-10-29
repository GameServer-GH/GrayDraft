/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** API 服务根（如 https://api.example.com）；开发留空则使用同域 `/api`（配合 Vite 代理） */
  readonly VITE_API_BASE_URL?: string
  /** WebSocket 完整地址，如 wss://api.example.com/ws；未使用可留空 */
  readonly VITE_WS_URL?: string
  /** OSS / CDN 根 URL；未使用可留空 */
  readonly VITE_OSS_BASE_URL?: string
  /** JWT 相关配置（勿将服务端私钥写入前端） */
  readonly VITE_JWT_KEY?: string
  /** 默认语言 BCP 47，如 zh-CN */
  readonly VITE_DEFAULT_LOCALE?: string
  /** 语言包帮助文档 URL 或路径 */
  readonly VITE_LANGUAGE_PACK_HELP_URL?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare global {
  interface Window {
    /** Wails v2 injects this in desktop builds */
    __WAILS__?: unknown
  }
}

export {}
