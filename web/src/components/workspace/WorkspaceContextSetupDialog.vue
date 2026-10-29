<script setup lang="ts">
import { computed, ref, watch, nextTick } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useI18n } from '@/composables/useI18n'
import { useAgentConfigStore } from '@/stores/agentConfig'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'

const props = withDefaults(
  defineProps<{
    modelValue: boolean
    zIndex?: number
  }>(),
  { zIndex: 10_300 },
)

const emit = defineEmits<{
  'update:modelValue': [open: boolean]
  saved: []
}>()

const { t } = useI18n()
const agentCfg = useAgentConfigStore()

const description = ref('')
const busy = ref(false)
const enriching = ref(false)
const error = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

const headingId = 'workspace-context-setup-heading'

watch(
  () => props.modelValue,
  async (open) => {
    if (!open) return
    error.value = ''
    description.value = agentCfg.contextProfile.description ?? ''
    await nextTick()
    inputRef.value?.focus()
  },
)

const canSave = computed(() => description.value.trim().length >= 2)

async function onSave() {
  if (!canSave.value || busy.value) return
  busy.value = true
  enriching.value = true
  error.value = ''
  try {
    await agentCfg.enrichAndSaveFromBrief(description.value.trim())
    await useWorkspaceSessionStore().flushSettingsNow()
    emit('saved')
    emit('update:modelValue', false)
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    busy.value = false
    enriching.value = false
  }
}

function onLater() {
  emit('update:modelValue', false)
}
</script>

<template>
  <Teleport to="body">
    <div v-if="modelValue" class="ws-ctx-setup" :style="{ zIndex: String(zIndex) }">
      <div class="ws-ctx-setup__backdrop" aria-hidden="true" />
      <div
        role="dialog"
        aria-modal="true"
        class="ws-ctx-setup__panel glass-panel"
        :aria-labelledby="headingId"
      >
        <header class="ws-ctx-setup__header">
          <h2 :id="headingId" class="ws-ctx-setup__title">{{ t('workspaceContextSetup.title') }}</h2>
        </header>

        <div class="ws-ctx-setup__body">
          <label class="ws-ctx-setup__field" :for="headingId + '-input'">
            <input
              :id="headingId + '-input'"
              ref="inputRef"
              v-model="description"
              type="text"
              class="ws-ctx-setup__input"
              :aria-label="t('workspaceContextSetup.title')"
              :placeholder="t('workspaceContextSetup.descriptionPlaceholder')"
              @keydown.enter.prevent="onSave"
            />
          </label>
          <p v-if="error" class="ws-ctx-setup__error" role="alert">{{ error }}</p>
        </div>

        <footer class="ws-ctx-setup__footer">
          <button type="button" class="ws-ctx-setup__btn ws-ctx-setup__btn--ghost" :disabled="busy" @click="onLater">
            {{ t('workspaceContextSetup.later') }}
          </button>
          <button
            type="button"
            class="ws-ctx-setup__btn ws-ctx-setup__btn--primary"
            :disabled="!canSave || busy"
            @click="onSave"
          >
            <Icon v-if="busy" name="refresh" :size="16" class="ws-ctx-setup__spin" />
            {{ enriching ? t('workspaceContextSetup.enriching') : t('workspaceContextSetup.save') }}
          </button>
        </footer>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.ws-ctx-setup {
  position: fixed;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 24px;
}

.ws-ctx-setup__backdrop {
  position: absolute;
  inset: 0;
  background: rgba(0, 0, 0, 0.45);
}

.ws-ctx-setup__panel {
  position: relative;
  width: min(440px, 100%);
  display: flex;
  flex-direction: column;
  border-radius: 16px;
  overflow: hidden;
}

.ws-ctx-setup__header {
  padding: 20px 20px 4px;
}

.ws-ctx-setup__title {
  margin: 0 0 6px;
  font-size: 17px;
  font-weight: 650;
}

.ws-ctx-setup__intro {
  margin: 0;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-secondary);
}

.ws-ctx-setup__body {
  padding: 12px 20px 8px;
}

.ws-ctx-setup__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
}

.ws-ctx-setup__label {
  font-size: 14px;
  font-weight: 600;
}

.ws-ctx-setup__input {
  width: 100%;
  box-sizing: border-box;
  padding: 12px 14px;
  border-radius: 10px;
  border: 1px solid rgba(60, 60, 67, 0.14);
  font-size: 15px;
  font-family: inherit;
  background: rgba(255, 255, 255, 0.72);
}

.ws-ctx-setup__error {
  margin: 8px 0 0;
  font-size: 13px;
  color: #c41e3a;
}

.ws-ctx-setup__footer {
  display: flex;
  justify-content: flex-end;
  gap: 10px;
  padding: 12px 20px 20px;
}

.ws-ctx-setup__btn {
  padding: 10px 16px;
  border-radius: 10px;
  font-size: 14px;
  font-weight: 600;
  border: none;
  cursor: pointer;
}

.ws-ctx-setup__btn--ghost {
  background: rgba(60, 60, 67, 0.08);
  color: var(--color-text-secondary);
}

.ws-ctx-setup__btn--primary {
  background: var(--color-accent);
  color: #fff;
  display: inline-flex;
  align-items: center;
  gap: 6px;
}

.ws-ctx-setup__btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ws-ctx-setup__spin {
  animation: ws-ctx-spin 0.8s linear infinite;
}

@keyframes ws-ctx-spin {
  to {
    transform: rotate(360deg);
  }
}
</style>
