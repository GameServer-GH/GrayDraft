import { defineStore } from 'pinia'
import { requestWorkspaceSettingsSave } from '@/services/workspaceSettingsHub'
import { firstValidEnabledLanguagePack, isLanguagePackSelectable } from '@/locale/resolve'
import type { InstalledPluginRow } from '@/types/plugin'

export type LocaleUiErrorCode = 'no_valid_pack' | 'invalid_selection' | null

export const useLocaleStore = defineStore('locale', {
  state: () => ({
    /** 为 true 时界面文案来自已选且已启用的语言包插件 */
    usePluginUiLanguage: false,
    activeLanguagePluginId: null as string | null,
    lastUiError: null as LocaleUiErrorCode,
  }),
  actions: {
    /** 已由工作区恢复；保留空实现。 */
    hydrate() {},
    persist() {
      requestWorkspaceSettingsSave()
    },
    clearUiError() {
      this.lastUiError = null
    },
    /**
     * 打开「使用语言包」时要求至少有一个已启用且校验通过的语言包。
     * @param installed 来自 plugins store
     */
    setUsePluginUiLanguage(enabled: boolean, installed: InstalledPluginRow[]): boolean {
      this.clearUiError()
      if (!enabled) {
        this.usePluginUiLanguage = false
        this.persist()
        return true
      }
      const first = firstValidEnabledLanguagePack(installed)
      if (!first) {
        this.lastUiError = 'no_valid_pack'
        return false
      }
      this.usePluginUiLanguage = true
      const currentOk =
        this.activeLanguagePluginId &&
        installed.some((p) => p.id === this.activeLanguagePluginId && isLanguagePackSelectable(p))
      if (!currentOk) this.activeLanguagePluginId = first.id
      this.persist()
      return true
    },
    /** 切换当前界面语言（插件 id）；校验失败不改变选择 */
    setActiveLanguagePluginId(pluginId: string | null, installed: InstalledPluginRow[]) {
      this.clearUiError()
      if (pluginId === null) {
        this.activeLanguagePluginId = null
        this.persist()
        return true
      }
      const row = installed.find((p) => p.id === pluginId)
      if (!row || !isLanguagePackSelectable(row)) {
        this.lastUiError = 'invalid_selection'
        return false
      }
      this.activeLanguagePluginId = pluginId
      this.persist()
      return true
    },
    /** 在插件页禁用语言包时，若正在使用则清除选中 */
    onLanguagePackDisabled(pluginId: string) {
      if (this.activeLanguagePluginId === pluginId) {
        this.activeLanguagePluginId = null
        this.persist()
      }
    },
  },
})
