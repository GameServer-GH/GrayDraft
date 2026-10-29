<script setup lang="ts">
import { computed } from 'vue'
import { ScrollAreaRoot, ScrollAreaViewport, ScrollAreaScrollbar, ScrollAreaThumb } from 'radix-vue'
import { useI18n } from '@/composables/useI18n'

const props = defineProps<{
  /** 覆盖默认「笔记目录」文案，知识库模式传入知识点目录 */
  ariaLabel?: string
}>()

const { t } = useI18n()
const resolvedAriaLabel = computed(() => props.ariaLabel ?? t('sidebar.aria'))
</script>

<template>
  <aside class="floating-sidebar glass-panel" :aria-label="resolvedAriaLabel">
    <ScrollAreaRoot class="floating-sidebar__scroll" type="hover" :scroll-hide-delay="400">
      <ScrollAreaViewport class="floating-sidebar__viewport">
        <div class="floating-sidebar__inner">
          <slot />
        </div>
      </ScrollAreaViewport>
      <ScrollAreaScrollbar class="floating-sidebar__scrollbar" orientation="vertical">
        <ScrollAreaThumb class="floating-sidebar__thumb" />
      </ScrollAreaScrollbar>
    </ScrollAreaRoot>
    <div v-if="$slots.footer" class="floating-sidebar__footer">
      <slot name="footer" />
    </div>
  </aside>
</template>

<style scoped>
.floating-sidebar {
  display: flex;
  flex-direction: column;
  width: 100%;
  max-width: 100%;
  box-sizing: border-box;
  height: 100%;
  min-height: 0;
  border-radius: 14px;
  padding: var(--space-3) var(--space-2) var(--space-3) var(--space-3);
}
.floating-sidebar__scroll {
  position: relative;
  flex: 1;
  min-height: 0;
  max-height: 100%;
  overflow-y: auto;
  height: 80vh;
}
.floating-sidebar__footer {
  flex-shrink: 0;
  margin-top: var(--space-2);
  padding-top: var(--space-2);
}
.floating-sidebar__viewport {
  height: 100%;
  max-height: 100%;
  padding-right: var(--space-1);
}
.floating-sidebar__inner {
  padding-right: var(--space-1);
}
.floating-sidebar__scrollbar {
  display: flex;
  user-select: none;
  touch-action: none;
  width: 10px;
  padding: 2px;
  background: rgba(60, 60, 67, 0.06);
  border-radius: 8px;
}
.floating-sidebar__thumb {
  flex: 1;
  background: rgba(60, 60, 67, 0.22);
  border-radius: 6px;
  min-height: 24px;
}
</style>
