<script setup lang="ts">
import { computed, ref } from 'vue'
import Icon from '@/components/common/Icon.vue'
import TipsMenu from '@/components/form/TipsMenu.vue'
import type { TipsMenuAction } from '@/components/form/TipsMenu.vue'
import FormConfirmDialog from '@/components/form/FormConfirmDialog.vue'
import FormTextDialog from '@/components/form/FormTextDialog.vue'
import KnowledgeEditDialog from '@/components/form/KnowledgeEditDialog.vue'
import type { ExtensionFieldRow } from '@/utils/extensionsEditor'
import type { KnowledgeExtensions } from '@/types/knowledge'
import { useI18n } from '@/composables/useI18n'
import { getIsDesktop } from '@/services/bridge'
import { useUiStore } from '@/stores/ui'
import { useCanvasStore } from '@/stores/canvas'
import { useAgentStore } from '@/stores/agent'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import { useKnowledgePanel } from '@/composables/useKnowledgePanel'
import { useKnowledgeDetailStore } from '@/stores/knowledgeDetail'
import {
  afterKnowledgeRemoved,
  afterKnowledgeUpdated,
} from '@/composables/useKnowledgeLibrarySync'
import { renameNotebook } from '@/services/notebook'
import { clearNotebookBrowserStorage } from '@/utils/notebook-html'

const { t } = useI18n()
const ui = useUiStore()
const canvas = useCanvasStore()
const agent = useAgentStore()
const workspaceSession = useWorkspaceSessionStore()
const knowledge = useKnowledgePanel()
const knowledgeDetail = useKnowledgeDetailStore()

const docTitle = computed(() => canvas.activeNotebook?.title ?? t('topbar.fallbackDocTitle'))
const selectedNotebookTitle = computed(() => canvas.activeNotebook?.title ?? '')

const toggleSidebarAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.topbar.toggleSidebarAria') : t('topbar.toggleSidebarAria'),
)
const toggleSidebarTitle = computed(() =>
  knowledge.enabled.value ? t('knowledge.topbar.toggleSidebarTitle') : t('topbar.toggleSidebarTitle'),
)
const centerRegionAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.topbar.centerAria') : t('topbar.notebookCenterAria'),
)
const deleteConfirmTitle = computed(() =>
  knowledge.enabled.value ? t('knowledge.deleteConfirmTitle') : t('notebook.deleteConfirmTitle'),
)
const deleteConfirmBody = computed(() =>
  knowledge.enabled.value
    ? t('knowledge.deleteConfirmBody', { title: selectedNotebookTitle.value })
    : t('notebook.deleteConfirmBody', { title: selectedNotebookTitle.value }),
)
const renameDialogTitle = computed(() =>
  knowledge.enabled.value ? t('knowledge.renameDialogTitle') : t('notebook.renameDialogTitle'),
)
const renameFieldLabel = computed(() =>
  knowledge.enabled.value ? t('knowledge.renameFieldLabel') : t('notebook.renameFieldLabel'),
)

type TopBarCenterMode = 'idle' | 'composing' | 'generating' | 'notebook'

const centerMode = computed<TopBarCenterMode>(() => {
  if (agent.streaming) return 'generating'
  const id = canvas.activeNotebookId
  const draft = id ? (agent.threads[id]?.composerDraft ?? '').trim() : ''
  if (draft) return 'composing'
  if (canvas.activeNotebookId && canvas.activeNotebook) return 'notebook'
  return 'idle'
})

const topBarEditActions = computed<TipsMenuAction[]>(() => {
  const actions: TipsMenuAction[] = [{ id: 'rename', label: t('topbar.actionRename') }]
  if (knowledge.enabled.value) {
    actions.push({ id: 'modify', label: t('knowledge.editModal.title') })
  }
  return actions
})
const topBarPreviewActions = computed<TipsMenuAction[]>(() => [])
const topBarMoreActions = computed<TipsMenuAction[]>(() => [
  { id: 'delete', label: t('topbar.delete'), danger: true },
])

