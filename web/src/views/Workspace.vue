<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import TopBar from '@/components/floating/TopBar.vue'
import Icon from '@/components/common/Icon.vue'
import Sidebar from '@/components/floating/Sidebar.vue'
import NotebookList from '@/components/floating/NotebookList.vue'
import KnowledgeListPanel from '@/components/knowledge/KnowledgeListPanel.vue'
import AgentDock from '@/components/floating/AgentDock.vue'
import CanvasRoot from '@/components/canvas/CanvasRoot.vue'
import SettingsView from '@/components/floating/SettingsView.vue'
import KnowledgeDetailDialog from '@/components/knowledge/KnowledgeDetailDialog.vue'
import WorkspaceContextSetupDialog from '@/components/workspace/WorkspaceContextSetupDialog.vue'
import { useI18n } from '@/composables/useI18n'
import { useKnowledgePanel } from '@/composables/useKnowledgePanel'
import { useKnowledgeIndexPoll } from '@/composables/useKnowledgeIndexPoll'
import { useCanvasStore } from '@/stores/canvas'
import { useKnowledgeDetailStore } from '@/stores/knowledgeDetail'
import { useUiStore } from '@/stores/ui'
import KnowledgeAddDialog from '@/components/form/KnowledgeAddDialog.vue'
import { galaxyViewScaleToPercent } from '@/utils/knowledgeGalaxy'
import type { ExtensionFieldRow } from '@/utils/extensionsEditor'
import type { KnowledgeExtensions } from '@/types/knowledge'
import {
  afterKnowledgeAdded,
  afterKnowledgeBulkChange,
  refreshKnowledgeLibrary,
} from '@/composables/useKnowledgeLibrarySync'
import { agentDockClampedWidth } from '@/utils/agentDockLayout'
import { useTourGuide, getGrayDraftTourSteps, shouldShowGrayDraftTour, markGrayDraftTourSeen } from '@/tour'
import TourOverlay from '@/tour/TourOverlay.vue'

const { t } = useI18n()
const canvasStore = useCanvasStore()
const canvasRootRef = ref<InstanceType<typeof CanvasRoot> | null>(null)
const ui = useUiStore()
const knowledge = useKnowledgePanel()

const addKnowledgeOpen = ref(false)
const addKnowledgeTitle = ref('')
const addKnowledgeContent = ref('')
const addKnowledgeSourceUrl = ref('')
const addKnowledgeExtensionRows = ref<ExtensionFieldRow[]>([])
const addKnowledgeStatus = ref('')
const addKnowledgeDialogZ = computed(() => ui.floatingZBase + 10_100)
const knowledgeDetailStore = useKnowledgeDetailStore()
const knowledgeDetailOpen = ref(false)
const knowledgeDetailDocId = ref<string | null>(null)
const knowledgeDetailChunkId = ref<string | null>(null)
const knowledgeDetailMediaUrl = ref<string | null>(null)
const knowledgeDetailDialogZ = computed(() => ui.floatingZBase + 10_150)
const workspaceContextSetupZ = computed(() => ui.floatingZBase + 10_400)

const topBarZ = computed(() => ui.floatingZBase + 10)
/** 画布 FAB：叠在顶栏 / 侧栏 / Agent 等所有工作区浮动层之上 */
const canvasFabZ = computed(() => ui.floatingZBase + 50)

/** ===== 新手指南 Tour ===== */
const tourSteps = getGrayDraftTourSteps({
  openSidebar: () => ui.setSidebarOpen(true),
  openAgent: () => ui.setAgentConversationOpen(true),
})
const tour = useTourGuide(tourSteps)
const hasTourQuery = ref(false)

function activateTourIfNeeded() {
  if (!shouldShowGrayDraftTour()) return
  hasTourQuery.value = true
  setTimeout(() => tour.start(), 600)
}

watch(() => tour.isActive.value, (active) => {
  if (!active && hasTourQuery.value) {
    hasTourQuery.value = false
    markGrayDraftTourSeen()
    const url = new URL(window.location.href)
    if (url.searchParams.has('tour')) {
      url.searchParams.delete('tour')
      window.history.replaceState({}, '', url.toString())
    }
  }
})

