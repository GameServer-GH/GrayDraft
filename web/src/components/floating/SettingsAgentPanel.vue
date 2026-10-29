<script setup lang="ts">
import { onMounted } from 'vue'
import Icon from '@/components/common/Icon.vue'
import { useI18n } from '@/composables/useI18n'
import { useAgentConfigStore } from '@/stores/agentConfig'

const { t } = useI18n()
const agentCfg = useAgentConfigStore()

onMounted(() => {
  void agentCfg.refreshFromServer()
})

function onUnstructuredInput(ev: Event) {
  const el = ev.target as HTMLTextAreaElement
  agentCfg.setUnstructuredPrompt(el.value)
}

function entitiesText(): string {
  return (agentCfg.contextProfile.entities ?? []).join('、')
}

function rulesText(): string {
  return (agentCfg.contextProfile.rules ?? []).join('\n')
}

function negativeText(): string {
  return (agentCfg.contextProfile.negative_context ?? []).join('、')
}

function onDescriptionInput(ev: Event) {
  agentCfg.setContextProfile({ description: (ev.target as HTMLInputElement).value })
}

function onEntitiesInput(ev: Event) {
  agentCfg.setEntitiesFromText((ev.target as HTMLTextAreaElement).value)
}

function onRulesInput(ev: Event) {
  agentCfg.setRulesFromText((ev.target as HTMLTextAreaElement).value)
}

function onNegativeInput(ev: Event) {
  agentCfg.setNegativeFromText((ev.target as HTMLTextAreaElement).value)
}
</script>

<template>
  <section class="agent-config">
    <h2 class="agent-config__h2">{{ t('settings.agentConfigHeading') }}</h2>

    <div
      class="agent-config__status"
      :class="agentCfg.multimodalEnabled ? 'agent-config__status--ok' : 'agent-config__status--warn'"
      role="status"
    >
      <Icon
        class="agent-config__status-icon"
        :name="agentCfg.multimodalEnabled ? 'circle-check' : 'alert-triangle'"
        :size="16"
      />
      <span>
        {{
          agentCfg.multimodalEnabled
            ? t('settings.agentMultimodalReady')
            : t('settings.agentMultimodalUnavailable')
        }}
      </span>
    </div>

    <!-- 非结构化 Prompt -->
    <h3 class="agent-config__h3">{{ t('settings.agentUnstructuredPromptTitle') }}</h3>
    <label class="agent-config__field" for="agent-unstructured-prompt">
      <textarea
        id="agent-unstructured-prompt"
        class="agent-config__textarea"
        :value="agentCfg.unstructuredPrompt"
        rows="10"
        :aria-label="t('settings.agentUnstructuredPromptTitle')"
        :placeholder="t('settings.agentUnstructuredPromptPlaceholder')"
        @input="onUnstructuredInput"
      />
    </label>

    <!-- 结构化上下文 -->
    <h3 class="agent-config__h3">{{ t('settings.agentContextTitle') }}</h3>
    <div class="agent-config__grid">
      <label class="agent-config__field agent-config__field--span" for="agent-context-desc">
        <span class="agent-config__label">{{ t('settings.agentContextDescriptionLabel') }}</span>
        <input
          id="agent-context-desc"
          class="agent-config__input"
          type="text"
          :value="agentCfg.contextProfile.description"
          :placeholder="t('settings.agentContextDescriptionPlaceholder')"
          @input="onDescriptionInput"
        />
      </label>
      <label class="agent-config__field" for="agent-context-entities">
        <span class="agent-config__label">{{ t('settings.agentContextEntitiesLabel') }}</span>
        <textarea
          id="agent-context-entities"
          class="agent-config__textarea agent-config__textarea--sm"
          :value="entitiesText()"
          rows="3"
          :placeholder="t('settings.agentContextEntitiesPlaceholder')"
          @input="onEntitiesInput"
        />
      </label>
      <label class="agent-config__field" for="agent-context-negative">
        <span class="agent-config__label">{{ t('settings.agentContextNegativeLabel') }}</span>
        <textarea
          id="agent-context-negative"
          class="agent-config__textarea agent-config__textarea--sm"
          :value="negativeText()"
          rows="3"
          :placeholder="t('settings.agentContextNegativePlaceholder')"
          @input="onNegativeInput"
        />
      </label>
      <label class="agent-config__field agent-config__field--span" for="agent-context-rules">
        <span class="agent-config__label">{{ t('settings.agentContextRulesLabel') }}</span>
        <textarea
          id="agent-context-rules"
          class="agent-config__textarea agent-config__textarea--md"
          :value="rulesText()"
          rows="5"
          :placeholder="t('settings.agentContextRulesPlaceholder')"
          @input="onRulesInput"
        />
      </label>
    </div>

    <!-- 基础 Prompt（只读） -->
    <h3 class="agent-config__h3">{{ t('settings.agentStructuredBaseTitle') }}</h3>
    <label class="agent-config__field" for="agent-structured-base">
      <textarea
        id="agent-structured-base"
        class="agent-config__textarea agent-config__textarea--readonly"
        :value="agentCfg.basePrompt"
        readonly
        rows="8"
        :aria-label="t('settings.agentStructuredBaseTitle')"
      />
    </label>
  </section>
