<script setup lang="ts">
import { ref } from 'vue'
import { useI18n } from '@/composables/useI18n'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import { formatNexusError } from '@/utils/nexusError'

const { t } = useI18n()
const session = useWorkspaceSessionStore()
const keyInput = ref('')
const busy = ref(false)
const localError = ref('')
const copiedKey = ref('')

const sampleWorkspaces = [
  { key: 'contest-scp-skills', content: '书生 SCP 技能与工具', docs: '36 篇，260 段' },
  { key: 'contest-papers', content: 'InternLM2、InternVL、InternVL 2.5', docs: '44 篇，250 段' },
]

async function copySampleKey(k: string) {
  localError.value = ''
  keyInput.value = k
  try {
    await navigator.clipboard.writeText(k)
    copiedKey.value = k
  } catch {
    copiedKey.value = ''
  }
}

async function onSubmit() {
  localError.value = ''
  const k = keyInput.value.trim()
  if (!k) {
    localError.value = t('workspace.gateKeyRequired')
    return
  }
  busy.value = true
  try {
    await session.openWithKey(k, true)
  } catch (e) {
    localError.value = session.openError ?? formatNexusError(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="ws-gate" role="dialog" aria-modal="true" :aria-label="t('workspace.gateTitle')">
    <div class="ws-gate__panel glass-panel">
      <h1 class="ws-gate__title">{{ t('workspace.gateTitle') }}</h1>
      <input
        id="ws-gate-key"
        v-model="keyInput"
        type="password"
        autocomplete="off"
        class="ws-gate__input"
        :placeholder="t('workspace.gatePlaceholder')"
        :aria-label="t('workspace.gateTitle')"
        :disabled="busy"
        @keydown.enter.prevent="onSubmit"
      />
      <p v-if="localError" class="ws-gate__error">{{ localError }}</p>
      <div class="ws-gate__samples">
        <p class="ws-gate__samples-title">
          {{ t('workspace.gateSamplesTitle') }}
          <span v-if="copiedKey" class="ws-gate__samples-copied">
            {{ t('workspace.gateSampleCopied') }}
          </span>
        </p>
        <button
          v-for="sample in sampleWorkspaces"
          :key="sample.key"
          type="button"
          class="ws-gate__sample"
          :class="{ 'ws-gate__sample--active': copiedKey === sample.key }"
          :title="sample.key"
          @click="copySampleKey(sample.key)"
        >
          <span class="ws-gate__sample-key">{{ sample.key }}</span>
          <span class="ws-gate__sample-content">{{ sample.content }}</span>
          <span class="ws-gate__sample-docs">{{ sample.docs }}</span>
        </button>
      </div>
      <button type="button" class="ws-gate__submit" :disabled="busy" @click="onSubmit">
        {{ busy ? t('workspace.gateSubmitting') : t('workspace.gateSubmit') }}
      </button>
    </div>
  </div>
</template>

<style scoped>
.ws-gate {
  position: fixed;
  inset: 0;
  z-index: 99990;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  background: rgba(0, 0, 0, 0.35);
  backdrop-filter: blur(8px);
}

.ws-gate__panel {
  width: min(420px, 100%);
  padding: var(--space-5);
  border-radius: 16px;
}

.ws-gate__title {
  margin: 0 0 var(--space-4);
  font-size: 1.25rem;
  font-weight: 650;
  letter-spacing: -0.02em;
}

.ws-gate__input {
  width: 100%;
  box-sizing: border-box;
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid var(--color-border, rgba(60, 60, 67, 0.18));
  background: var(--color-surface, rgba(255, 255, 255, 0.85));
  font-family: inherit;
  font-size: 0.9375rem;
  font-weight: 600;
  color: var(--color-text);
  margin-bottom: var(--space-3);
}

.ws-gate__input::placeholder {
  color: var(--color-text-secondary);
  font-weight: 600;
}

.ws-gate__error {
  margin: 0 0 var(--space-2);
  font-size: 0.8125rem;
  font-weight: 600;
  color: #c00;
}

.ws-gate__submit {
  width: 100%;
  margin-top: var(--space-2);
  padding: 10px 14px;
  border: none;
  border-radius: 10px;
  font-family: inherit;
  font-size: 0.9375rem;
  font-weight: 650;
  cursor: pointer;
  background: var(--color-accent, #007aff);
  color: #fff;
}

.ws-gate__submit:disabled {
  opacity: 0.6;
  cursor: not-allowed;
}

.ws-gate__samples {
  margin-top: var(--space-3);
}

.ws-gate__samples-title {
  margin: 0 0 var(--space-2);
  font-size: 0.75rem;
  font-weight: 650;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  gap: var(--space-2);
  flex-wrap: wrap;
}

.ws-gate__samples-copied {
  color: #2e7d32;
}

.ws-gate__sample {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  width: 100%;
  box-sizing: border-box;
  margin-bottom: var(--space-2);
  padding: 8px 10px;
  border-radius: 10px;
  border: 1px solid var(--color-border, rgba(60, 60, 67, 0.18));
  background: var(--color-surface, rgba(255, 255, 255, 0.65));
  font-family: inherit;
  text-align: left;
  cursor: pointer;
  transition: border-color 0.15s ease, background 0.15s ease;
}

.ws-gate__sample:hover {
  border-color: var(--color-accent, #007aff);
  background: var(--color-surface, rgba(255, 255, 255, 0.9));
}

.ws-gate__sample--active {
  border-color: #2e7d32;
}

.ws-gate__sample-key {
  flex-shrink: 0;
  font-size: 0.8125rem;
  font-weight: 700;
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  color: var(--color-accent, #007aff);
  word-break: break-all;
}

.ws-gate__sample-content {
  flex: 1;
  min-width: 0;
  font-size: 0.8125rem;
  font-weight: 600;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.ws-gate__sample-docs {
  flex-shrink: 0;
  font-size: 0.75rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}
</style>
