/** 插件类型 — 方案 types/plugin.d.ts */
export type PluginKind = 'language-pack' | 'html-notebook' | 'agent-skill'

export interface PluginManifest {
  id: string
  version: string
  kind: PluginKind
  name: string
}

/** UI 列表项（已安装插件一行） */
export interface InstalledPluginRow extends PluginManifest {
  enabled: boolean
  /** `language-pack`：BCP 47 标签，与 manifest / 词条一致 */
  localeTag?: string
  /** `language-pack`：完整 UI 词条，须通过 `validateLocaleMessages` */
  messages?: Record<string, string>
}
