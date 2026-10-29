<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import Icon from '@/components/common/Icon.vue'
import Select from '@/components/form/Select.vue'
import type { FormSelectOption } from '@/components/form/Select.vue'
import { useI18n } from '@/composables/useI18n'
import { languagePackHelpUrl } from '@/config/env'
import { isLanguagePackSelectable } from '@/locale/resolve'
import { useAgentModelConfigStore, VENDOR_KIND_IDS, type VendorKindId } from '@/stores/agentModelConfig'
import { useLocaleStore } from '@/stores/locale'
import { usePluginsStore } from '@/stores/plugins'
import { useUiStore } from '@/stores/ui'
import SettingsPluginsPanel from '@/components/floating/SettingsPluginsPanel.vue'
import SettingsDataPanel from '@/components/floating/SettingsDataPanel.vue'
import SettingsAgentPanel from '@/components/floating/SettingsAgentPanel.vue'

const emit = defineEmits<{
  close: []
}>()

const { t } = useI18n()
const ui = useUiStore()
const plugins = usePluginsStore()
const locale = useLocaleStore()
const agentModels = useAgentModelConfigStore()

const tab = ref<'general' | 'models' | 'agent' | 'plugins' | 'data'>('general')

const newVendorKind = ref<VendorKindId>('deepseek')
const newVendorKey = ref('')
const selectedVendorId = ref<string | null>(null)
const newModelName = ref('')

const modelNamePlaceholder = computed(() => {
  if (selectedVendor.value?.kindId === 'zhipu') return 'glm-5.1'
  return 'deepseek-v4-flash'
})

function vendorKindLabel(kind: VendorKindId): string {
  if (kind === 'anthropic') return t('settings.vendorAnthropic')
  if (kind === 'openai') return t('settings.vendorOpenai')
  if (kind === 'google') return t('settings.vendorGoogle')
  if (kind === 'zhipu') return t('settings.vendorZhipu')
  return t('settings.vendorDeepseek')
}

const vendorKindSelectOptions = computed<FormSelectOption[]>(() =>
  VENDOR_KIND_IDS.map((id) => ({
    value: id,
    label: vendorKindLabel(id),
  })),
)

function onAddVendor() {
  agentModels.addVendor(newVendorKind.value, newVendorKey.value)
  newVendorKey.value = ''
  const last = agentModels.vendors[agentModels.vendors.length - 1]
  if (last) selectedVendorId.value = last.id
}

function onSelectVendor(id: string) {
  selectedVendorId.value = id
}

const selectedVendor = computed(() =>
  selectedVendorId.value ? agentModels.vendors.find((v) => v.id === selectedVendorId.value) : undefined,
)

function onAddModel() {
  if (!selectedVendorId.value) return
  agentModels.addModel(selectedVendorId.value, newModelName.value)
  newModelName.value = ''
}

watch(
  () => agentModels.vendors.map((v) => v.id).join(),
  () => {
    if (selectedVendorId.value && !agentModels.vendors.some((v) => v.id === selectedVendorId.value)) {
      selectedVendorId.value = null
    }
  },
)

watch([tab, () => agentModels.vendors.length], () => {
  if (tab.value !== 'models') return
  if (selectedVendorId.value) return
  const first = agentModels.vendors[0]
  if (first) selectedVendorId.value = first.id
})

const selectablePacks = computed(() => plugins.installed.filter(isLanguagePackSelectable))

/** 无可用语言包时用于 Select 的占位项（不可提交） */
const LANGUAGE_PACK_NONE = '__none'

const languagePackSelectOptions = computed<FormSelectOption[]>(() => {
  if (!selectablePacks.value.length) {
    return [{ value: LANGUAGE_PACK_NONE, label: t('settings.languagePlaceholder') }]
  }
  return selectablePacks.value.map((p) => ({
    value: p.id,
    label: `${p.name} (${p.localeTag ?? ''})`.trim(),
  }))
})

const languagePackSelectValue = computed(() => {
  const id = locale.activeLanguagePluginId ?? ''
  if (selectablePacks.value.some((p) => p.id === id)) return id
  if (!selectablePacks.value.length) return LANGUAGE_PACK_NONE
  return selectablePacks.value[0]!.id
})

function onLanguagePackSelect(v: string) {
  if (v === LANGUAGE_PACK_NONE) return
  locale.setActiveLanguagePluginId(v, plugins.installed)
}

const pluginToggleBlockedText = computed(() =>
  plugins.lastToggleBlockedKey ? t(plugins.lastToggleBlockedKey) : '',
)

