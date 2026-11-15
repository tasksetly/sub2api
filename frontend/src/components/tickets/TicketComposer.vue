<template>
  <form class="space-y-3" @submit.prevent="submit">
    <textarea
      v-model="content"
      rows="4"
      class="input min-h-28 resize-y"
      :placeholder="placeholder"
      :disabled="disabled || submitting"
      :aria-keyshortcuts="hideSubmit ? undefined : 'Control+Enter Meta+Enter'"
      @keydown="onKeydown"
    />
    <p v-if="!hideSubmit" class="text-xs text-gray-500 dark:text-gray-400">{{ t('tickets.sendShortcut') }}</p>

    <div v-if="files.length" class="grid grid-cols-3 gap-2 sm:grid-cols-5">
      <div v-for="(file, index) in files" :key="`${file.name}-${file.lastModified}`" class="relative aspect-square overflow-hidden rounded-md border border-gray-200 dark:border-dark-600">
        <img :src="previews[index]" :alt="file.name" class="h-full w-full object-cover" />
        <button
          type="button"
          class="absolute right-1 top-1 flex h-7 w-7 items-center justify-center rounded-full bg-black/65 text-white hover:bg-black/80"
          :aria-label="t('tickets.removeImage')"
          :disabled="disabled || submitting"
          @click="removeFile(index)"
        >
          <Icon name="x" size="sm" />
        </button>
      </div>
    </div>

    <div class="flex flex-wrap items-center justify-between gap-3">
      <label class="btn btn-secondary cursor-pointer" :class="(disabled || submitting) && 'pointer-events-none opacity-50'">
        <Icon name="upload" size="sm" />
        {{ t('tickets.addImages') }}
        <input class="sr-only" type="file" accept="image/png,image/jpeg,image/gif,image/webp" multiple :disabled="disabled || submitting" @change="selectFiles" />
      </label>
      <button v-if="!hideSubmit" type="submit" class="btn btn-primary" :disabled="!canSubmit">
        <Icon v-if="submitting" name="refresh" size="sm" class="animate-spin" />
        <Icon v-else name="arrowRight" size="sm" />
        {{ submitLabel }}
      </button>
    </div>
  </form>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'

const props = withDefaults(defineProps<{
  submitting?: boolean
  disabled?: boolean
  placeholder?: string
  submitLabel?: string
  hideSubmit?: boolean
  maxFiles?: number
}>(), {
  submitting: false,
  disabled: false,
  placeholder: '',
  submitLabel: '',
  hideSubmit: false,
  maxFiles: 5
})

const emit = defineEmits<{
  submit: [payload: { content: string; images: File[] }]
  change: [payload: { content: string; images: File[] }]
}>()
const { t } = useI18n()
const content = ref('')
const files = ref<File[]>([])
const previews = ref<string[]>([])
const canSubmit = computed(() => !props.hideSubmit && !props.disabled && !props.submitting && (!!content.value.trim() || files.value.length > 0))

watch([content, files], () => emit('change', { content: content.value.trim(), images: [...files.value] }), { deep: true })

function selectFiles(event: Event) {
  const input = event.target as HTMLInputElement
  const selected = Array.from(input.files || []).filter((file) => file.type.startsWith('image/'))
  const available = Math.max(0, props.maxFiles - files.value.length)
  const accepted = selected.slice(0, available)
  files.value.push(...accepted)
  previews.value.push(...accepted.map((file) => URL.createObjectURL(file)))
  input.value = ''
}

function removeFile(index: number) {
  URL.revokeObjectURL(previews.value[index])
  files.value.splice(index, 1)
  previews.value.splice(index, 1)
}

function onKeydown(event: KeyboardEvent) {
  if (
    props.hideSubmit || event.key !== 'Enter' || (!event.ctrlKey && !event.metaKey) ||
    event.shiftKey || event.altKey || event.isComposing || event.keyCode === 229
  ) return
  event.preventDefault()
  if (!event.repeat) submit()
}

function submit() {
  if (!canSubmit.value) return
  emit('submit', { content: content.value.trim(), images: [...files.value] })
}

onBeforeUnmount(() => previews.value.forEach((url) => URL.revokeObjectURL(url)))
</script>