const editMenuPanelAria = computed(() => `${t('topbar.editAria')}${t('tipsMenu.menuSuffix')}`)
const previewMenuPanelAria = computed(() => `${t('topbar.previewAria')}${t('tipsMenu.menuSuffix')}`)
const moreMenuPanelAria = computed(() => `${t('topbar.moreAria')}${t('tipsMenu.menuSuffix')}`)

const composingPreview = computed(() => {
  const id = canvas.activeNotebookId
  const raw = (id ? agent.threads[id]?.composerDraft ?? '' : '').replace(/\s+/g, ' ').trim()
  const max = 80
  if (raw.length <= max) return raw
  return `${raw.slice(0, max)}…`
})

const agentConversationIcon = computed(() =>
  agent.streaming ? 'sparkle-highlight' : 'sparkle-2',
)
const agentConversationTitle = computed(() => {
  if (!agent.streaming) return t('topbar.agentTitleIdle')
  if (agent.streamingPhase === 'import') return t('topbar.agentTitleImporting')
  if (agent.streamingPhase === 'search') return t('topbar.agentTitleSearching')
  if (agent.streamingPhase === 'chat') return t('topbar.agentTitleChatting')
  return t('topbar.agentTitleBusy')
})

/** 与 Agent 输入区忙碌态同步，展示在顶栏中央 */
const agentBusyHint = computed(() => {
  if (!agent.streaming) return ''
  if (agent.streamingPhase === 'import') return t('agentInput.stateImporting')
  if (agent.streamingPhase === 'search') return t('agentInput.stateSearching')
  if (agent.streamingPhase === 'chat') return t('agentInput.stateChatting')
  return ''
})

const deleteNotebookDialogOpen = ref(false)
const deleteNotebookBusy = ref(false)

const switchWorkspaceDialogOpen = ref(false)
const switchWorkspaceBusy = ref(false)

const renameNotebookDialogOpen = ref(false)
const renameNotebookDraft = ref('')
const renameNotebookBusy = ref(false)

const editKnowledgeOpen = ref(false)
const editKnowledgeTitle = ref('')
const editKnowledgeContent = ref('')
const editKnowledgeSourceUrl = ref('')
const editKnowledgeExtensionRows = ref<ExtensionFieldRow[]>([])
const editKnowledgeBusy = ref(false)
const editKnowledgeDocId = ref<string | null>(null)

const deleteNotebookDialogZ = computed(() => ui.floatingZBase + 10_100)
const renameNotebookDialogZ = computed(() => ui.floatingZBase + 10_100)
const switchWorkspaceDialogZ = computed(() => ui.floatingZBase + 10_100)

function onMoreMenuAction(id: string) {
  if (id === 'delete') {
    deleteNotebookDialogOpen.value = true
  }
}

function onEditMenuAction(id: string) {
  if (id === 'rename') {
    renameNotebookDraft.value = selectedNotebookTitle.value
    renameNotebookDialogOpen.value = true
    return
  }
  if (id === 'modify' && knowledge.enabled.value) {
    const docId = canvas.activeNotebookId
    if (!docId) return
    editKnowledgeDocId.value = docId
    editKnowledgeOpen.value = true
  }
}

async function onRenameNotebookConfirm(draftTrimmed: string) {
  const id = canvas.activeNotebookId
  if (!id || renameNotebookBusy.value) return
  const title =
    draftTrimmed ||
    (knowledge.enabled.value ? t('common.untitledKnowledge') : t('common.untitledNotebook'))
  renameNotebookBusy.value = true
  try {
    if (knowledge.enabled.value) {
      const doc = await knowledge.fetchEntry(id)
      await knowledge.saveEntry({
        id,
        title,
        content: doc.content,
        source_url: doc.source_url,
        extensions: doc.extensions,
      })
      await afterKnowledgeUpdated(id, false)
      knowledgeDetail.invalidate(id)
      void knowledgeDetail.load(id, { force: true })
    } else {
      const meta = await renameNotebook({ id, title })
      canvas.patchNotebookMeta(meta)
    }
    renameNotebookDialogOpen.value = false
  } catch (e) {
    canvas.setNotebookLoadError(knowledge.formatError(e))
  } finally {
    renameNotebookBusy.value = false
  }
}