const localeUiErrorText = computed(() => {
  if (locale.lastUiError === 'no_valid_pack') return t('settings.languageNoValidPack')
  if (locale.lastUiError === 'invalid_selection') return t('settings.languageError')
  return ''
})

watch(tab, (v) => {
  if (v === 'plugins') {
    plugins.clearToggleBlocked()
    plugins.clearPluginImportFeedback()
  }
})

watch(
  () => [tab.value, locale.usePluginUiLanguage, selectablePacks.value.map((p) => p.id).join()] as const,
  () => {
    if (tab.value !== 'general' || !locale.usePluginUiLanguage) return
    const id = locale.activeLanguagePluginId
    const ok = Boolean(id && selectablePacks.value.some((p) => p.id === id))
    if (!ok && selectablePacks.value.length) {
      locale.setActiveLanguagePluginId(selectablePacks.value[0]!.id, plugins.installed)
    }
  },
)

function onClose() {
  ui.setSettingsOpen(false)
  emit('close')
}

function openLanguageHelp() {
  window.open(languagePackHelpUrl(), '_blank', 'noopener,noreferrer')
}

function onToggleUsePlugin(ev: Event) {
  const el = ev.target as HTMLInputElement
  const want = el.checked
  const ok = locale.setUsePluginUiLanguage(want, plugins.installed)
  if (!ok) el.checked = false
}

</script>

