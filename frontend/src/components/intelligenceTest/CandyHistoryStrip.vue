<template>
  <div
    :class="compact ? 'flex h-3 min-w-12 flex-[1_1_10rem] overflow-hidden rounded-sm' : 'mt-4 flex gap-1 overflow-x-auto py-1'"
    :style="compact ? compactStyle : undefined"
    :role="compact ? 'img' : 'group'"
    :aria-label="compact ? compactLabel : t('intelligenceTest.history')"
  >
    <component
      :is="compact ? 'span' : 'button'"
      v-for="item in orderedItems"
      :key="item.id"
      :type="compact ? undefined : 'button'"
      :class="[
        'flex-1',
        compact ? 'min-w-0' : 'h-8 min-w-3 max-w-10 rounded-sm transition-opacity hover:opacity-70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500',
        candyStatusColor(item.status)
      ]"
      :title="runLabel(item)"
      :aria-label="compact ? undefined : runLabel(item)"
      :data-testid="compact ? 'candy-run-compact' : 'candy-run'"
      @click="!compact && emit('select', item)"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import type { IntelligenceTestItem } from '@/api/intelligenceTest'
import { candyStatusColor, candyStatusKey, formatCandyRunDate } from './candyHistory'

const props = defineProps<{ items: IntelligenceTestItem[]; compact?: boolean }>()
const emit = defineEmits<{ select: [item: IntelligenceTestItem] }>()
const { t, locale } = useI18n()
const orderedItems = computed(() => [...props.items].reverse())
const compactStyle = computed(() => ({
  columnGap: `clamp(0px, calc(100% / ${Math.max(1, props.items.length)} - 2px), 2px)`,
}))
const compactLabel = computed(() => `${t('intelligenceTest.history')}: ${orderedItems.value.map(item => t(candyStatusKey(item.status))).join(', ')}`)

function runLabel(item: IntelligenceTestItem) {
  return [formatCandyRunDate(item, locale.value), item.model_name || item.model_id, t(candyStatusKey(item.status))]
    .filter(Boolean).join(' · ')
}
</script>