async function onEditKnowledgeConfirm(payload: {
  id: string
  title: string
  content: string
  source_url: string
  extensions?: KnowledgeExtensions
}) {
  if (editKnowledgeBusy.value) return
  editKnowledgeBusy.value = true
  try {
    await knowledge.saveEntry(payload)
    await afterKnowledgeUpdated(payload.id, true)
    knowledgeDetail.invalidate(payload.id)
    void knowledgeDetail.load(payload.id, { force: true })
    editKnowledgeOpen.value = false
  } catch (e) {
    canvas.setNotebookLoadError(knowledge.formatError(e))
  } finally {
    editKnowledgeBusy.value = false
  }
}

async function onSwitchWorkspaceConfirm() {
  if (switchWorkspaceBusy.value) return
  switchWorkspaceBusy.value = true
  try {
    await workspaceSession.leaveWorkspace()
    switchWorkspaceDialogOpen.value = false
  } catch (e) {
    canvas.setNotebookLoadError(knowledge.formatError(e))
  } finally {
    switchWorkspaceBusy.value = false
  }
}

async function onDeleteNotebookConfirm() {
  const id = canvas.activeNotebookId
  if (!id || deleteNotebookBusy.value) return
  deleteNotebookBusy.value = true
  try {
    await knowledge.removeEntry(id)
    if (!knowledge.enabled.value) {
      clearNotebookBrowserStorage(id)
    }
    agent.discardThread(id)
    if (knowledge.enabled.value) {
      await afterKnowledgeRemoved(id)
    } else {
      const list = await knowledge.loadEntries()
      canvas.setNotebooks(list)
    }
    canvas.recenterViewToActiveNotebookOrOrigin()
    deleteNotebookDialogOpen.value = false
  } catch (e) {
    canvas.setNotebookLoadError(knowledge.formatError(e))
  } finally {
    deleteNotebookBusy.value = false
  }
}

function onToggleSidebar() {
  ui.toggleSidebar()
}

function onToggleAgentConversation() {
  ui.toggleAgentConversation()
}

function onToggleSettings() {
  ui.toggleSettings()
}
</script>

