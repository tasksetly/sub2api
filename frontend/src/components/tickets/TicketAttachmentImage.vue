<template>
  <div class="min-w-0">
    <button
      v-if="displayUrl"
      type="button"
      class="group relative block aspect-[4/3] w-full overflow-hidden rounded-md bg-gray-100 focus:outline-none focus:ring-2 focus:ring-primary-500 focus:ring-offset-2 dark:bg-dark-900 dark:focus:ring-offset-dark-800"
      :aria-label="t('tickets.previewImage', { name: attachment.name })"
      @click="previewOpen = true"
    >
      <img
        :src="displayUrl"
        :alt="attachment.name"
        class="h-full w-full object-cover transition duration-200 group-hover:scale-[1.02]"
        @error="handleImageError"
      />
      <span class="absolute inset-x-0 bottom-0 flex items-center gap-2 bg-black/65 px-3 py-2 text-left text-xs text-white">
        <span class="min-w-0 flex-1 truncate">{{ attachment.name }}</span>
        <Icon name="eye" size="sm" class="shrink-0" />
      </span>
    </button>

    <div v-else class="flex aspect-[4/3] w-full items-center justify-center rounded-md bg-gray-100 px-4 dark:bg-dark-900">
      <div v-if="loading" class="flex items-center gap-2 text-sm text-gray-500 dark:text-gray-400">
        <Icon name="refresh" size="sm" class="animate-spin" />
        <span>{{ attachment.name }}</span>
      </div>
      <div v-else class="flex flex-col items-center gap-2 text-center">
        <Icon name="exclamationCircle" size="lg" class="text-gray-400" />
        <span class="text-xs text-gray-500 dark:text-gray-400">{{ t('tickets.imageLoadFailed') }}</span>
        <button type="button" class="text-xs font-medium text-primary-600 hover:text-primary-700 dark:text-primary-400" @click="loadImage">
          {{ t('tickets.retryImage') }}
        </button>
      </div>
    </div>
  </div>

  <Teleport to="body">
    <div
      v-if="previewOpen && displayUrl"
      class="fixed inset-0 z-[80] flex items-center justify-center bg-black/90 p-4 sm:p-8"
      role="dialog"
      aria-modal="true"
      :aria-label="attachment.name"
      @click.self="previewOpen = false"
    >
      <div class="absolute left-4 right-4 top-4 flex items-center justify-between gap-4 text-white sm:left-8 sm:right-8 sm:top-6">
        <span class="truncate text-sm font-medium">{{ attachment.name }}</span>
        <div class="flex shrink-0 items-center gap-2">
          <a :href="displayUrl" :download="attachment.name" class="flex h-10 w-10 items-center justify-center rounded-full bg-white/10 hover:bg-white/20" :aria-label="t('common.download')">
            <Icon name="download" size="md" />
          </a>
          <button type="button" class="flex h-10 w-10 items-center justify-center rounded-full bg-white/10 hover:bg-white/20" :aria-label="t('common.close')" @click="previewOpen = false">
            <Icon name="x" size="md" />
          </button>
        </div>
      </div>
      <img :src="displayUrl" :alt="attachment.name" class="max-h-[calc(100vh-7rem)] max-w-full object-contain" />
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { downloadTicketAttachment } from '@/api/ticketAttachments'
import Icon from '@/components/icons/Icon.vue'
import type { TicketAttachment } from '@/types/ticket'

const props = defineProps<{ attachment: TicketAttachment }>()
const { t } = useI18n()
const displayUrl = ref('')
const loading = ref(false)
const previewOpen = ref(false)
let objectUrl = ''
let requestVersion = 0

function releaseObjectURL() {
  if (!objectUrl) return
  URL.revokeObjectURL(objectUrl)
  objectUrl = ''
}

async function loadImage() {
  const version = ++requestVersion
  loading.value = true
  displayUrl.value = ''
  releaseObjectURL()
  try {
    const blob = await downloadTicketAttachment(props.attachment.url)
    if (version !== requestVersion) return
    objectUrl = URL.createObjectURL(blob)
    displayUrl.value = objectUrl
  } catch {
    if (version === requestVersion) displayUrl.value = ''
  } finally {
    if (version === requestVersion) loading.value = false
  }
}

function handleImageError() {
  ++requestVersion
  displayUrl.value = ''
  loading.value = false
  releaseObjectURL()
}

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') previewOpen.value = false
}

watch(() => props.attachment.url, () => void loadImage(), { immediate: true })

onMounted(() => document.addEventListener('keydown', handleKeydown))

onBeforeUnmount(() => {
  ++requestVersion
  document.removeEventListener('keydown', handleKeydown)
  releaseObjectURL()
})
</script>
