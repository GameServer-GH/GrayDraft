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
</style>
