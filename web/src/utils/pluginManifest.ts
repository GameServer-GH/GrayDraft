import { validateLocaleMessages } from '@/locale/schema'
import type { InstalledPluginRow, PluginKind } from '@/types/plugin'

const KINDS: PluginKind[] = ['language-pack', 'html-notebook', 'agent-skill']

export type PluginManifestParseErrorKey =
  | 'settings.pluginImportInvalidJson'
  | 'settings.pluginImportInvalidShape'
  | 'settings.pluginImportInvalidKind'
  | 'settings.pluginImportLanguagePack'

function isRecord(v: unknown): v is Record<string, unknown> {
  return v !== null && typeof v === 'object' && !Array.isArray(v)
}

function asNonEmptyString(v: unknown): v is string {
  return typeof v === 'string' && v.trim().length > 0
}

/** 从 JSON 对象解析为待安装行（默认未启用） */
export function parseInstalledPluginManifest(raw: unknown):
  | { ok: true; row: InstalledPluginRow }
  | { ok: false; errorKey: PluginManifestParseErrorKey } {
  if (!isRecord(raw)) {
    return { ok: false, errorKey: 'settings.pluginImportInvalidShape' }
  }
  const id = raw.id
  const name = raw.name
  const version = raw.version
  const kind = raw.kind
  if (!asNonEmptyString(id) || !asNonEmptyString(name) || !asNonEmptyString(version)) {
    return { ok: false, errorKey: 'settings.pluginImportInvalidShape' }
  }
  if (!asNonEmptyString(kind) || !KINDS.includes(kind as PluginKind)) {
    return { ok: false, errorKey: 'settings.pluginImportInvalidKind' }
  }
  const k = kind as PluginKind
  if (k === 'language-pack') {
    const localeTag = raw.localeTag
    if (!asNonEmptyString(localeTag)) {
      return { ok: false, errorKey: 'settings.pluginImportLanguagePack' }
    }
    const v = validateLocaleMessages(raw.messages)
    if (!v.ok) {
      return { ok: false, errorKey: 'settings.pluginImportLanguagePack' }
    }
    return {
      ok: true,
      row: {
        id: id.trim(),
        name: name.trim(),
        version: version.trim(),
        kind: k,
        enabled: false,
        localeTag: localeTag.trim(),
        messages: { ...(raw.messages as Record<string, string>) },
      },
    }
  }
  return {
    ok: true,
    row: {
      id: id.trim(),
      name: name.trim(),
      version: version.trim(),
      kind: k,
      enabled: false,
    },
  }
}
