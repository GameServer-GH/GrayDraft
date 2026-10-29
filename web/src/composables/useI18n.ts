import { computed } from 'vue'
import { useLocaleStore } from '@/stores/locale'
import { usePluginsStore } from '@/stores/plugins'
import { documentLangFromState, resolveUiMessages } from '@/locale/resolve'
import type { LocaleMessageKey } from '@/locale/schema'
import { interpolate } from '@/locale/interpolate'

export function useI18n() {
  const localeStore = useLocaleStore()
  const plugins = usePluginsStore()

  const messages = computed(() =>
    resolveUiMessages({
      usePluginUiLanguage: localeStore.usePluginUiLanguage,
      activeLanguagePluginId: localeStore.activeLanguagePluginId,
      installed: plugins.installed,
    }),
  )

  const documentHtmlLang = computed(() =>
    documentLangFromState({
      usePluginUiLanguage: localeStore.usePluginUiLanguage,
      activeLanguagePluginId: localeStore.activeLanguagePluginId,
      installed: plugins.installed,
    }),
  )

  function t(key: LocaleMessageKey, params?: Record<string, string>) {
    return interpolate(messages.value[key] ?? String(key), params)
  }

  return { t, messages, documentHtmlLang, localeStore }
}
