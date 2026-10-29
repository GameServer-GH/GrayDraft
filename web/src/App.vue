<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { useI18n } from '@/composables/useI18n'
import Workspace from '@/views/Workspace.vue'
import WorkspaceKeyGate from '@/components/workspace/WorkspaceKeyGate.vue'
import { useUiStore } from '@/stores/ui'
import { getIsDesktop } from '@/services/bridge'
import { useWorkspaceSessionStore } from '@/stores/workspaceSession'
import { useCanvasStore } from '@/stores/canvas'

const ui = useUiStore()
const session = useWorkspaceSessionStore()
const { documentHtmlLang } = useI18n()

const bootstrapped = ref(false)
const showWorkspaceGate = computed(
  () => !getIsDesktop() && bootstrapped.value && !session.isWorkspaceReady,
)

watch(
  documentHtmlLang,
  (lang) => {
    document.documentElement.lang = lang
  },
  { immediate: true },
)

function flushCanvasViewBeforeLeave() {
  if (!session.isWorkspaceReady) return
  useCanvasStore().persistViewNow()
  void session.flushSettingsNow()
}

onMounted(async () => {
  session.registerSettingsSaver()
  window.addEventListener('beforeunload', flushCanvasViewBeforeLeave)
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'hidden') flushCanvasViewBeforeLeave()
  })
  if (getIsDesktop()) {
    await session.bootstrapDesktop()
  } else {
    await session.tryResumeFromStoredKey()
  }
  bootstrapped.value = true
})

/** 与 Workspace 顶栏一致：floatingZBase + 10 + 5，供底部输入条叠在顶栏之上 */
const composeZ = computed(() => ui.floatingZBase + 15)
</script>

<template>
  <WorkspaceKeyGate v-if="showWorkspaceGate" />
  <div v-else-if="!bootstrapped && !getIsDesktop()" class="app-boot" aria-busy="true" />
  <div v-else-if="bootstrapped && getIsDesktop() && !session.isWorkspaceReady" class="app-boot app-boot--error">
    {{ session.openError || 'Workspace unavailable' }}
  </div>
  <template v-else-if="bootstrapped && (session.isWorkspaceReady || getIsDesktop())">
    <div
      id="gd-compose-root"
      class="gd-compose-root"
      :style="{ zIndex: composeZ }"
      v-show="ui.composeBarVisible"
    />
    <Workspace />
  </template>
</template>

<style scoped>
.gd-compose-root {
  position: fixed;
  left: 50%;
  bottom: var(--space-3);
  transform: translateX(-50%);
  width: min(720px, calc(100vw - 2 * var(--space-3)));
  pointer-events: none;
}

.gd-compose-root > * {
  pointer-events: auto;
}
.app-boot {
  position: fixed;
  inset: 0;
  background: var(--color-bg, #f5f5f7);
}
.app-boot--error {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: var(--space-4);
  font-size: 0.875rem;
  color: #c00;
}
</style>
