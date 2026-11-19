<template>
  <div ref="root" @pointerleave="hovered = false" @keydown.esc.stop="close(true)">
    <button
      ref="trigger"
      type="button"
      class="grid h-[30px] w-[30px] place-items-center rounded-full border border-gray-200 bg-white text-gray-600 hover:bg-gray-100 dark:border-dark-600 dark:bg-dark-800 dark:text-gray-300 dark:hover:bg-dark-700"
      :aria-label="`${groupName} · ${t('intelligenceTest.svgPrompt')}`"
      :aria-expanded="open"
      :aria-controls="panelId"
      @pointerenter="hovered = $event.pointerType === 'mouse'"
      @click="pinned ? close() : pinned = true"
    >
      <Icon name="lightbulb" size="sm" />
    </button>
    <div
      v-show="open"
      :id="panelId"
      class="absolute right-0 top-full z-10 max-h-64 w-full max-w-md space-y-3 overflow-auto rounded-lg border border-gray-200 bg-white p-3 text-sm text-gray-800 shadow-lg dark:border-dark-600 dark:bg-dark-800 dark:text-gray-100"
      tabindex="0"
    >
      <h3 class="text-sm font-semibold">{{ t('intelligenceTest.svgPrompt') }}</h3>
      <p v-if="!prompts.length">{{ t('intelligenceTest.svgPromptUnavailable') }}</p>
      <div v-for="prompt in prompts" :key="prompt.text" class="border-t border-gray-200 pt-2 dark:border-dark-600">
        <p v-if="prompt.current" class="mb-1 text-xs font-medium text-gray-500 dark:text-gray-400">{{ t('intelligenceTest.svgPromptCurrent') }}</p>
        <p class="whitespace-pre-wrap break-words">{{ prompt.text }}</p>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { onClickOutside } from '@vueuse/core'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import type { IntelligenceTestItem } from '@/api/intelligenceTest'

const props = defineProps<{ groupName: string; items: IntelligenceTestItem[] }>()
const { t } = useI18n()
const panelId = useId()
const root = ref<HTMLElement | null>(null)
const trigger = ref<HTMLButtonElement | null>(null)
const hovered = ref(false)
const pinned = ref(false)
const open = computed(() => hovered.value || pinned.value)
const prompts = computed(() => {
  const unique = new Map<string, { text: string; current: boolean }>()
  for (const item of props.items) {
    const text = item.prompt_text?.trim()
    if (text && (!unique.has(text) || !item.prompt_is_current)) {
      unique.set(text, { text, current: item.prompt_is_current === true })
    }
  }
  return [...unique.values()]
})

function close(restoreFocus = false) {
  if (restoreFocus && open.value) trigger.value?.focus()
  hovered.value = false
  pinned.value = false
}

onClickOutside(root, () => close())
</script>