<template>
  <div
    class="settings-view"
    role="dialog"
    aria-modal="true"
    aria-labelledby="settings-title"
    :style="{ zIndex: ui.settingsModalZIndex }"
  >
    <div class="settings-view__shell glass-panel">
      <header class="settings-view__top">
        <h1 id="settings-title" class="settings-view__title">
          {{ t('settings.title') }}
        </h1>
        <div class="settings-view__top-actions">
          <button type="button" class="settings-view__close" @click="onClose">
            {{ t('settings.done') }}
          </button>
          <button
            type="button"
            class="settings-view__close-muted"
            :aria-label="t('settings.closeDismissAria')"
            :title="t('settings.closeDismissAria')"
            @click="onClose"
          >
            <Icon class="settings-view__close-muted-icon" name="x" :size="18" />
          </button>
        </div>
      </header>
      <div class="settings-view__body">
        <nav class="settings-view__nav glass-chip" :aria-label="t('settings.navAria')">
          <button
            type="button"
            class="settings-view__tab"
            :class="{ 'settings-view__tab--active': tab === 'general' }"
            @click="tab = 'general'"
          >
            <Icon class="settings-view__tab-icon" name="category" :size="16" />
            {{ t('settings.tabGeneral') }}
          </button>
          <button
            type="button"
            class="settings-view__tab"
            :class="{ 'settings-view__tab--active': tab === 'agent' }"
            @click="tab = 'agent'"
          >
            <Icon class="settings-view__tab-icon" name="ai" :size="16" />
            {{ t('settings.tabAgent') }}
          </button>
          <button
            type="button"
            class="settings-view__tab"
            :class="{ 'settings-view__tab--active': tab === 'models' }"
            @click="tab = 'models'"
          >
            <Icon class="settings-view__tab-icon" name="key" :size="16" />
            {{ t('settings.tabModels') }}
          </button>
          <button
            type="button"
            class="settings-view__tab"
            :class="{ 'settings-view__tab--active': tab === 'plugins' }"
            @click="tab = 'plugins'"
          >
            <Icon class="settings-view__tab-icon" name="code" :size="16" />
            {{ t('settings.tabPlugins') }}
          </button>
          <button
            type="button"
            class="settings-view__tab"
            :class="{ 'settings-view__tab--active': tab === 'data' }"
            @click="tab = 'data'"
          >
            <Icon class="settings-view__tab-icon" name="database" :size="16" />
            {{ t('settings.tabData') }}
          </button>
        </nav>
        <div class="settings-view__content glass-chip">
          <section v-if="tab === 'general'" class="settings-view__section">
            <h2 class="settings-view__h2">{{ t('settings.generalHeading') }}</h2>

            <h3 class="settings-view__h3 settings-view__h3--sub">{{ t('settings.languageHeading') }}</h3>
            <p v-if="localeUiErrorText" class="settings-view__p settings-view__err" role="alert">
              {{ localeUiErrorText }}
            </p>
            <label class="settings-view__toggle-field">
              <input
                type="checkbox"
                class="settings-view__checkbox"
                :checked="locale.usePluginUiLanguage"
                @change="onToggleUsePlugin"
              />
              <span class="settings-view__toggle-label">{{ t('settings.languageUsePlugin') }}</span>
            </label>
            <div class="settings-view__lang-row">
              <label class="settings-view__field settings-view__field--grow">
                <span class="settings-view__label">{{ t('settings.languageSelect') }}</span>
                <div class="settings-view__language-select">
                  <Select
                    id="settings-language-pack"
                    :model-value="languagePackSelectValue"
                    :options="languagePackSelectOptions"
                    :disabled="!locale.usePluginUiLanguage || selectablePacks.length === 0"
                    :aria-label="t('settings.languageSelect')"
                    @update:model-value="onLanguagePackSelect"
                  />
                </div>
              </label>
              <button
                type="button"
                class="settings-view__help-icon"
                :aria-label="t('settings.languageHelpAria')"
                :title="t('settings.languageHelpTitle')"
                @click="openLanguageHelp"
              >
                <Icon name="help" :size="18" />
              </button>
            </div>

            <div class="settings-view__switch-row">
              <span class="settings-view__switch-label">{{ t("settings.canvasDisable") }}</span>
              <label class="settings-view__switch">
                <input
                  type="checkbox"
                  :checked="ui.canvasDisabled"
                  @change="ui.setCanvasDisabled(($event.target as HTMLInputElement).checked)"
                />
                <span class="settings-view__switch-track"></span>
              </label>
            </div>
          </section>

          <section v-else-if="tab === 'agent'" class="settings-view__section settings-view__section--agent">
            <SettingsAgentPanel />
          </section>

          <section v-else-if="tab === 'models'" class="settings-view__section settings-view__section--models">
            <h2 class="settings-view__h2">{{ t('settings.modelsHeading') }}</h2>

            <h3 class="settings-view__h3 settings-view__h3--sub">{{ t('settings.modelsVendorsHeading') }}</h3>
            <div class="settings-view__models-add-row">
              <label class="settings-view__field settings-view__field--grow">
                <span class="settings-view__label">{{ t('settings.modelsVendorKind') }}</span>
                <div class="settings-view__language-select">
                  <Select
                    id="settings-model-vendor-kind"
                    :model-value="newVendorKind"
                    :options="vendorKindSelectOptions"
                    :aria-label="t('settings.modelsVendorKind')"
                    @update:model-value="(v) => (newVendorKind = v as VendorKindId)"
                  />
                </div>
              </label>
              <label class="settings-view__field settings-view__field--grow">
                <span class="settings-view__label">{{ t('settings.modelsVendorKey') }}</span>
                <input
                  v-model="newVendorKey"
                  class="settings-view__input settings-view__input--active"
                  type="password"
                  autocomplete="off"
                  :placeholder="t('settings.modelsVendorKey')"
                />
              </label>
              <button
                type="button"
                class="settings-view__add-btn"
                :disabled="!newVendorKey.trim()"
                @click="onAddVendor"
              >
                {{ t('settings.modelsAddVendorBtn') }}
              </button>
            </div>

            <h3 class="settings-view__h3 settings-view__h3--sub">{{ t('settings.modelsVendorListTitle') }}</h3>
            <p v-if="!agentModels.vendors.length" class="settings-view__p muted">{{ t('settings.modelsNoVendors') }}</p>
            <ul v-if="agentModels.vendors.length" class="settings-view__vendor-list" role="list">
              <li
                v-for="v in agentModels.vendors"
                :key="v.id"
                class="settings-view__vendor-item"
                :class="{ 'settings-view__vendor-item--active': selectedVendorId === v.id }"
                role="listitem"
              >
                <div class="settings-view__vendor-row1">
                  <button type="button" class="settings-view__vendor-select" @click="onSelectVendor(v.id)">
                    <span class="settings-view__vendor-title">{{ vendorKindLabel(v.kindId) }}</span>
                    <span class="settings-view__vendor-meta">{{
                      t('settings.modelsModelCount', { count: String(v.models.length) })
                    }}</span>
                  </button>
                  <label class="settings-view__vendor-toggle">
                    <input
                      type="checkbox"
                      class="settings-view__checkbox"
                      :checked="v.enabled"
                      @change="
                        agentModels.setVendorEnabled(v.id, ($event.target as HTMLInputElement).checked)
                      "
                    />
                    <span class="settings-view__toggle-label">{{ t('settings.modelsVendorEnabled') }}</span>
                  </label>
                  <button
                    type="button"
                    class="settings-view__icon-btn"
                    :aria-label="t('settings.modelsDeleteVendorAria')"
                    :title="t('settings.modelsDeleteVendorAria')"
                    @click="agentModels.removeVendor(v.id)"
                  >
                    <Icon name="trash" :size="18" />
                  </button>
                </div>
                <label class="settings-view__field settings-view__field--block">
                  <span class="settings-view__label">{{ t('settings.modelsVendorKey') }}</span>
                  <input
                    class="settings-view__input settings-view__input--active"
                    type="password"
                    autocomplete="off"
                    :value="v.apiKey"
                    @change="agentModels.setVendorApiKey(v.id, ($event.target as HTMLInputElement).value)"
                  />
                </label>
              </li>
            </ul>

            <h3 class="settings-view__h3 settings-view__h3--sub">{{ t('settings.modelsEditModelsTitle') }}</h3>
            <template v-if="!selectedVendor">
              <p class="settings-view__p muted">{{ t('settings.modelsNoVendorSelected') }}</p>
            </template>
            <template v-else>
              <p class="settings-view__p muted settings-view__p--tight">
                {{ vendorKindLabel(selectedVendor.kindId) }}
              </p>
              <div class="settings-view__models-add-row">
                <label class="settings-view__field settings-view__field--grow">
                  <span class="settings-view__label">{{ t('settings.modelsModelIdLabel') }}</span>
                  <input
                    v-model="newModelName"
                    class="settings-view__input settings-view__input--active"
                    type="text"
                    autocomplete="off"
                    :placeholder="modelNamePlaceholder"
                  />
                </label>
                <button
                  type="button"
                  class="settings-view__add-btn"
                  :disabled="!newModelName.trim()"
                  @click="onAddModel"
                >
                  {{ t('settings.modelsAddModelBtn') }}
                </button>
              </div>
              <p v-if="!selectedVendor.models.length" class="settings-view__p muted">
                {{ t('settings.modelsNoModelsInVendor') }}
              </p>
              <ul v-else class="settings-view__model-list" role="list">
                <li v-for="m in selectedVendor.models" :key="m.id" class="settings-view__model-item" role="listitem">
                  <label class="settings-view__vendor-toggle settings-view__vendor-toggle--compact">
                    <input
                      type="checkbox"
                      class="settings-view__checkbox"
                      :checked="m.enabled"
                      @change="
                        agentModels.setModelEnabled(
                          selectedVendor.id,
                          m.id,
                          ($event.target as HTMLInputElement).checked,
                        )
                      "
                    />
                    <span class="settings-view__model-name">{{ m.name }}</span>
                  </label>
                  <button
                    type="button"
                    class="settings-view__icon-btn"
                    :aria-label="t('settings.modelsDeleteModelAria')"
                    :title="t('settings.modelsDeleteModelAria')"
                    @click="agentModels.removeModel(selectedVendor.id, m.id)"
                  >
                    <Icon name="trash" :size="16" />
                  </button>
                </li>
              </ul>
            </template>
          </section>

          <section v-else-if="tab === 'plugins'" class="settings-view__section settings-view__section--plugins">
            <SettingsPluginsPanel :plugin-toggle-blocked-text="pluginToggleBlockedText" />
          </section>

          <section v-else class="settings-view__section settings-view__section--data">
            <SettingsDataPanel />
          </section>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.settings-view {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  box-sizing: border-box;
  background: rgba(234, 234, 238, 0.72);
  backdrop-filter: blur(18px) saturate(1.2);
  -webkit-backdrop-filter: blur(18px) saturate(1.2);
}