<template>
  <div class="floating-topbar" role="banner">
    <div class="floating-topbar__cluster floating-topbar__cluster--left">
      <button
        v-if="!getIsDesktop()"
        type="button"
        class="glass-chip floating-topbar__chip floating-topbar__icon-btn"
        :aria-label="t('topbar.switchWorkspaceAria')"
        :title="t('topbar.switchWorkspaceTitle')"
        @click="switchWorkspaceDialogOpen = true"
      >
        <span class="floating-topbar__icon-wrap" aria-hidden="true">
          <Icon name="list-details" :size="20" />
        </span>
      </button>
      <button
        type="button"
        class="glass-chip floating-topbar__chip floating-topbar__icon-btn"
        :aria-pressed="ui.sidebarOpen"
        :aria-label="toggleSidebarAria"
        :title="toggleSidebarTitle"
        @click="onToggleSidebar"
      >
        <span class="floating-topbar__icon-wrap" aria-hidden="true">
          <Icon name="layout-sidebar" :size="20" />
        </span>
      </button>
    </div>

    <div
      class="floating-topbar__cluster floating-topbar__cluster--center"
      :class="{ 'floating-topbar__cluster--center--wide': centerMode === 'notebook' }"
    >
      <div
        class="glass-chip floating-topbar__chip floating-topbar__title-chip"
        :class="{
          'floating-topbar__title-chip--composing': centerMode === 'composing',
          'floating-topbar__title-chip--generating': centerMode === 'generating',
          'floating-topbar__title-chip--notebook': centerMode === 'notebook',
        }"
        :role="centerMode === 'notebook' ? 'region' : 'status'"
        :aria-live="
          centerMode === 'idle' || centerMode === 'notebook' ? 'off' : 'polite'
        "
        :aria-busy="centerMode === 'generating'"
        :aria-label="centerMode === 'notebook' ? centerRegionAria : undefined"
      >
        <template v-if="centerMode === 'idle'">
          <span class="floating-topbar__brand">GrayDraft</span>
          <span class="floating-topbar__sep" aria-hidden="true">/</span>
          <span class="floating-topbar__doc">{{ docTitle }}</span>
        </template>
        <template v-else-if="centerMode === 'composing'">
          <span class="floating-topbar__status-icon" aria-hidden="true">
            <Icon name="message-circle" :size="18" />
          </span>
          <span class="floating-topbar__status-col">
            <span class="floating-topbar__status-label">{{ t('topbar.statusComposing') }}</span>
            <span class="floating-topbar__status-detail">{{ composingPreview }}</span>
          </span>
        </template>
        <template v-else-if="centerMode === 'generating'">
          <span
            class="floating-topbar__status-icon floating-topbar__status-icon--pulse"
            aria-hidden="true"
          >
            <Icon name="sparkle-highlight" :size="18" />
          </span>
          <span class="floating-topbar__status-col">
            <span class="floating-topbar__status-label">{{ agentBusyHint }}</span>
            <span class="floating-topbar__status-detail">{{ docTitle }}</span>
          </span>
        </template>
        <template v-else>
          <span class="floating-topbar__nb-name">{{ selectedNotebookTitle }}</span>
          <div class="floating-topbar__nb-controls">
            <TipsMenu
              id="topbar-nb-edit"
              :ariaLabel="t('topbar.editAria')"
              :panelAriaLabel="editMenuPanelAria"
              :actions="topBarEditActions"
              @action="onEditMenuAction"
            />
            <TipsMenu
              v-if="topBarPreviewActions.length"
              id="topbar-nb-preview"
              :ariaLabel="t('topbar.previewAria')"
              :panelAriaLabel="previewMenuPanelAria"
              :actions="topBarPreviewActions"
            />
            <div class="floating-topbar__nb-more-wrap">
              <TipsMenu
                id="topbar-nb-more"
                :ariaLabel="t('topbar.moreAria')"
                :panelAriaLabel="moreMenuPanelAria"
                :actions="topBarMoreActions"
                @action="onMoreMenuAction"
              />
            </div>
          </div>
        </template>
      </div>
    </div>

    <FormConfirmDialog
      v-model="switchWorkspaceDialogOpen"
      :title="t('topbar.switchWorkspaceConfirmTitle')"
      :message="t('topbar.switchWorkspaceConfirmBody')"
      :confirm-label="t('topbar.switchWorkspaceTitle')"
      :cancel-label="t('common.cancel')"
      :z-index="switchWorkspaceDialogZ"
      :busy="switchWorkspaceBusy"
      @confirm="onSwitchWorkspaceConfirm"
    />

    <FormConfirmDialog
      v-model="deleteNotebookDialogOpen"
      :title="deleteConfirmTitle"
      :message="deleteConfirmBody"
      :confirm-label="t('topbar.delete')"
      :cancel-label="t('common.cancel')"
      confirm-danger
      :z-index="deleteNotebookDialogZ"
      :busy="deleteNotebookBusy"
      @confirm="onDeleteNotebookConfirm"
    />

    <FormTextDialog
      v-model="renameNotebookDialogOpen"
      v-model:text="renameNotebookDraft"
      :title="renameDialogTitle"
      :field-label="renameFieldLabel"
      :confirm-label="t('common.save')"
      :cancel-label="t('common.cancel')"
      :z-index="renameNotebookDialogZ"
      :busy="renameNotebookBusy"
      @confirm="onRenameNotebookConfirm"
    />

    <KnowledgeEditDialog
      v-if="knowledge.enabled"
      v-model="editKnowledgeOpen"
      v-model:title-text="editKnowledgeTitle"
      v-model:content-text="editKnowledgeContent"
      v-model:source-url="editKnowledgeSourceUrl"
      v-model:extension-rows="editKnowledgeExtensionRows"
      :document-id="editKnowledgeDocId"
      :title="t('knowledge.editModal.title')"
      :title-label="t('knowledge.addModal.titleLabel')"
      :content-label="t('knowledge.addModal.contentLabel')"
      :source-url-label="t('knowledge.addModal.sourceUrlLabel')"
      :extensions-label="t('knowledge.addModal.extensionsLabel')"
      :extension-key-placeholder="t('knowledge.addModal.extensionKeyPlaceholder')"
      :extension-value-placeholder="t('knowledge.addModal.extensionValuePlaceholder')"
      :add-extension-label="t('knowledge.addModal.addExtension')"
      :remove-extension-aria="t('knowledge.addModal.removeExtensionAria')"
      :loading-label="t('knowledge.editModal.loading')"
      :confirm-label="t('knowledge.editModal.confirm')"
      :cancel-label="t('common.cancel')"
      :z-index="renameNotebookDialogZ"
      :busy="editKnowledgeBusy"
      @confirm="onEditKnowledgeConfirm"
    />

    <div class="floating-topbar__cluster floating-topbar__cluster--right">
      <button
        type="button"
        class="glass-chip floating-topbar__chip floating-topbar__icon-btn"
        :aria-pressed="ui.agentConversationOpen"
        :aria-busy="agent.streaming"
        :aria-label="t('topbar.toggleAgentAria')"
        :title="agentConversationTitle"
        @click="onToggleAgentConversation"
      >
        <span
          class="floating-topbar__icon-wrap floating-topbar__icon-wrap--agent"
          :class="{ 'floating-topbar__icon-wrap--agent-pulse': agent.streaming }"
          aria-hidden="true"
        >
          <Icon :name="agentConversationIcon" :size="20" />
        </span>
      </button>
      <button
        type="button"
        class="glass-chip floating-topbar__chip floating-topbar__icon-btn"
        :aria-pressed="ui.settingsOpen"
        :aria-label="t('topbar.openSettingsAria')"
        :title="t('topbar.openSettingsTitle')"
        @click="onToggleSettings"
      >
        <span class="floating-topbar__icon-wrap" aria-hidden="true">
          <Icon name="settings-2" :size="20" />
        </span>
      </button>
    </div>
  </div>
