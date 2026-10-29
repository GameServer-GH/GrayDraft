<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useI18n } from '@/composables/useI18n'
import type { LocaleMessageKey } from '@/locale/schema'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginKind } from '@/types/plugin'

defineProps<{
  pluginToggleBlockedText: string
}>()

const { t } = useI18n()
const plugins = usePluginsStore()
const fileInputRef = ref<HTMLInputElement | null>(null)
const githubUrl = ref('')
const githubBusy = ref(false)

function kindLabel(k: PluginKind): string {
  if (k === 'language-pack') return t('pluginKind.languagePack')
  if (k === 'html-notebook') return t('pluginKind.htmlNotebook')
  return t('pluginKind.agentSkill')
}

function kindIcon(k: PluginKind): string {
  if (k === 'language-pack') return 'world-www'
  if (k === 'html-notebook') return 'file-type-html'
  return 'code-dots'
}

function triggerImport() {
  plugins.clearPluginImportFeedback()
  fileInputRef.value?.click()
}

async function onImportFile(ev: Event) {
  const input = ev.target as HTMLInputElement
  const file = input.files?.[0]
  input.value = ''
  if (!file) return
  const text = await file.text()
  plugins.importPluginFromJsonText(text)
}

const importErrorText = computed(() => {
  const key = plugins.lastImportErrorKey
  return key ? t(key as LocaleMessageKey) : ''
})

async function onGithubImport() {
  plugins.clearPluginImportFeedback()
  githubBusy.value = true
  try {
    await plugins.importPluginFromGitHubUrl(githubUrl.value)
  } finally {
    githubBusy.value = false
  }
}
</script>

<template>
  <div class="plugins-panel">
    <div class="plugins-panel__head">
      <div class="plugins-panel__head-text">
        <h2 class="plugins-panel__title">{{ t('settings.pluginsHeading') }}</h2>
      </div>
    </div>

    <div class="plugins-panel__import-card glass-chip">
      <div class="plugins-panel__import-row">
        <span class="plugins-panel__import-label">{{ t('settings.pluginsImportFile') }}</span>
        <div class="plugins-panel__import-file-wrap">
          <input
            ref="fileInputRef"
            type="file"
            class="plugins-panel__file-input"
            accept=".json,application/json"
            :aria-label="t('settings.pluginsImportAria')"
            @change="onImportFile"
          />
          <button type="button" class="plugins-panel__import-btn" @click="triggerImport">
            <Icon class="plugins-panel__import-icon" name="cloud-upload" :size="18" />
            {{ t('settings.pluginsImport') }}
          </button>
        </div>
      </div>
      <div class="plugins-panel__import-row plugins-panel__import-row--github">
        <span class="plugins-panel__import-label">{{ t('settings.pluginsImportGithubLink') }}</span>
        <div class="plugins-panel__github">
          <input
            v-model.trim="githubUrl"
            type="url"
            autocomplete="off"
            class="plugins-panel__github-input"
            :placeholder="t('settings.pluginsImportGithubPlaceholder')"
            @keydown.enter.prevent="onGithubImport"
          />
          <button
            type="button"
            class="plugins-panel__github-btn"
            :disabled="githubBusy || !githubUrl"
            :aria-label="t('settings.pluginsImportGithubAria')"
            @click="onGithubImport"
          >
            <Icon
              v-if="githubBusy"
              class="plugins-panel__spin"
              name="refresh"
              :size="18"
            />
            <Icon v-else class="plugins-panel__github-icon" name="brand-github" :size="18" />
            {{ githubBusy ? t('settings.pluginsImportFetching') : t('settings.pluginsImportGithubBtn') }}
          </button>
        </div>
      </div>
    </div>

    <p v-if="pluginToggleBlockedText" class="plugins-panel__alert" role="alert">
      {{ pluginToggleBlockedText }}
    </p>
    <p v-if="importErrorText" class="plugins-panel__alert" role="alert">
      {{ importErrorText }}
    </p>

    <h3 class="plugins-panel__h3">{{ t('settings.pluginsInstalledSection') }}</h3>
    <ul class="plugins-panel__installed" role="list">
      <li v-for="p in plugins.installed" :key="p.id" class="plugins-panel__row glass-chip">
        <div class="plugins-panel__row-icon" :data-kind="p.kind">
          <Icon :name="kindIcon(p.kind)" :size="22" />
        </div>
        <div class="plugins-panel__row-main">
          <div class="plugins-panel__row-top">
            <span class="plugins-panel__row-name">{{ p.name }}</span>
            <span class="plugins-panel__row-ver">v{{ p.version }}</span>
          </div>
          <div class="plugins-panel__row-meta">
            <span class="plugins-panel__kind">{{ kindLabel(p.kind) }}</span>
            <span class="plugins-panel__row-status" :data-on="p.enabled">
              {{ p.enabled ? t('settings.pluginStatusOn') : t('settings.pluginStatusOff') }}
            </span>
          </div>
        </div>
        <button type="button" class="plugins-panel__toggle" @click="plugins.togglePlugin(p.id)">
          <Icon
            class="plugins-panel__toggle-icon"
            :name="p.enabled ? 'toggle-right' : 'toggle-left'"
            :size="18"
          />
          <span class="plugins-panel__toggle-label">{{
            p.enabled ? t('settings.pluginBtnDisable') : t('settings.pluginBtnEnable')
          }}</span>
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.plugins-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
  min-width: 0;
}