.settings-view__shell {
  width: 92%;
  max-width: 1200px;
  height: 85%;
  max-height: 860px;
  flex-shrink: 0;
  border-radius: 16px;
  padding: var(--space-4);
  display: flex;
  flex-direction: column;
  min-height: 0;
  box-sizing: border-box;
  background: var(--glass-bg-deep);
  box-shadow: var(--shadow-float-lg);
}

.settings-view__top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
  flex-shrink: 0;
  padding: 0 var(--space-2);
}

.settings-view__top-actions {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
}

.settings-view__close-muted {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  padding: 0;
  border-radius: 10px;
  border: 1px solid rgba(60, 60, 67, 0.1);
  background: rgba(60, 60, 67, 0.06);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease,
    border-color 0.15s ease;
}

.settings-view__close-muted:hover {
  background: rgba(60, 60, 67, 0.1);
  color: var(--color-text);
  border-color: rgba(60, 60, 67, 0.16);
}

.settings-view__close-muted:focus-visible {
  outline: none;
  box-shadow: 0 0 0 2px rgba(0, 122, 255, 0.18);
}

.settings-view__close-muted-icon {
  display: block;
  opacity: 0.88;
}

.settings-view__title {
  margin: 0;
  display: flex;
  align-items: center;
  gap: 10px;
  font-size: 20px;
  font-weight: 650;
  letter-spacing: -0.02em;
}
.settings-view__title-icon {
  flex-shrink: 0;
  opacity: 0.8;
}