</template>

<style scoped>
.floating-topbar {
  position: relative;
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-3);
  max-width: 99%;
  margin: 0 auto;
  padding: 0 var(--space-1);
  pointer-events: none;
}

.floating-topbar__cluster {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  pointer-events: auto;
  min-height: 44px;
}

.floating-topbar__cluster--left {
  flex: 0 0 auto;
  position: relative;
  z-index: 2;
}

/** 相对整条顶栏水平居中，不参与左右之间的 flex 分栏居中 */
.floating-topbar__cluster--center {
  position: absolute;
  left: 50%;
  top: 0;
  transform: translateX(-50%);
  flex: none;
  width: max-content;
  max-width: min(520px, calc(100% - 180px));
  justify-content: center;
  min-width: 0;
}

.floating-topbar__cluster--center--wide {
  max-width: min(920px, calc(100% - 180px));
}

.floating-topbar__cluster--right {
  flex: 0 0 auto;
  position: relative;
  z-index: 2;
}

.floating-topbar__chip {
  min-height: 44px;
}

.floating-topbar__title-chip {
  display: flex;
  align-items: center;
  gap: var(--space-2);
  padding: 0 var(--space-4);
  max-width: min(520px, 100%);
}

.floating-topbar__title-chip--composing,
.floating-topbar__title-chip--generating {
  align-items: flex-start;
  padding-top: 8px;
  padding-bottom: 8px;
}