.plugins-panel__head {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-4);
  flex-wrap: wrap;
}

.plugins-panel__head-text {
  flex: 1 1 220px;
  min-width: 0;
}

.plugins-panel__title {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 650;
}

.plugins-panel__intro {
  margin: 0;
  font-size: 14px;
  line-height: 1.55;
  color: var(--color-text-secondary);
}

.plugins-panel__import-card {
  padding: var(--space-3);
  border-radius: 12px;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.plugins-panel__import-row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  flex-wrap: wrap;
}

.plugins-panel__import-row--github {
  align-items: flex-start;
}

.plugins-panel__import-label {
  flex: 0 0 88px;
  font-size: 13px;
  font-weight: 650;
  color: var(--color-text-secondary);
}

.plugins-panel__import-file-wrap {
  flex: 1 1 200px;
  min-width: 0;
  display: flex;
  align-items: center;
}

.plugins-panel__file-input {
  position: absolute;
  width: 1px;
  height: 1px;
  padding: 0;
  margin: -1px;
  overflow: hidden;
  clip: rect(0, 0, 0, 0);
  white-space: nowrap;
  border: 0;
}

.plugins-panel__import-btn {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 16px;
  font-size: 14px;
  font-weight: 600;
  border-radius: 10px;
  border: 1px solid rgba(0, 122, 255, 0.35);
  background: linear-gradient(180deg, rgba(255, 255, 255, 0.95), rgba(240, 246, 255, 0.88));
  color: var(--color-accent);
  cursor: pointer;
  transition:
    background 0.15s ease,
    box-shadow 0.15s ease;
}

.plugins-panel__import-btn:hover {
  background: #fff;
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.12);
}

.plugins-panel__import-icon {
  flex-shrink: 0;
  opacity: 0.9;
}

.plugins-panel__github {
  flex: 1 1 240px;
  min-width: 0;
  display: flex;
  gap: 10px;
  flex-wrap: wrap;
  align-items: center;
}

.plugins-panel__github-input {
  flex: 1 1 180px;
  min-width: 0;
  border: 1px solid var(--color-border);
  border-radius: 10px;
  padding: 10px 12px;
  font: inherit;
  font-size: 14px;
  background: rgba(255, 255, 255, 0.85);
  color: var(--color-text);
}

.plugins-panel__github-btn {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 10px 14px;
  font-size: 14px;
  font-weight: 600;
  border-radius: 10px;
  border: 1px solid rgba(30, 30, 35, 0.14);
  background: rgba(30, 30, 35, 0.06);
  color: var(--color-text);
  cursor: pointer;
  transition:
    background 0.15s ease,
    border-color 0.15s ease;
}

