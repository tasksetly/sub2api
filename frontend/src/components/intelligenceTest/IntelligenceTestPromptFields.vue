<template>
  <div class="space-y-3">
    <div>
      <label class="input-label" for="svg-test-template">{{ t('admin.accounts.svgTest.promptTemplate') }}</label>
      <Select id="svg-test-template" v-model="templateId" :options="templateOptions" searchable clearable :loading="templatesLoading" :disabled="disabled || templateSaving" :placeholder="t('admin.accounts.svgTest.templatePlaceholder')" @update:model-value="applyPromptTemplate" />
      <p class="mt-1 text-xs text-gray-500">{{ t('admin.accounts.svgTest.templateHint') }}</p>
    </div>
    <TextArea ref="promptInput" id="svg-test-prompt" v-model="prompt" :label="t('admin.accounts.svgTest.prompt')" :placeholder="t('admin.accounts.svgTest.promptPlaceholder')" :hint="t('admin.accounts.svgTest.promptHint')" :rows="5" required :disabled="disabled || templateSaving" />
    <div class="flex flex-col gap-2 sm:flex-row sm:items-end">
      <Input id="svg-test-template-name" v-model="templateName" :label="t('admin.accounts.svgTest.templateName')" :placeholder="t('admin.accounts.svgTest.templateNamePlaceholder')" :disabled="templateSaving" />
      <button type="button" class="btn btn-secondary shrink-0" :disabled="!canSaveTemplate" @click="savePromptTemplate">
        {{ t(templateSaving ? 'common.saving' : 'admin.accounts.svgTest.saveTemplate') }}
      </button>
    </div>
    <p v-if="templateError" class="text-sm text-red-600" role="alert">
      {{ templateError }}
      <button type="button" class="underline" :disabled="templatesLoading" @click="loadPromptTemplates">{{ t('common.refresh') }}</button>
    </p>
    <p v-if="templateSaved" class="text-sm text-green-600" role="status">{{ t('admin.accounts.svgTest.templateSaved') }}</p>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import TextArea from '@/components/common/TextArea.vue'
import { adminAPI } from '@/api/admin'
import type { ScheduledTestPromptTemplate } from '@/types'
import { extractApiErrorMessage } from '@/utils/apiError'

const props = defineProps<{ modelValue: string; disabled?: boolean }>()
const emit = defineEmits<{ 'update:modelValue': [value: string]; saving: [value: boolean] }>()
const { t } = useI18n()
const prompt = computed({ get: () => props.modelValue, set: value => emit('update:modelValue', value) })
const promptInput = ref<InstanceType<typeof TextArea> | null>(null)
const promptTemplates = ref<ScheduledTestPromptTemplate[]>([])
const templateId = ref('')
const templateName = ref('')
const templatesLoading = ref(false)
const templateSaving = ref(false)
const templateSaved = ref(false)
const templateError = ref('')
const controller = new AbortController()
const templateOptions = computed<SelectOption[]>(() => promptTemplates.value.map(item => ({ value: String(item.id), label: item.name })))
const validPrompt = computed(() => !!prompt.value.trim() && Array.from(prompt.value.trim()).length <= 20000)
const canSaveTemplate = computed(() => !templatesLoading.value && !templateSaving.value && !props.disabled && validPrompt.value && !!templateName.value.trim() && Array.from(templateName.value.trim()).length <= 200)
watch(templateSaving, value => emit('saving', value), { flush: 'sync' })
watch(prompt, value => {
  templateSaved.value = false
  if (promptTemplates.value.find(item => String(item.id) === templateId.value)?.prompt_text !== value) templateId.value = ''
})

async function loadPromptTemplates() {
  if (templatesLoading.value || templateSaving.value) return
  templatesLoading.value = true
  templateError.value = ''
  try {
    const items = await adminAPI.scheduledTests.listPromptTemplates()
    if (!controller.signal.aborted) promptTemplates.value = items
  } catch (cause) {
    if (!controller.signal.aborted) templateError.value = extractApiErrorMessage(cause, t('admin.accounts.svgTest.templatesLoadFailed'))
  } finally {
    templatesLoading.value = false
  }
}

function applyPromptTemplate(value: SelectOption['value']) {
  if (props.disabled || templateSaving.value) return
  const item = promptTemplates.value.find(item => String(item.id) === String(value ?? ''))
  if (item) {
    prompt.value = item.prompt_text
    promptInput.value?.focus()
  }
}

async function savePromptTemplate() {
  if (!canSaveTemplate.value) return
  templateSaving.value = true
  templateSaved.value = false
  templateError.value = ''
  try {
    const created = await adminAPI.scheduledTests.createPromptTemplate({
      name: templateName.value.trim(), prompt_text: prompt.value.trim(), expected_answer: ''
    })
    if (controller.signal.aborted) return
    promptTemplates.value = [created, ...promptTemplates.value]
    templateName.value = ''
    if (prompt.value === created.prompt_text) templateId.value = String(created.id)
    templateSaved.value = true
  } catch (cause) {
    if (!controller.signal.aborted) templateError.value = extractApiErrorMessage(cause, t('admin.accounts.svgTest.templateSaveFailed'))
  } finally {
    templateSaving.value = false
  }
}

defineExpose({ focus: () => promptInput.value?.focus() })
onMounted(loadPromptTemplates)
onBeforeUnmount(() => { controller.abort(); emit('saving', false) })
</script>
