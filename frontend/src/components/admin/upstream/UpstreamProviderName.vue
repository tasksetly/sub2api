<template>
  <div class="flex max-w-[220px] items-center gap-1 font-medium">
    <span class="min-w-0 truncate" :title="name">{{ name }}</span>
    <HelpTooltip v-if="notes?.trim() || syncError" trigger="click" class="!ml-0 shrink-0" width-class="w-72 max-w-[calc(100vw-2rem)]">
      <template #trigger>
        <button
          type="button"
          class="rounded text-gray-400 transition hover:text-primary-600 focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary-500 dark:text-gray-500 dark:hover:text-primary-400"
          :class="{ 'text-red-600 dark:text-red-400': syncError }"
          :aria-label="t(syncError ? 'admin.upstreamProviders.syncFailed' : 'admin.upstreamProviders.formNotes')"
        >
          <Icon :name="syncError ? 'exclamationCircle' : 'infoCircle'" size="xs" />
        </button>
      </template>
      <div class="max-h-60 overflow-y-auto pr-4 font-normal">
        <div v-if="notes?.trim()">
          <p class="mb-1 font-medium">{{ t('admin.upstreamProviders.formNotes') }}</p>
          <p class="whitespace-pre-wrap break-words">{{ notes }}</p>
        </div>
        <div v-if="syncError" :class="{ 'mt-3': notes?.trim() }">
          <p class="mb-1 font-medium text-red-300">{{ t('admin.upstreamProviders.syncFailed') }}</p>
          <p class="whitespace-pre-wrap break-words">{{ syncError }}</p>
        </div>
      </div>
    </HelpTooltip>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Icon from '@/components/icons/Icon.vue'

defineProps<{
  name: string
  notes?: string | null
  syncError?: string
}>()

const { t } = useI18n()
</script>
