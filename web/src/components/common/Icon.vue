<script setup lang="ts">
/** Tabler 图标。用遮罩上色，使 stroke/fill 跟随 currentColor。 */
import { computed } from 'vue'

const props = defineProps<{
  name: string
  size?: number
}>()

const px = computed(() => `${props.size ?? 20}px`)
const mask = computed(() => `url("/icons/${props.name}.svg")`)
</script>

<template>
  <span
    class="icon"
    :style="{
      width: px,
      height: px,
      maskImage: mask,
      WebkitMaskImage: mask,
    }"
    aria-hidden="true"
  />
</template>

<style scoped>
.icon {
  display: inline-block;
  flex-shrink: 0;
  vertical-align: middle;
  background-color: currentColor;
  mask-repeat: no-repeat;
  mask-position: center;
  mask-size: contain;
  mask-mode: alpha;
  -webkit-mask-repeat: no-repeat;
  -webkit-mask-position: center;
  -webkit-mask-size: contain;
}
</style>