const agentHostStyle = computed(() => ({
  zIndex: ui.agentConversationZIndex,
  width: agentDockClampedWidth(ui.agentDockWidthPx),
}))

/** 画布缩放显示；星系模式 100% 对应更高密度的内部 viewScale。 */
const canvasZoomPercentLabel = computed(() => {
  const pct = knowledge.enabled.value
    ? galaxyViewScaleToPercent(canvasStore.viewScale)
    : Math.round(canvasStore.viewScale * 100)
  return `${pct}%`
})

const refreshing = ref(false)
const creatingNotebook = ref(false)
const notebookSearchQuery = ref('')

const sidebarSearchPlaceholder = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.searchPlaceholder') : t('workspace.searchPlaceholder'),
)
const sidebarSearchAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.searchAria') : t('workspace.searchAria'),
)
const recenterCanvasAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.recenterCanvasAria') : t('workspace.recenterCanvasAria'),
)
const recenterCanvasTitle = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.recenterCanvasTitle') : t('workspace.recenterCanvasTitle'),
)
const refreshListAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.refreshListAria') : t('workspace.refreshListAria'),
)
const newEntryAria = computed(() =>
  knowledge.enabled.value ? t('knowledge.workspace.newAria') : t('workspace.newNotebookAria'),
)

watch(
  () => ui.sidebarOpen,
  (open) => {
    if (!open) notebookSearchQuery.value = ''
  },
)

// 知识点过多时默认关闭画布（用户手动设置过开关则不覆盖）
watch(
  () => canvasStore.notebooks.length,
  (count) => ui.autoDisableCanvasForLibrarySize(count),
  { immediate: true },
)

async function loadNotebooksFromServer(syncGalaxy = true) {
  try {
    await refreshKnowledgeLibrary(syncGalaxy ? { type: 'list-sync' } : false)
  } catch (e) {
    canvasStore.setNotebookLoadError(knowledge.formatError(e))
  }
}

function onRecenterCanvas() {
  if (knowledge.enabled.value && canvasRootRef.value) {
    canvasRootRef.value.focusActiveGalaxyNode()
    return
  }
  canvasStore.recenterViewToActiveNotebookOrOrigin()
}

async function onRefreshNotebooks() {
  if (refreshing.value) return
  refreshing.value = true
  try {
    await refreshKnowledgeLibrary({ type: 'full' })
  } finally {
    refreshing.value = false
  }
}

function onOpenKnowledgeDetail(docId: string) {
  knowledgeDetailDocId.value = docId
  knowledgeDetailChunkId.value = null
  knowledgeDetailMediaUrl.value = null
  knowledgeDetailOpen.value = true
  canvasStore.selectNotebook(docId)
}

function onKnowledgeDetailNavigate(docId: string) {
  knowledgeDetailDocId.value = docId
  knowledgeDetailChunkId.value = null
  knowledgeDetailMediaUrl.value = null
  canvasStore.selectNotebook(docId)
}

watch(
  () => knowledgeDetailStore.citationOpen,
  (req) => {
    if (!req) return
    knowledgeDetailDocId.value = req.docId
    knowledgeDetailChunkId.value = req.chunkId
    knowledgeDetailMediaUrl.value = req.mediaUrl
    knowledgeDetailOpen.value = true
    canvasStore.selectNotebook(req.docId)
  },
)

function onNewEntryClick() {
  if (knowledge.enabled.value) {
    addKnowledgeTitle.value = ''
    addKnowledgeContent.value = ''
    addKnowledgeSourceUrl.value = ''
    addKnowledgeExtensionRows.value = []
    addKnowledgeStatus.value = ''
    addKnowledgeOpen.value = true
    return
  }
  void onCreateLegacyNotebook()
}

