<template>
  <span
    class="inline-flex items-center rounded px-2 py-1 text-xs font-medium"
    :class="[badgeClass, { 'gap-1.5': attention }]"
  >
    <span
      v-if="attention"
      data-testid="ticket-badge-attention-dot"
      class="h-1.5 w-1.5 rounded-full bg-red-500"
      aria-hidden="true"
    ></span>
    {{ label }}
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'

const props = withDefaults(defineProps<{
  value: string
  type?: 'status' | 'priority'
  attention?: boolean
}>(), {
  type: 'status',
  attention: false
})
const { t } = useI18n()
const label = computed(() => t(`tickets.${props.type}.${props.value}`))
const badgeClass = computed(() => {
  const classes: Record<string, string> = {
    open: 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    in_progress: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    waiting_user: 'bg-emerald-50 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-300',
    closed: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
    low: 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300',
    normal: 'bg-blue-50 text-blue-700 dark:bg-blue-900/30 dark:text-blue-300',
    high: 'bg-amber-50 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300',
    urgent: 'bg-red-50 text-red-700 dark:bg-red-900/30 dark:text-red-300'
  }
  return classes[props.value] || classes.normal
})
</script>
