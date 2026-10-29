import { defineStore } from 'pinia'
import type { InstalledPluginRow } from '@/types/plugin'
import { validateLocaleMessages } from '@/locale/schema'
import type { LocaleMessages } from '@/locale/schema'
import { zhCNMessages } from '@/locale/builtins/zh-CN'
import { enUSMessages } from '@/locale/builtins/en-US'
import { useLocaleStore } from '@/stores/locale'
import { parseInstalledPluginManifest } from '@/utils/pluginManifest'
import { resolveGithubManifestUrl } from '@/utils/githubManifestUrl'
import { requestWorkspaceSettingsSave } from '@/services/workspaceSettingsHub'

function cloneMessages(m: LocaleMessages): Record<string, string> {
  return { ...m }
}

export function getBuiltinInstalledPlugins(): InstalledPluginRow[] {
  return [
    {
      id: 'lang-zh-cn',
      name: '简体中文语言包',
      version: '1.0.0',
      kind: 'language-pack',
      enabled: true,
      localeTag: 'zh-CN',
      messages: cloneMessages(zhCNMessages),
    },
    {
      id: 'lang-en-us',
      name: 'English (US) language pack',
      version: '1.0.0',
      kind: 'language-pack',
      enabled: false,
      localeTag: 'en-US',
      messages: cloneMessages(enUSMessages),
    },
  ]
}

/** 插件列表 — 持久化在工作区 settings.json（由 Go 存储） */
export const usePluginsStore = defineStore('plugins', {
  state: (): {
    installed: InstalledPluginRow[]
    lastToggleBlockedKey: 'settings.languageError' | null
    lastImportErrorKey: string | null
  } => ({
    installed: getBuiltinInstalledPlugins(),
    lastToggleBlockedKey: null,
    lastImportErrorKey: null,
  }),
  actions: {
    resetToBuiltinInstalled() {
      this.installed = getBuiltinInstalledPlugins()
    },
    replaceInstalledFromServer(rows: InstalledPluginRow[]) {
      this.installed = JSON.parse(JSON.stringify(rows)) as InstalledPluginRow[]
    },
    clearToggleBlocked() {
      this.lastToggleBlockedKey = null
    },
    clearPluginImportFeedback() {
      this.lastImportErrorKey = null
    },
    clearPluginPanelAlerts() {
      this.clearToggleBlocked()
      this.lastImportErrorKey = null
    },
    togglePlugin(id: string) {
      const row = this.installed.find((p) => p.id === id)
      if (!row) return
      const turningOn = !row.enabled
      if (row.kind === 'language-pack' && turningOn) {
        const v = validateLocaleMessages(row.messages ?? {})
        if (!v.ok) {
          this.lastToggleBlockedKey = 'settings.languageError'
          return
        }
      }
      this.lastToggleBlockedKey = null
      row.enabled = !row.enabled
      if (row.kind === 'language-pack' && !row.enabled) {
        useLocaleStore().onLanguagePackDisabled(row.id)
      }
      requestWorkspaceSettingsSave()
    },
    importPluginFromJsonText(text: string) {
      this.lastImportErrorKey = null
      let parsed: unknown
      try {
        parsed = JSON.parse(text) as unknown
      } catch {
        this.lastImportErrorKey = 'settings.pluginImportInvalidJson'
        return
      }
      const result = parseInstalledPluginManifest(parsed)
      if (!result.ok) {
        this.lastImportErrorKey = result.errorKey
        return
      }
      if (this.installed.some((p) => p.id === result.row.id)) {
        this.lastImportErrorKey = 'settings.pluginImportDuplicate'
        return
      }
      const row = { ...result.row }
      if (row.kind === 'language-pack' && row.messages) {
        row.messages = { ...row.messages }
      }
      this.installed.push(row)
      requestWorkspaceSettingsSave()
    },
    async importPluginFromGitHubUrl(rawInput: string) {
      this.lastImportErrorKey = null
      const fetchUrl = resolveGithubManifestUrl(rawInput)
      if (!fetchUrl) {
        this.lastImportErrorKey = 'settings.pluginImportInvalidGithubUrl'
        return
      }
      let text: string
      try {
        const res = await fetch(fetchUrl, { credentials: 'omit', mode: 'cors' })
        if (!res.ok) {
          this.lastImportErrorKey = 'settings.pluginImportGithubFetchFailed'
          return
        }
        text = await res.text()
      } catch {
        this.lastImportErrorKey = 'settings.pluginImportGithubFetchFailed'
        return
      }
      this.importPluginFromJsonText(text)
    },
  },
})