async function onCreateLegacyNotebook() {
  if (creatingNotebook.value) return
  creatingNotebook.value = true
  const anchorId = canvasStore.activeNotebookId
  try {
    const { createNotebook } = await import('@/services/notebook')
    const meta = await createNotebook({ title: t('common.untitledNotebook') })
    await loadNotebooksFromServer()
    canvasStore.placeNewNotebookAfterAnchor(meta.id, anchorId)
    canvasStore.selectNotebook(meta.id)
    canvasStore.recenterViewToActiveNotebookOrOrigin()
  } catch (e) {
    canvasStore.setNotebookLoadError(knowledge.formatError(e))
  } finally {
    creatingNotebook.value = false
  }
}

async function onAddKnowledgeConfirm(payload: {
  title: string
  content: string
  source_url: string
  extensions?: KnowledgeExtensions
}) {
  if (creatingNotebook.value) return
  creatingNotebook.value = true
  const anchorId = canvasStore.activeNotebookId
  addKnowledgeStatus.value = t('knowledge.addModal.statusIngesting')
  try {
    const { docId } = await knowledge.addEntry({
      title: payload.title,
      content: payload.content,
      source_url: payload.source_url || undefined,
      extensions: payload.extensions,
    })
    await afterKnowledgeAdded(docId)
    const pickId = docId && canvasStore.notebooks.some((n) => n.id === docId) ? docId : null
    if (pickId) {
      if (!knowledge.enabled.value) {
        canvasStore.placeNewNotebookAfterAnchor(pickId, anchorId)
      }
      canvasStore.selectNotebook(pickId)
    }
    if (!knowledge.enabled.value) {
      canvasStore.recenterViewToActiveNotebookOrOrigin()
    }
    addKnowledgeOpen.value = false
  } catch (e) {
    canvasStore.setNotebookLoadError(knowledge.formatError(e))
    addKnowledgeStatus.value = t('knowledge.addModal.statusFailed')
  } finally {
    creatingNotebook.value = false
  }
}

useKnowledgeIndexPoll(
  () => knowledge.enabled.value,
  async () => {
    await afterKnowledgeBulkChange()
  },
)

onMounted(async () => {
  // 知识库列表与 Agent 配置在 workspaceSession.openWithKey 已加载；星系图由 CanvasRoot useKnowledgeGalaxy 激活时拉取。
  if (!knowledge.enabled.value && canvasStore.notebooks.length === 0) {
    await loadNotebooksFromServer(false)
  }
  activateTourIfNeeded()
})
</script>

