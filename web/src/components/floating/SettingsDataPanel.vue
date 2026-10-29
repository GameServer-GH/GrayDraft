<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import FormConfirmDialog from '@/components/form/FormConfirmDialog.vue'
import { useI18n } from '@/composables/useI18n'
import {
  clearWorkspaceData,
  fetchWorkspaceDataStats,
  formatStorageBytes,
  type ClearWorkspaceDataTarget,
  type WorkspaceDataStats,
} from '@/services/workspaceData'
import { useUiStore } from '@/stores/ui'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import { afterKnowledgeDataCleared } from '@/composables/useKnowledgeLibrarySync'
import { listMemories, memorySummary, revokeMemory, type WorkspaceMemory } from '@/services/workspaceMemory'

const { t } = useI18n()
const ui = useUiStore()
const workspaceSession = useWorkspaceSessionStore()

const loading = ref(false)
const clearing = ref<ClearWorkspaceDataTarget | null>(null)
const confirmDialogOpen = ref(false)
const pendingClearTarget = ref<ClearWorkspaceDataTarget | null>(null)
const confirmBusy = ref(false)
const error = ref<string | null>(null)
const memories = ref<WorkspaceMemory[]>([])
const memoryError = ref<string | null>(null)
const revokingId = ref<string | null>(null)
const stats = ref<WorkspaceDataStats>({
  local: { threads: 0, bytes: 0 },
  knowledge_documents: 0,
  redis_retrieval_entries: 0,
  redis_answer_entries: 0,
  nexus_available: true,
})

const confirmDialogZ = computed(() => ui.settingsModalZIndex + 500)

const redisTotal = computed(
  () => stats.value.redis_retrieval_entries + stats.value.redis_answer_entries,
)

const confirmDialogTitle = computed(() => {
  const target = pendingClearTarget.value
  if (target === 'local') return t('settings.dataConfirmLocalTitle')
  if (target === 'knowledge') return t('settings.dataConfirmKnowledgeTitle')
  if (target === 'redis') return t('settings.dataConfirmRedisTitle')
  return ''
})

const confirmDialogMessage = computed(() => {
  const target = pendingClearTarget.value
  if (target === 'local') return t('settings.dataConfirmLocalBody')
  if (target === 'knowledge') return t('settings.dataConfirmKnowledgeBody')
  if (target === 'redis') return t('settings.dataConfirmRedisBody')
  return ''
})

function memoryKindLabel(kind: string): string {
  if (kind === 'preference') return t('settings.memoryKindPreference')
  if (kind === 'stable_fact') return t('settings.memoryKindFact')
  if (kind === 'task_state') return t('settings.memoryKindTask')
  if (kind === 'feedback') return t('settings.memoryKindFeedback')
  return kind
}

async function loadMemories() {
  memoryError.value = null
  try {
    memories.value = await listMemories()
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : String(e)
    memories.value = []
  }
}

async function onRevokeMemory(id: string) {
  if (revokingId.value) return
  revokingId.value = id
  memoryError.value = null
  try {
    await revokeMemory(id)
    memories.value = memories.value.filter((item) => item.memory_id !== id)
  } catch (e) {
    memoryError.value = e instanceof Error ? e.message : String(e)
  } finally {
    revokingId.value = null
  }
}

async function refresh() {
  loading.value = true
  error.value = null
  try {
    stats.value = await fetchWorkspaceDataStats()
    await loadMemories()
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    loading.value = false
  }
}

function requestClear(target: ClearWorkspaceDataTarget) {
  pendingClearTarget.value = target
  confirmDialogOpen.value = true
}

function onCancelClear() {
  pendingClearTarget.value = null
}

async function onConfirmClear() {
  const target = pendingClearTarget.value
  if (!target || confirmBusy.value) return
  confirmBusy.value = true
  clearing.value = target
  error.value = null
  try {
    await clearWorkspaceData(target)
    if (target === 'knowledge' && workspaceSession.nexusKbType) {
      await afterKnowledgeDataCleared()
    }
    await refresh()
    confirmDialogOpen.value = false
    pendingClearTarget.value = null
  } catch (e) {
    error.value = e instanceof Error ? e.message : String(e)
  } finally {
    confirmBusy.value = false
    clearing.value = null
  }
}

onMounted(() => {
  void refresh()
})
</script>