.plugins-panel__github-btn:hover:not(:disabled) {
  background: rgba(30, 30, 35, 0.09);
  border-color: rgba(30, 30, 35, 0.22);
}

.plugins-panel__github-btn:disabled {
  opacity: 0.65;
  cursor: not-allowed;
}

.plugins-panel__github-icon {
  flex-shrink: 0;
  opacity: 0.88;
}

.plugins-panel__hint {
  margin: 0;
  font-size: 12px;
  line-height: 1.45;
  color: var(--color-text-secondary);
}

.plugins-panel__hint--sub {
  margin-top: calc(var(--space-2) * -1);
  opacity: 0.92;
}

.plugins-panel__alert {
  margin: 0;
  padding: 10px 12px;
  border-radius: 10px;
  font-size: 13px;
  line-height: 1.45;
  color: #7a1e12;
  background: rgba(255, 236, 232, 0.95);
  border: 1px solid rgba(180, 35, 24, 0.18);
}

.plugins-panel__h3 {
  margin: 0;
  font-size: 13px;
  font-weight: 700;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
}

.plugins-panel__installed {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.plugins-panel__row {
  display: flex;
  align-items: center;
  gap: var(--space-3);
  padding: 12px 14px;
  border-radius: 12px;
}

.plugins-panel__row-icon {
  flex-shrink: 0;
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 10px;
  background: rgba(255, 255, 255, 0.55);
  border: 1px solid rgba(0, 0, 0, 0.06);
}

.plugins-panel__row-icon[data-kind='language-pack'] {
  background: rgba(0, 122, 255, 0.08);
}

.plugins-panel__row-icon[data-kind='html-notebook'] {
  background: rgba(120, 80, 200, 0.08);
}

.plugins-panel__row-icon[data-kind='agent-skill'] {
  background: rgba(20, 160, 120, 0.08);
}

.plugins-panel__row-main {
  flex: 1 1 auto;
  min-width: 0;
}

.plugins-panel__row-top {
  display: flex;
  align-items: baseline;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.plugins-panel__row-name {
  font-size: 15px;
  font-weight: 650;
  color: var(--color-text);
}

.plugins-panel__row-ver {
  font-size: 12px;
  color: var(--color-text-secondary);
  font-weight: 600;
}

.plugins-panel__row-meta {
  display: flex;
  align-items: center;
  gap: 10px;
  margin-top: 4px;
  flex-wrap: wrap;
}

.plugins-panel__kind {
  font-size: 11px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 6px;
  background: var(--color-accent-muted);
  color: var(--color-accent);
}

.plugins-panel__row-status {
  font-size: 12px;
  color: var(--color-text-secondary);
}

.plugins-panel__row-status[data-on='true'] {
  color: var(--color-accent);
  font-weight: 600;
}

.plugins-panel__toggle {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  gap: 6px;
  padding: 8px 12px;
  font-size: 13px;
  font-weight: 600;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.85);
  color: var(--color-text);
  cursor: pointer;
  transition:
    border-color 0.15s ease,
    background 0.15s ease;
}

.plugins-panel__toggle:hover {
  border-color: rgba(0, 122, 255, 0.35);
  background: #fff;
}

.plugins-panel__toggle-icon {
  flex-shrink: 0;
  opacity: 0.8;
}

@keyframes plugins-panel-spin {
  to {
    transform: rotate(360deg);
  }
}

.plugins-panel__spin {
  animation: plugins-panel-spin 0.75s linear infinite;
}

@media (max-width: 560px) {
  .plugins-panel__import-row {
    flex-direction: column;
    align-items: flex-start;
  }

  .plugins-panel__import-label {
    flex-basis: auto;
  }

  .plugins-panel__github {
    width: 100%;
  }

  .plugins-panel__github-btn {
    width: 100%;
    justify-content: center;
  }

  .plugins-panel__row {
    flex-wrap: wrap;
  }

  .plugins-panel__toggle {
    width: 100%;
    justify-content: center;
  }
}
</style>