<template>
  <div class="workspace">
    <!-- 全视口底层画布：浮动层叠在其上，尺寸变化不挤压画布 -->
    <template v-if="!ui.canvasDisabled">
    <main class="workspace__canvas-layer">
      <CanvasRoot
        ref="canvasRootRef"
        @open-knowledge-detail="onOpenKnowledgeDetail"
      />
    </main>
    </template>
    <!-- 画布关闭时的空白区域：点击可取消选中知识点，切回工作区维度对话 -->
    <main
      v-else
      class="workspace__canvas-layer workspace__canvas-layer--off"
      :class="{ 'workspace__canvas-layer--deselected': !canvasStore.activeNotebookId }"
      role="button"
      :tabindex="canvasStore.activeNotebookId ? 0 : -1"
      :aria-label="t('workspace.canvasOffDeselectAria')"
      @click="canvasStore.selectNotebook(null)"
      @keydown.enter="canvasStore.selectNotebook(null)"
    >
      <p v-if="canvasStore.activeNotebookId" class="workspace__canvas-off-hint">
        {{ t('workspace.canvasOffHint') }}
      </p>
    </main>

    <div class="workspace__top">
      <TopBar />
    </div>

    <p
      v-if="!knowledge.enabled"
      class="workspace__legacy-banner"
      role="status"
    >
      {{ t('workspace.legacyHtmlBanner') }}
    </p>

    <aside
      v-show="ui.sidebarOpen"
      class="workspace__sidebar-host"
      :class="{ 'workspace__sidebar-host--knowledge': knowledge.enabled }"
      :style="{ zIndex: ui.floatingZBase }"
    >
        <div class="workspace__sidebar-search">
          <span class="workspace__sidebar-search-icon" aria-hidden="true">
            <Icon name="search" :size="16" />
          </span>
          <input
            v-model="notebookSearchQuery"
            type="search"
            class="workspace__sidebar-search-input"
            :placeholder="sidebarSearchPlaceholder"
            :aria-label="sidebarSearchAria"
            autocomplete="off"
            spellcheck="false"
          />
        </div>
        <Sidebar :aria-label="knowledge.enabled ? t('knowledge.sidebar.aria') : undefined">
          <KnowledgeListPanel
            v-if="knowledge.enabled"
            :search-query="notebookSearchQuery"
            @open-detail="onOpenKnowledgeDetail"
          />
          <NotebookList v-else :search-query="notebookSearchQuery" />
          <template #footer>
            <button
              type="button"
              class="workspace__new-notebook-btn"
              :disabled="creatingNotebook"
              :aria-label="newEntryAria"
              @click="onNewEntryClick"
            >
              <Icon class="workspace__new-notebook-icon" name="square-rounded-plus" :size="18" />
              <span>{{ knowledge.enabled ? t('workspace.newKnowledge') : t('workspace.newNotebook') }}</span>
            </button>
          </template>
        </Sidebar>
    </aside>

    <aside
      v-show="ui.agentConversationOpen"
      class="workspace__agent-host"
      :class="{ 'workspace__agent-host--expanded': ui.agentConversationExpanded }"
      :style="agentHostStyle"
    >
      <AgentDock />
    </aside>

    <div v-if="!ui.canvasDisabled" class="workspace__canvas-fab">
        <span
          class="glass-chip workspace__canvas-fab-chip workspace__canvas-fab-zoom"
          aria-live="polite"
          :title="t('workspace.canvasZoomTitle')"
        >
          {{ canvasZoomPercentLabel }}
        </span>
        <button
          type="button"
          class="glass-chip workspace__canvas-fab-chip workspace__canvas-fab-btn"
          :aria-label="recenterCanvasAria"
          :title="recenterCanvasTitle"
          @click="onRecenterCanvas"
        >
          <span class="workspace__canvas-fab-icon-wrap" aria-hidden="true">
            <Icon name="map-pin" :size="20" />
          </span>
        </button>
        <button
          type="button"
          class="glass-chip workspace__canvas-fab-chip workspace__canvas-fab-btn"
          :aria-label="refreshListAria"
          :title="t('workspace.refreshListTitle')"
          :disabled="refreshing"
          @click="onRefreshNotebooks"
        >
          <span class="workspace__canvas-fab-icon-wrap" aria-hidden="true">
            <Icon name="refresh" :size="20" />
          </span>
        </button>
    </div>

    <SettingsView v-if="ui.settingsOpen" />

    <WorkspaceContextSetupDialog
      v-if="knowledge.enabled"
      v-model="ui.workspaceContextSetupOpen"
      :z-index="workspaceContextSetupZ"
    />

    <KnowledgeDetailDialog
      v-if="knowledge.enabled"
      v-model="knowledgeDetailOpen"
      :document-id="knowledgeDetailDocId"
      :chunk-id="knowledgeDetailChunkId"
      :media-url="knowledgeDetailMediaUrl"
      :z-index="knowledgeDetailDialogZ"
      @navigate="onKnowledgeDetailNavigate"
    />

    <KnowledgeAddDialog
      v-if="knowledge.enabled"
      v-model="addKnowledgeOpen"
      v-model:title-text="addKnowledgeTitle"
      v-model:content-text="addKnowledgeContent"
      v-model:source-url="addKnowledgeSourceUrl"
      v-model:extension-rows="addKnowledgeExtensionRows"
      :title="t('knowledge.addModal.title')"
      :title-label="t('knowledge.addModal.titleLabel')"
      :content-label="t('knowledge.addModal.contentLabel')"
      :source-url-label="t('knowledge.addModal.sourceUrlLabel')"
      :extensions-label="t('knowledge.addModal.extensionsLabel')"
      :extension-key-placeholder="t('knowledge.addModal.extensionKeyPlaceholder')"
      :extension-value-placeholder="t('knowledge.addModal.extensionValuePlaceholder')"
      :add-extension-label="t('knowledge.addModal.addExtension')"
      :remove-extension-aria="t('knowledge.addModal.removeExtensionAria')"
      :confirm-label="t('knowledge.addModal.confirm')"
      :cancel-label="t('common.cancel')"
      :status-hint="addKnowledgeStatus"
      :z-index="addKnowledgeDialogZ"
      :busy="creatingNotebook"
      @confirm="onAddKnowledgeConfirm"
    />

    <!-- ===== 新手指南 —— 浮动引导卡 ===== -->
    <template v-if="hasTourQuery && tour.isActive.value">
      <TourOverlay
        :step="tour.currentStep.value"
        :step-index="tour.currentIndex.value"
        :total-steps="tourSteps.length"
        :is-first="tour.isFirst.value"
        :is-last="tour.isLast.value"
        :target-selector="tour.currentStep.value?.targetSelector"
        :spotlight-padding="tour.currentStep.value?.spotlightPadding ?? 8"
        :card-placement="tour.currentStep.value?.cardPlacement ?? 'auto'"
        :card-offset="tour.currentStep.value?.cardOffset ?? 16"
        @next="tour.next"
        @prev="tour.prev"
        @skip="tour.skip"
      />
    </template>
  </div>