.settings-view__close {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 16px;
  font-size: 14px;
  font-weight: 600;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.65);
  color: var(--color-accent);
  cursor: pointer;
  transition: background 0.15s ease;
  height: 38px;
}
.settings-view__close-icon {
  flex-shrink: 0;
  opacity: 0.85;
}

.settings-view__close:hover {
  background: #fff;
}

.settings-view__body {
  display: flex;
  gap: var(--space-4);
  min-height: 0;
  flex: 1;
}

.settings-view__nav {
  flex: 0 0 172px;
  display: flex;
  flex-direction: column;
  align-items: stretch;
  padding: var(--space-2);
  gap: 6px;
  border-radius: 12px;
  min-height: 0;
  overflow-y: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.settings-view__nav::-webkit-scrollbar {
  display: none;
}

.settings-view__tab {
  display: flex;
  align-items: center;
  gap: 10px;
  width: 100%;
  text-align: left;
  padding: 11px 12px;
  white-space: normal;
  line-height: 1.35;
  border-radius: 8px;
  border: none;
  background: transparent;
  color: var(--color-text-secondary);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease;
}
.settings-view__tab-icon {
  flex-shrink: 0;
  opacity: 0.72;
}
.settings-view__tab--active .settings-view__tab-icon {
  opacity: 0.95;
}

.settings-view__tab:hover {
  background: var(--color-selected-soft);
  color: var(--color-text);
}

.settings-view__tab--active {
  background: var(--color-selected-soft);
  color: var(--color-text);
}

.settings-view__content {
  flex: 1 1 auto;
  min-width: 0;
  min-height: 0;
  border-radius: 12px;
  padding: var(--space-4);
  overflow: auto;
  scrollbar-width: none;
  -ms-overflow-style: none;
}

.settings-view__content::-webkit-scrollbar {
  display: none;
}

.settings-view__section {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}

.settings-view__section--plugins {
  display: flex;
  flex-direction: column;
  min-height: 0;
}

.settings-view__h2 {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 650;
}

.settings-view__h3 {
  margin: 0 0 var(--space-2);
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 15px;
  font-weight: 600;
}
.settings-view__h3-icon {
  flex-shrink: 0;
  opacity: 0.75;
}

.settings-view__h3--sub {
  margin-top: var(--space-4);
  font-size: 14px;
  font-weight: 600;
}

.settings-view__err {
  color: #b42318;
  font-size: 13px;
}

.settings-view__p--tight {
  margin-top: calc(var(--space-1) * -1);
}

.settings-view__toggle-field {
  display: flex;
  align-items: center;
  gap: 10px;
  margin: var(--space-2) 0 var(--space-2);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
}

.settings-view__checkbox {
  width: 16px;
  height: 16px;
  accent-color: var(--color-accent);
  cursor: pointer;
}

.settings-view__toggle-label {
  color: var(--color-text);
}

/* ── 画布开关 ── */
.settings-view__switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 0;
  border-top: 1px solid rgba(60, 60, 67, 0.08);
  margin-top: var(--space-3);
}

.settings-view__switch-label {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.settings-view__switch {
  position: relative;
  display: inline-block;
  width: 44px;
  height: 26px;
  flex-shrink: 0;
  cursor: pointer;
}

.settings-view__switch input {
  position: absolute;
  opacity: 0;
  width: 0;
  height: 0;
}

.settings-view__switch-track {
  position: absolute;
  inset: 0;
  border-radius: 26px;
  background: rgba(60, 60, 67, 0.22);
  transition: background 0.2s ease;
}

.settings-view__switch-track::before {
  content: "";
  position: absolute;
  top: 3px;
  left: 3px;
  width: 20px;
  height: 20px;
  border-radius: 50%;
  background: #fff;
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.18);
  transition: transform 0.2s ease;
}

