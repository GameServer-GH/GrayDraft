<script setup lang="ts">
import { computed } from 'vue'
import Icon from '@/components/common/Icon.vue'
import type { ExtensionFieldRow } from '@/utils/extensionsEditor'

withDefaults(
  defineProps<{
    label: string
    hint?: string
    keyPlaceholder: string
    valuePlaceholder: string
    addLabel: string
    removeAria: string
    disabled?: boolean
  }>(),
  { hint: '', disabled: false },
)

const rows = defineModel<ExtensionFieldRow[]>('rows', { required: true })

const canRemove = computed(() => rows.value.length > 0)

function addRow() {
  rows.value = [...rows.value, { key: '', value: '' }]
}

function removeRow(index: number) {
  rows.value = rows.value.filter((_, i) => i !== index)
}
</script>

<template>
  <div class="ext-fields">
    <div class="ext-fields__head">
      <span class="ext-fields__label">{{ label }}</span>
      <button
        type="button"
        class="ext-fields__add"
        :disabled="disabled"
        @click="addRow"
      >
        {{ addLabel }}
      </button>
    </div>
    <p v-if="hint" class="ext-fields__hint">{{ hint }}</p>
    <div v-if="rows.length === 0" class="ext-fields__empty" />
    <div
      v-for="(row, index) in rows"
      :key="index"
      class="ext-fields__row"
    >
      <input
        v-model="row.key"
        type="text"
        class="ext-fields__input ext-fields__input--key"
        :placeholder="keyPlaceholder"
        autocomplete="off"
        :disabled="disabled"
      />
      <input
        v-model="row.value"
        type="text"
        class="ext-fields__input"
        :placeholder="valuePlaceholder"
        autocomplete="off"
        :disabled="disabled"
      />
      <button
        type="button"
        class="ext-fields__remove"
        :aria-label="removeAria"
        :disabled="disabled || !canRemove"
        @click="removeRow(index)"
      >
        <Icon name="x" :size="14" />
      </button>
    </div>
  </div>
</template>

<style scoped>
.ext-fields {
  margin-bottom: var(--space-3);
}

.ext-fields__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  margin-bottom: var(--space-2);
}

.ext-fields__label {
  font-size: 13px;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.ext-fields__add {
  border: none;
  border-radius: 8px;
  padding: 4px 10px;
  font: inherit;
  font-size: 12px;
  font-weight: 600;
  color: var(--color-accent);
  background: rgba(0, 122, 255, 0.1);
  cursor: pointer;
}

.ext-fields__add:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.ext-fields__hint {
  margin: 0 0 var(--space-2);
  font-size: 12px;
  line-height: 1.45;
  color: var(--color-text-secondary);
}

.ext-fields__empty {
  min-height: 0;
}

.ext-fields__row {
  display: grid;
  grid-template-columns: minmax(88px, 0.9fr) 1.4fr auto;
  gap: 8px;
  margin-bottom: 8px;
}

.ext-fields__input {
  box-sizing: border-box;
  width: 100%;
  padding: 8px 10px;
  border-radius: 10px;
  border: none;
  font: inherit;
  font-size: 13px;
  background: rgba(255, 255, 255, 0.55);
  box-shadow: inset 0 0 0 1px rgba(0, 0, 0, 0.08);
}

.ext-fields__input:disabled {
  opacity: 0.55;
}

.ext-fields__remove {
  width: 32px;
  height: 36px;
  border: none;
  border-radius: 8px;
  font-size: 18px;
  line-height: 1;
  color: var(--color-text-secondary);
  background: rgba(255, 255, 255, 0.45);
  cursor: pointer;
}

.ext-fields__remove:disabled {
  opacity: 0.4;
  cursor: not-allowed;
}
</style>