</template>

<style scoped>
.workspace {
  --topbar-h: 56px;
  --float-gap: 12px;
  /* 右下角定位/刷新按钮行：距底 gap + 44px 芯片高，Agent 侧栏底部与此对齐留白 */
  --workspace-canvas-fab-band-h: calc(var(--float-gap) + 44px + var(--space-2));
  position: relative;
  min-height: 100%;
  height: 100%;
  background: radial-gradient(120% 80% at 50% 0%, #f5f5f7 0%, var(--color-bg) 55%);
  color: var(--color-text);
}
.workspace__canvas-layer {
  position: fixed;
  inset: 0;
  z-index: 0;
  display: flex;
  flex-direction: column;
  min-height: 0;
  padding: 0;
  margin: 0;
  box-sizing: border-box;
}

.workspace__canvas-layer--off {
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.workspace__canvas-layer--deselected {
  cursor: default;
}

.workspace__canvas-off-hint {
  margin: 0;
  padding: 10px 16px;
  max-width: min(520px, calc(100vw - 48px));
  font-size: 13px;
  line-height: 1.5;
  text-align: center;
  color: var(--color-text-secondary);
  background: rgba(255, 255, 255, 0.5);
  border: 1px solid rgba(60, 60, 67, 0.1);
  border-radius: 12px;
  pointer-events: none;
}

.workspace__legacy-banner {
  position: fixed;
  top: calc(var(--float-gap) + var(--topbar-h) + 6px);
  left: 50%;
  transform: translateX(-50%);
  z-index: calc(var(--float-gap) + 20);
  max-width: min(640px, calc(100vw - 32px));
  margin: 0;
  padding: 8px 14px;
  font-size: 12px;
  line-height: 1.45;
  text-align: center;
  color: #9a5b00;
  background: rgba(255, 149, 0, 0.12);
  border: 1px solid rgba(255, 149, 0, 0.28);
  border-radius: 10px;
  box-shadow: var(--shadow-float-sm);
}

.workspace__top {
  position: fixed;
  top: var(--float-gap);
  left: 0;
  right: 0;
  z-index: v-bind(topBarZ);
  pointer-events: none;
}

.workspace__top :deep(.floating-topbar) {
  pointer-events: auto;
}

.workspace__sidebar-host {
  position: fixed;
  top: calc(var(--topbar-h) + var(--float-gap));
  left: var(--float-gap);
  bottom: var(--float-gap);
  width: min(280px, calc(100vw - 2 * var(--float-gap)));
  max-width: calc(100vw - 2 * var(--float-gap));
  overflow: hidden;
  pointer-events: auto;
}

.workspace__sidebar-host--knowledge {
  width: min(320px, calc(100vw - 2 * var(--float-gap)));
}

.workspace__sidebar-search {
  position: absolute;
  top: var(--space-3);
  left: var(--space-3);
  right: var(--space-3);
  z-index: 2;
  display: flex;
  align-items: center;
  gap: 8px;
  box-sizing: border-box;
  min-width: 0;
  max-width: calc(100% - 2 * var(--space-3));
  height: 36px;
  padding: 0 10px 0 12px;
  border-radius: 10px;
  border: 1px solid var(--color-border);
  background: rgba(255, 255, 255, 0.88);
  box-shadow: var(--shadow-float-sm);
  overflow: hidden;
}

.workspace__sidebar-search-icon {
  display: flex;
  flex-shrink: 0;
  opacity: 0.55;
  line-height: 0;
}

.workspace__sidebar-search-input {
  flex: 1;
  width: 0;
  min-width: 0;
  max-width: 100%;
  height: 100%;
  padding: 0;
  border: none;
  background: transparent;
  font: inherit;
  font-size: 13px;
  color: var(--color-text);
  outline: none;
}

.workspace__sidebar-search-input::placeholder {
  color: var(--color-text-secondary);
  opacity: 0.85;
}

.workspace__sidebar-host :deep(.floating-sidebar) {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 100%;
  min-height: 0;
  height: 100%;
  box-sizing: border-box;
  padding: calc(var(--space-3) + 36px + var(--space-2)) var(--space-2) var(--space-3) var(--space-3);}

.workspace__sidebar-host :deep(.floating-sidebar__scroll) {
  flex: 1;
  min-height: 0;
}

.workspace__new-notebook-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  width: 100%;
  min-height: 40px;
  padding: 0 14px;
  border: none;
  border-radius: 10px;
  font: inherit;
  font-size: 14px;
  font-weight: 600;
  color: #fff;
  background: var(--color-accent);
  cursor: pointer;
  box-shadow: 0 4px 14px rgba(0, 122, 255, 0.35);
  transition:
    filter 0.14s ease,
    transform 0.12s ease,
    opacity 0.16s ease;
}

.workspace__new-notebook-btn:hover:not(:disabled) {
  filter: brightness(1.06);
}

.workspace__new-notebook-btn:active:not(:disabled) {
  transform: scale(0.98);
}

.workspace__new-notebook-btn:focus-visible {
  outline: none;
  box-shadow:
    0 4px 14px rgba(0, 122, 255, 0.35),
    0 0 0 3px rgba(0, 122, 255, 0.22);
}

.workspace__new-notebook-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.workspace__new-notebook-icon {
  flex-shrink: 0;
  display: block;
  color: inherit;
  opacity: 0.95;
}

.workspace__agent-host {
  position: fixed;
  top: calc(var(--topbar-h) + var(--float-gap));
  right: var(--float-gap);
  bottom: var(--workspace-canvas-fab-band-h);
  pointer-events: auto;
  transition: width 0.22s ease;
}

.workspace__canvas-fab {
  position: fixed;
  right: var(--float-gap);
  bottom: var(--float-gap);
  z-index: v-bind(canvasFabZ);
  display: flex;
  align-items: center;
  gap: var(--space-2);
  pointer-events: auto;
}

.workspace__canvas-fab-chip {
  min-height: 44px;
}

.workspace__canvas-fab-btn {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 44px;
  padding: 0;
  border: none;
  background: transparent;
  color: var(--color-text);
}

.workspace__canvas-fab-btn {
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

.workspace__canvas-fab-btn:hover:not(:disabled) {
  background: rgba(255, 255, 255, 0.35);
}

.workspace__canvas-fab-btn:disabled {
  opacity: 0.55;
  cursor: not-allowed;
}

.workspace__canvas-fab-icon-wrap {
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 0;
  opacity: 0.92;
}

.workspace__canvas-fab-zoom {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 3.25rem;
  font-size: 13px;
  font-weight: 600;
  font-variant-numeric: tabular-nums;
  letter-spacing: -0.02em;
  color: var(--color-text-secondary);
  user-select: none;
}
</style>
