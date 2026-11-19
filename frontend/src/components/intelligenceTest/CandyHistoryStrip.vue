<template>
  <div
    :class="compact ? 'flex w-full max-w-32 gap-0.5' : 'mt-4 flex gap-1 overflow-x-auto py-1'"
    :role="compact ? 'img' : 'group'"
    :aria-label="compact ? compactLabel : t('intelligenceTest.history')"
  >
    <component
      :is="compact ? 'span' : 'button'"
      v-for="item in orderedItems"
      :key="item.id"
      :type="compact ? undefined : 'button'"
      :class="[
        'flex-1 rounded-sm',
        compact ? 'h-2 min-w-0' : 'h-8 min-w-3 max-w-10 transition-opacity hover:opacity-70 focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-blue-500',
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
const compactLabel = computed(() => `${t('intelligenceTest.history')}: ${orderedItems.value.map(item => t(candyStatusKey(item.status))).join(', ')}`)

function runLabel(item: IntelligenceTestItem) {
  return [formatCandyRunDate(item, locale.value), item.model_name || item.model_id, t(candyStatusKey(item.status))]
    .filter(Boolean).join(' · ')
}
</script>
