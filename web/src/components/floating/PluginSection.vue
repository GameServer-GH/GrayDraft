<script setup lang="ts">
import { Separator } from 'radix-vue'
import Icon from '@/components/common/Icon.vue'
import { usePluginsStore } from '@/stores/plugins'
import type { PluginKind } from '@/types/plugin'

const plugins = usePluginsStore()

function kindLabel(k: PluginKind): string {
  const labels: Record<PluginKind, string> = {
    'language-pack': '语言包',
    'html-notebook': 'HTML 笔记',
    'agent-skill': 'Agent 技能',
  }
  return labels[k]
}
</script>

<template>
  <div class="plugin-section">
    <div class="plugin-section__head">
      <span class="plugin-section__title">
        <Icon class="plugin-section__title-icon" name="code" :size="14" />
        插件
      </span>
    </div>
    <Separator class="plugin-section__sep" orientation="horizontal" decorative />
    <ul class="plugin-section__ul">
      <li v-for="p in plugins.installed" :key="p.id" class="plugin-section__row">
        <div class="plugin-section__info">
          <span class="plugin-section__name">{{ p.name }}</span>
          <span class="plugin-section__badge">{{ kindLabel(p.kind) }}</span>
          <span class="plugin-section__ver">v{{ p.version }}</span>
        </div>
        <button
          type="button"
          class="plugin-section__toggle"
          :aria-pressed="p.enabled"
          :title="p.enabled ? '点击禁用' : '点击启用'"
          @click="plugins.togglePlugin(p.id)"
        >
          <Icon
            class="plugin-section__toggle-icon"
            :name="p.enabled ? 'toggle-right' : 'toggle-left'"
            :size="14"
          />
          {{ p.enabled ? '开' : '关' }}
        </button>
      </li>
    </ul>
  </div>
</template>

<style scoped>
.plugin-section__head {
  margin-top: var(--space-4);
  margin-bottom: var(--space-2);
}
.plugin-section__title {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.04em;
  text-transform: uppercase;
  opacity: 0.65;
}
.plugin-section__title-icon {
  flex-shrink: 0;
  opacity: 0.85;
}
.plugin-section__sep {
  height: 1px;
  background: rgba(255, 255, 255, 0.12);
  margin: 0 0 var(--space-3);
}
.plugin-section__ul {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;
}
.plugin-section__row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: var(--space-2);
  padding: var(--space-2);
  border-radius: 8px;
  background: rgba(0, 0, 0, 0.15);
  border: 1px solid rgba(255, 255, 255, 0.06);
}
.plugin-section__info {
  min-width: 0;
  flex: 1;
}
.plugin-section__name {
  display: block;
  font-size: 13px;
  font-weight: 600;
}
.plugin-section__badge {
  display: inline-block;
  margin-top: 4px;
  font-size: 10px;
  padding: 2px 6px;
  border-radius: 4px;
  background: rgba(138, 180, 255, 0.15);
  color: #b4c8ff;
}
.plugin-section__ver {
  display: block;
  margin-top: 4px;
  font-size: 11px;
  opacity: 0.5;
}
.plugin-section__toggle {
  flex-shrink: 0;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 4px;
  min-width: 36px;
  padding: 4px 8px;
  font-size: 12px;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.15);
  background: rgba(255, 255, 255, 0.06);
  color: var(--color-text);
  cursor: pointer;
}
.plugin-section__toggle-icon {
  flex-shrink: 0;
  opacity: 0.8;
}
.plugin-section__toggle:hover {
  background: rgba(255, 255, 255, 0.1);
}
</style>
