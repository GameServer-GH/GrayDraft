<script setup lang="ts">
import { ref, watch, computed, onBeforeUnmount, onMounted, nextTick } from 'vue'
import {
  readNotebookDocument,
  sanitizeNotebookCss,
  getOrCreateNotebookShadowRoot,
  buildNotebookShadowShellCss,
} from '@/utils/notebook-html'
import { useNotebookDocCacheStore } from '@/stores/notebookDocCache'
import { useNotebookContentStore } from '@/stores/notebookContent'
import { useCanvasStore } from '@/stores/canvas'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import KnowledgeDetailPane from '@/components/canvas/KnowledgeDetailPane.vue'

const props = defineProps<{
  notebookId: string
}>()

const host = ref<HTMLElement | null>(null)
const contentStore = useNotebookContentStore()
const docCache = useNotebookDocCacheStore()
const canvas = useCanvasStore()
const session = useWorkspaceSessionStore()

const knowledgePreview = computed(
  () => canvas.notebooks.find((n) => n.id === props.notebookId)?.preview?.trim() ?? '',
)

/**
 * 正文随父级画布 world 的 scale 缩放；由 {@link NotebookCard} 的 `noteBodyCounterStyle` 固定为卡片 layout 尺寸。
 * v3.1：open Shadow DOM，样式与外部隔离；CSS 变量由宿主继承；便于调试与测量对齐。
 */
function renderKnowledgePreview(el: HTMLElement, text: string) {
  const sr = getOrCreateNotebookShadowRoot(el)
  while (sr.firstChild) {
    sr.removeChild(sr.firstChild)
  }
  const style = document.createElement('style')
  style.textContent = `
    :host { display: block; height: 100%; box-sizing: border-box; }
    .kb-preview {
      margin: 0;
      padding: 16px 18px;
      font: 14px/1.55 ui-sans-serif, system-ui, sans-serif;
      color: var(--color-text, #1c1c1e);
      white-space: pre-wrap;
      word-break: break-word;
      overflow: auto;
      max-height: 100%;
      box-sizing: border-box;
    }
  `
  sr.appendChild(style)
  const pre = document.createElement('p')
  pre.className = 'kb-preview'
  pre.textContent = text
  sr.appendChild(pre)
}

async function renderLoaded() {
  const el = host.value
  if (!el) return

  if (session.nexusKbType && knowledgePreview.value) {
    renderKnowledgePreview(el, knowledgePreview.value)
    return
  }

  await docCache.ensureLoaded(props.notebookId)
  const { html, css } = readNotebookDocument(props.notebookId)
  if (session.nexusKbType && !html.trim() && knowledgePreview.value) {
    renderKnowledgePreview(el, knowledgePreview.value)
    return
  }

  const safeCss = sanitizeNotebookCss(css)

  const sr = getOrCreateNotebookShadowRoot(el)
  while (sr.firstChild) {
    sr.removeChild(sr.firstChild)
  }

  const style = document.createElement('style')
  style.textContent = buildNotebookShadowShellCss(safeCss)
  sr.appendChild(style)

  const wrap = document.createElement('div')
  wrap.className = 'nb-surface'
  wrap.setAttribute('data-nb-host', props.notebookId)
  wrap.innerHTML = html
  sr.appendChild(wrap)
}

watch(
  () => [props.notebookId, contentStore.revision, knowledgePreview.value, session.nexusKbType] as const,
  () => {
    void nextTick(() => void renderLoaded())
  },
)

onMounted(() => {
  void nextTick(() => void renderLoaded())
})

onBeforeUnmount(() => {
  const el = host.value
  if (!el) return
  const sr = getOrCreateNotebookShadowRoot(el)
  while (sr.firstChild) {
    sr.removeChild(sr.firstChild)
  }
})
</script>

<template>
  <div class="notebook-html-pane-clip">
    <KnowledgeDetailPane v-if="session.nexusKbType" :notebook-id="notebookId" />
    <div v-else ref="host" class="notebook-html-pane agent-card-container" />
  </div>
</template>

<style scoped>
.notebook-html-pane-clip {
  flex: 1;
  min-height: 0;
  min-width: 0;
  width: 100%;
  height: 100%;
  position: relative;
  overflow: hidden;
  box-sizing: border-box;
}

/**
 * Shadow 宿主：用应用主题变量，避免整卡死白底把 Agent 渐变/玻璃衬成「发灰简笔画」；
 * 自定义属性会沿继承链进入 Shadow 宿主，供 notebook-html 内 :host token 桥接读取。
 */
.notebook-html-pane {
  width: 100%;
  height: 100%;
  min-height: 0;
  min-width: 0;
  box-sizing: border-box;
  background: var(--color-bg-elevated, #f5f5f7);
  color: var(--color-text, #1c1c1e);
}
</style>