<template>
  <section class="data-panel">
    <div class="data-panel__head">
      <h2 class="data-panel__title">{{ t('settings.dataHeading') }}</h2>
      <button
        type="button"
        class="data-panel__refresh"
        :disabled="loading"
        :aria-label="t('settings.dataRefreshAria')"
        :title="t('settings.dataRefreshAria')"
        @click="refresh"
      >
        <Icon name="refresh" :size="16" />
      </button>
    </div>

    <p v-if="error" class="data-panel__err" role="alert">{{ error }}</p>
    <p v-if="!stats.nexus_available" class="data-panel__warn" role="status">
      {{ t('settings.dataNexusUnavailable') }}
    </p>

    <ul class="data-panel__list" role="list">
      <li class="data-panel__item glass-chip" role="listitem">
        <div class="data-panel__item-main">
          <h3 class="data-panel__item-title">{{ t('settings.dataLocalTitle') }}</h3>
          <p class="data-panel__item-stat">
            {{
              t('settings.dataLocalStat', {
                threads: String(stats.local.threads),
                size: formatStorageBytes(stats.local.bytes),
              })
            }}
          </p>
        </div>
        <button
          type="button"
          class="data-panel__clear-btn"
          :disabled="clearing !== null || stats.local.threads === 0"
          @click="requestClear('local')"
        >
          {{ clearing === 'local' ? t('settings.dataClearing') : t('settings.dataClearBtn') }}
        </button>
      </li>

      <li class="data-panel__item glass-chip" role="listitem">
        <div class="data-panel__item-main">
          <h3 class="data-panel__item-title">{{ t('settings.dataKnowledgeTitle') }}</h3>
          <p class="data-panel__item-stat">
            {{ t('settings.dataKnowledgeStat', { count: String(stats.knowledge_documents) }) }}
          </p>
        </div>
        <button
          type="button"
          class="data-panel__clear-btn"
          :disabled="clearing !== null || !stats.nexus_available || stats.knowledge_documents === 0"
          @click="requestClear('knowledge')"
        >
          {{ clearing === 'knowledge' ? t('settings.dataClearing') : t('settings.dataClearBtn') }}
        </button>
      </li>

      <li class="data-panel__item glass-chip" role="listitem">
        <div class="data-panel__item-main">
          <h3 class="data-panel__item-title">{{ t('settings.dataRedisTitle') }}</h3>
          <p class="data-panel__item-stat">
            {{
              t('settings.dataRedisStat', {
                retrieval: String(stats.redis_retrieval_entries),
                answer: String(stats.redis_answer_entries),
                total: String(redisTotal),
              })
            }}
          </p>
        </div>
        <button
          type="button"
          class="data-panel__clear-btn"
          :disabled="clearing !== null || !stats.nexus_available || redisTotal === 0"
          @click="requestClear('redis')"
        >
          {{ clearing === 'redis' ? t('settings.dataClearing') : t('settings.dataClearBtn') }}
        </button>
      </li>
    </ul>

    <section class="data-panel__memory" :aria-label="t('settings.memoryHeading')">
      <h3 class="data-panel__item-title">{{ t('settings.memoryHeading') }}</h3>
      <p v-if="memoryError" class="data-panel__err" role="alert">{{ memoryError }}</p>
      <p v-else-if="memories.length === 0" class="data-panel__item-desc">{{ t('settings.memoryEmpty') }}</p>
      <ul v-else class="data-panel__memory-list" role="list">
        <li v-for="item in memories" :key="item.memory_id" class="data-panel__memory-row">
          <div class="data-panel__memory-main">
            <p class="data-panel__memory-subject">{{ item.subject }}</p>
            <p class="data-panel__item-desc">{{ memoryKindLabel(item.kind) }} · {{ memorySummary(item) }}</p>
          </div>
          <button
            type="button"
            class="data-panel__clear-btn"
            :disabled="revokingId !== null"
            @click="onRevokeMemory(item.memory_id)"
          >
            {{ revokingId === item.memory_id ? t('settings.dataClearing') : t('settings.memoryRevoke') }}
          </button>
        </li>
      </ul>
    </section>

    <FormConfirmDialog
      v-model="confirmDialogOpen"
      :title="confirmDialogTitle"
      :message="confirmDialogMessage"
      :confirm-label="t('settings.dataConfirmOk')"
      :cancel-label="t('common.cancel')"
      confirm-danger
      :z-index="confirmDialogZ"
      :busy="confirmBusy"
      @confirm="onConfirmClear"
      @cancel="onCancelClear"
    />
  </section>
</template>

<style scoped>
.data-panel {
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.data-panel__memory {
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
}

.data-panel__memory-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-2);
  max-height: 280px;
  overflow: auto;
}

.data-panel__memory-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
}

.data-panel__memory-main {
  min-width: 0;
}

.data-panel__memory-subject {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text);
}

.data-panel__head {
  position: relative;
  padding-right: 40px;
}

.data-panel__title {
  margin: 0 0 var(--space-2);
  font-size: 18px;
  font-weight: 700;
}

.data-panel__intro {
  margin: 0;
  font-size: 14px;
  line-height: 1.55;
  color: var(--color-text-secondary);
}

.data-panel__refresh {
  position: absolute;
  top: 0;
  right: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 32px;
  height: 32px;
  padding: 0;
  border: 1px solid var(--color-border);
  border-radius: 8px;
  background: rgba(255, 255, 255, 0.75);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.data-panel__refresh:hover:not(:disabled) {
  border-color: rgba(0, 122, 255, 0.35);
  color: var(--color-accent);
}

.data-panel__refresh:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.data-panel__err {
  margin: 0;
  font-size: 13px;
  color: #c0392b;
}

.data-panel__warn {
  margin: 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.data-panel__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--space-3);
}

.data-panel__item {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  padding: var(--space-3);
}

.data-panel__item-main {
  min-width: 0;
  flex: 1;
}

.data-panel__item-title {
  margin: 0 0 4px;
  font-size: 15px;
  font-weight: 650;
}

.data-panel__item-desc {
  margin: 0 0 8px;
  font-size: 13px;
  line-height: 1.5;
  color: var(--color-text-secondary);
}

.data-panel__item-stat {
  margin: 0;
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text);
}

.data-panel__clear-btn {
  flex-shrink: 0;
  align-self: center;
  padding: 8px 14px;
  border-radius: 8px;
  border: 1px solid rgba(192, 57, 43, 0.35);
  background: rgba(255, 255, 255, 0.85);
  color: #c0392b;
  font: inherit;
  font-size: 13px;
  font-weight: 600;
  cursor: pointer;
}

.data-panel__clear-btn:hover:not(:disabled) {
  background: rgba(192, 57, 43, 0.08);
}

.data-panel__clear-btn:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
</style>