.settings-view__switch input:checked + .settings-view__switch-track {
  background: var(--color-accent);
}

.settings-view__switch input:checked + .settings-view__switch-track::before {
  transform: translateX(18px);
}

.settings-view__lang-row {
  display: flex;
  align-items: flex-end;
  gap: var(--space-2);
  margin-top: var(--space-3);
}

.settings-view__field--grow {
  flex: 1;
  min-width: 0;
}

.settings-view__language-select {
  width: 100%;
  min-width: 0;
}

.settings-view__language-select :deep(.form-select) {
  display: flex;
  width: 100%;
}

.settings-view__help-icon {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 34px;
  height: 34px;
  margin-bottom: 1px;
  padding: 0;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.75);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    background 0.15s ease,
    border-color 0.15s ease;
}

.settings-view__help-icon:hover {
  background: #fff;
  border-color: rgba(0, 122, 255, 0.35);
  color: var(--color-accent);
}

.settings-view__p {
  margin: 0 0 var(--space-3);
  font-size: 14px;
  line-height: 1.55;
  color: var(--color-text);
}

.settings-view__p.muted {
  color: var(--color-text-secondary);
}

.settings-view__grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(min(220px, 100%), 1fr));
  gap: var(--space-3);
}

.settings-view__field {
  display: flex;
  flex-direction: column;
  gap: 6px;
  font-size: 13px;
}

.settings-view__label {
  color: var(--color-text-secondary);
  font-weight: 600;
}

.settings-view__input {
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 8px 10px;
  font: inherit;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.75);
  color: var(--color-text-secondary);
}

.settings-view__input--active {
  color: var(--color-text);
}

.settings-view__models-add-row {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  gap: var(--space-3);
  margin-bottom: var(--space-4);
}

@media (min-width: 720px) {
  .settings-view__models-add-row {
    display: grid;
    grid-template-columns: 1fr 1fr auto;
    align-items: end;
  }
}

.settings-view__section--models .settings-view__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.settings-view__add-btn {
  flex-shrink: 0;
  height: 38px;
  padding: 0 18px;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: var(--color-accent-muted);
  color: var(--color-accent);
  font-size: 14px;
  font-weight: 600;
  cursor: pointer;
  transition: background 0.15s ease;
}

.settings-view__add-btn:hover:not(:disabled) {
  background: rgba(0, 122, 255, 0.14);
}

.settings-view__add-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}

.settings-view__vendor-list,
.settings-view__model-list {
  list-style: none;
  margin: 0 0 var(--space-4);
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.settings-view__vendor-item {
  border-radius: 12px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.55);
  padding: var(--space-3);
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.settings-view__vendor-item--active {
  border-color: var(--color-selected-line);
  box-shadow: 0 0 0 2px var(--color-selected-soft);
}

.settings-view__vendor-row1 {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
  margin-bottom: var(--space-2);
}

.settings-view__vendor-select {
  flex: 1 1 160px;
  min-width: 0;
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 8px 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  background: rgba(255, 255, 255, 0.65);
  cursor: pointer;
  font: inherit;
  text-align: left;
  transition:
    border-color 0.15s ease,
    background 0.15s ease;
}

.settings-view__vendor-select:hover {
  border-color: rgba(0, 122, 255, 0.2);
  background: rgba(255, 255, 255, 0.9);
}

.settings-view__vendor-title {
  font-size: 14px;
  font-weight: 650;
  color: var(--color-text);
}

.settings-view__vendor-meta {
  font-size: 12px;
  font-weight: 600;
  color: var(--color-text-secondary);
  flex-shrink: 0;
}

.settings-view__vendor-toggle {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
  user-select: none;
  flex-shrink: 0;
}

.settings-view__vendor-toggle--compact {
  flex: 1;
  min-width: 0;
}

.settings-view__field--block {
  width: 100%;
}

.settings-view__icon-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 38px;
  height: 38px;
  padding: 0;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.75);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition:
    background 0.15s ease,
    color 0.15s ease,
    border-color 0.15s ease;
}

.settings-view__icon-btn:hover {
  background: #fff;
  color: #b42318;
  border-color: rgba(180, 35, 24, 0.25);
}

.settings-view__model-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--space-2);
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.65);
}

.settings-view__model-name {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 720px) {
  .settings-view__body {
    flex-direction: column;
  }

  .settings-view__nav {
    flex: 0 0 auto;
    flex-direction: row;
    flex-wrap: wrap;
  }
}
</style>