</template>

<style scoped>
.agent-config {
  width: 100%;
  min-width: 0;
  box-sizing: border-box;
}

.agent-config__h2 {
  margin: 0 0 var(--space-2);
  font-size: 17px;
  font-weight: 650;
  color: var(--color-text);
  letter-spacing: -0.01em;
}

.agent-config__h3 {
  margin: var(--space-4) 0 var(--space-2);
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.agent-config__p {
  margin: 0 0 var(--space-3);
  font-size: 14px;
  line-height: 1.55;
  color: var(--color-text);
}

.agent-config__p--muted {
  color: var(--color-text-secondary);
}

.agent-config__p--tight {
  margin-top: calc(var(--space-1) * -1);
}

.agent-config__status {
  display: flex;
  align-items: flex-start;
  gap: 10px;
  margin: 0 0 var(--space-1);
  padding: 10px 12px;
  border-radius: 10px;
  border: 1px solid transparent;
  font-size: 13px;
  font-weight: 600;
  line-height: 1.5;
}

.agent-config__status-icon {
  flex-shrink: 0;
  margin-top: 1px;
}

.agent-config__status--ok {
  background: rgba(52, 199, 89, 0.08);
  border-color: rgba(52, 199, 89, 0.18);
  color: #1a7f37;
}

.agent-config__status--warn {
  background: rgba(255, 149, 0, 0.08);
  border-color: rgba(255, 149, 0, 0.2);
  color: #9a6700;
}

.agent-config__field {
  display: flex;
  flex-direction: column;
  gap: 8px;
  min-width: 0;
}

.agent-config__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.agent-config__input,
.agent-config__textarea {
  width: 100%;
  box-sizing: border-box;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  padding: 8px 10px;
  font: inherit;
  font-size: 14px;
  line-height: 1.55;
  color: var(--color-text);
  background: rgba(255, 255, 255, 0.75);
  transition:
    border-color 0.15s ease,
    box-shadow 0.15s ease;
}

.agent-config__textarea {
  resize: vertical;
  min-height: 120px;
}

.agent-config__textarea--sm {
  min-height: 72px;
}

.agent-config__textarea--md {
  min-height: 112px;
}

.agent-config__textarea--readonly {
  font-family: ui-monospace, SFMono-Regular, Menlo, monospace;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-secondary);
  background: rgba(60, 60, 67, 0.04);
  cursor: default;
  resize: vertical;
  min-height: 140px;
}

.agent-config__input:focus,
.agent-config__textarea:focus {
  outline: none;
  border-color: rgba(0, 122, 255, 0.4);
  box-shadow: 0 0 0 3px rgba(0, 122, 255, 0.08);
}

.agent-config__textarea--readonly:focus {
  border-color: var(--color-border);
  box-shadow: none;
}

.agent-config__grid {
  display: grid;
  grid-template-columns: 1fr;
  gap: var(--space-3);
  margin-bottom: var(--space-1);
}

@media (min-width: 720px) {
  .agent-config__grid {
    grid-template-columns: 1fr 1fr;
  }
}

.agent-config__field--span {
  grid-column: 1 / -1;
}

.agent-config__footnote {
  margin: var(--space-2) 0 0;
  font-size: 12px;
  line-height: 1.5;
  color: var(--color-text-secondary);
}
</style>