.floating-topbar__status-icon {
  flex-shrink: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  margin-top: 1px;
  line-height: 0;
  opacity: 0.78;
  color: var(--color-text-secondary);
}

.floating-topbar__title-chip--generating .floating-topbar__status-icon {
  color: var(--color-text);
}

@keyframes floating-topbar-status-pulse {
  0%,
  100% {
    opacity: 0.72;
    transform: scale(1);
  }
  50% {
    opacity: 1;
    transform: scale(1.08);
  }
}

.floating-topbar__status-icon--pulse {
  animation: floating-topbar-status-pulse 1.15s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
  .floating-topbar__status-icon--pulse {
    animation: none;
  }
}

.floating-topbar__status-col {
  display: flex;
  flex-direction: column;
  gap: 2px;
  min-width: 0;
  text-align: left;
}

.floating-topbar__status-label {
  font-size: 12px;
  font-weight: 650;
  letter-spacing: -0.01em;
  color: var(--color-text);
}

.floating-topbar__status-detail {
  font-size: 12px;
  font-weight: 600;
  line-height: 1.35;
  color: var(--color-text-secondary);
  overflow: hidden;
  display: -webkit-box;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
  line-clamp: 2;
  word-break: break-word;
}

.floating-topbar__icon-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--color-text);
  cursor: pointer;
  transition:
    background 0.15s ease,
    box-shadow 0.15s ease,
    transform 0.12s ease;
}

.floating-topbar__icon-btn:hover {
  background: rgba(255, 255, 255, 0.35);
}

.floating-topbar__icon-btn[aria-pressed='true'] {
  color: var(--color-text);
  box-shadow: inset 0 0 0 1px rgba(28, 28, 30, 0.16);
  background: rgba(255, 255, 255, 0.78);
}

.floating-topbar__icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  opacity: 0.92;
}

.floating-topbar__icon-wrap--agent {
  transform-origin: center center;
}

@keyframes floating-topbar-agent-pulse {
  0%,
  100% {
    transform: scale(1);
  }
  50% {
    transform: scale(1.16);
  }
}

.floating-topbar__icon-wrap--agent-pulse {
  animation: floating-topbar-agent-pulse 1.15s ease-in-out infinite;
}

@media (prefers-reduced-motion: reduce) {
  .floating-topbar__icon-wrap--agent-pulse {
    animation: none;
  }
}

.floating-topbar__brand {
  font-weight: 700;
  font-size: 15px;
  letter-spacing: -0.02em;
  color: var(--color-text);
}

.floating-topbar__sep {
  color: var(--color-text-secondary);
  opacity: 0.85;
  font-size: 14px;
}

.floating-topbar__doc {
  font-size: 14px;
  font-weight: 600;
  color: var(--color-text-secondary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  min-width: 0;
}

.floating-topbar__title-chip--notebook {
  align-items: center;
  justify-content: space-between;
  gap: var(--space-4);
  padding: 5px var(--space-3) 5px var(--space-4);
  max-width: min(900px, 100%);
}

.floating-topbar__nb-name {
  flex: 1 1 auto;
  min-width: 0;
  font-size: 15px;
  font-weight: 650;
  letter-spacing: -0.02em;
  color: var(--color-text);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  padding-right: var(--space-2);
}

.floating-topbar__nb-controls {
  display: flex;
  flex-shrink: 0;
  align-items: center;
  gap: 2px;
}

.floating-topbar__nb-more-wrap {
  margin-left: var(--space-3);
  padding-left: var(--space-3);
}

.floating-topbar__title-chip--notebook :deep(.form-tips-menu) {
  flex-shrink: 0;
}

@media (max-width: 720px) {
  .floating-topbar__nb-controls {
    flex-wrap: wrap;
    justify-content: flex-end;
    max-width: 100%;
  }

  .floating-topbar__title-chip--notebook {
    flex-wrap: wrap;
    row-gap: var(--space-2);
  }

  .floating-topbar__nb-name {
    flex: 1 1 100%;
    max-width: 100%;
  }
}
</style>
