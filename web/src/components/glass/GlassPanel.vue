<script setup lang="ts">
/** 毛玻璃基础组件 — 方案 components/glass/ */
defineProps<{
  as?: string
}>()
</script>

<template>
  <component :is="as ?? 'div'" class="glass-panel">
    <slot />
  </component>
</template>
