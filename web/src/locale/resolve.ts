import { defaultLocale } from '@/config/env'
import { zhCNMessages } from '@/locale/builtins/zh-CN'
import { enUSMessages } from '@/locale/builtins/en-US'
import type { LocaleMessages } from '@/locale/schema'
import { validateLocaleMessages } from '@/locale/schema'
import type { InstalledPluginRow } from '@/types/plugin'

const BUILTIN_TABLE: Record<string, LocaleMessages> = {
  'zh-cn': zhCNMessages,
  'en-us': enUSMessages,
}

/** BCP 47 规范化：zh_cn → zh-CN */
export function normalizeLocaleTag(tag: string): string {
  const s = tag.trim().replace(/_/g, '-')
  if (!s) return 'zh-CN'
  const parts = s.split('-')
  const lang = parts[0]!.toLowerCase()
  if (parts.length === 1) return lang
  const rest = parts.slice(1).map((p) => (p.length === 2 ? p.toUpperCase() : p.toLowerCase()))
  return [lang, ...rest].join('-')
}

export function builtinMessagesForTag(tag: string): LocaleMessages {
  const key = normalizeLocaleTag(tag).toLowerCase()
  return BUILTIN_TABLE[key] ?? zhCNMessages
}

export function builtinMessagesForDefaultEnv(): LocaleMessages {
  return builtinMessagesForTag(defaultLocale())
}

export function messagesFromLanguagePackRow(row: InstalledPluginRow): LocaleMessages | null {
  if (row.kind !== 'language-pack' || !row.messages) return null
  const v = validateLocaleMessages(row.messages)
  if (!v.ok) return null
  return row.messages as LocaleMessages
}

export function resolveUiMessages(opts: {
  usePluginUiLanguage: boolean
  activeLanguagePluginId: string | null
  installed: InstalledPluginRow[]
}): LocaleMessages {
  const { usePluginUiLanguage, activeLanguagePluginId, installed } = opts
  if (!usePluginUiLanguage || !activeLanguagePluginId) {
    return builtinMessagesForDefaultEnv()
  }
  const row = installed.find((p) => p.id === activeLanguagePluginId)
  if (!row || row.kind !== 'language-pack' || !row.enabled) {
    return builtinMessagesForDefaultEnv()
  }
  return messagesFromLanguagePackRow(row) ?? builtinMessagesForDefaultEnv()
}

export function documentLangFromState(opts: {
  usePluginUiLanguage: boolean
  activeLanguagePluginId: string | null
  installed: InstalledPluginRow[]
}): string {
  const { usePluginUiLanguage, activeLanguagePluginId, installed } = opts
  if (!usePluginUiLanguage || !activeLanguagePluginId) {
    return normalizeLocaleTag(defaultLocale())
  }
  const row = installed.find((p) => p.id === activeLanguagePluginId)
  if (!row || row.kind !== 'language-pack' || !row.enabled) {
    return normalizeLocaleTag(defaultLocale())
  }
  const tag = row.localeTag?.trim()
  if (tag) return normalizeLocaleTag(tag)
  return normalizeLocaleTag(defaultLocale())
}

export function firstValidEnabledLanguagePack(installed: InstalledPluginRow[]): InstalledPluginRow | null {
  for (const p of installed) {
    if (p.kind !== 'language-pack' || !p.enabled) continue
    if (messagesFromLanguagePackRow(p)) return p
  }
  return null
}

export function isLanguagePackSelectable(p: InstalledPluginRow): boolean {
  return p.kind === 'language-pack' && p.enabled && Boolean(messagesFromLanguagePackRow(p))
}
