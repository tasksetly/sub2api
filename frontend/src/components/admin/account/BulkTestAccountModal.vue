<template>
  <BaseDialog
    :show="show"
    :title="t('admin.accounts.bulkActions.selectTestModelTitle')"
    width="narrow"
    @close="handleClose"
  >
    <div class="space-y-4">
      <div class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('admin.accounts.bulkActions.selectedForTest', { count: accountIds.length }) }}
      </div>

      <div class="space-y-1.5">
        <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.accounts.testModel') }}
        </label>
        <Select
          v-model="selectedModelId"
          :options="modelOptions"
          :disabled="loadingModels || !!loadError"
          :placeholder="loadingModels ? `${t('common.loading')}` : t('admin.accounts.selectTestModel')"
        />
      </div>

      <p v-if="loadingModels" class="text-sm text-gray-500 dark:text-gray-400">
        {{ t('admin.accounts.bulkActions.loadingTestModels') }}
      </p>
      <p v-else-if="loadError" class="text-sm text-red-600 dark:text-red-400">
        {{ loadError }}
      </p>
      <p v-else-if="modelOptions.length === 0" class="text-sm text-amber-700 dark:text-amber-300">
        {{ t('admin.accounts.bulkActions.noCommonTestModels') }}
      </p>
    </div>

    <template #footer>
      <div class="flex justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="testing" @click="handleClose">
          {{ t('common.cancel') }}
        </button>
        <button
          type="button"
          class="btn btn-primary"
          :disabled="loadingModels || !!loadError || !selectedModelId || testing"
          @click="confirm"
        >
          <Icon v-if="testing" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="play" size="sm" />
          {{ testing ? t('admin.accounts.bulkActions.testing') : t('admin.accounts.bulkActions.startTest') }}
        </button>
      </div>
    </template>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI } from '@/api/admin'
import type { ClaudeModel } from '@/types'

const { t } = useI18n()

const props = defineProps<{
  show: boolean
  accountIds: number[]
  testing?: boolean
}>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'confirm', modelId: string): void
}>()

const modelOptions = ref<SelectOption[]>([])
const selectedModelId = ref('')
const loadingModels = ref(false)
const loadError = ref('')
let requestVersion = 0

watch(
  () => props.show,
  (show) => {
    if (!show) return
    void loadCommonModels()
  }
)

async function loadCommonModels() {
  const version = ++requestVersion
  const accountIds = [...new Set(props.accountIds.filter(id => Number.isSafeInteger(id) && id > 0))]
  selectedModelId.value = ''
  modelOptions.value = []
  loadError.value = ''

  if (accountIds.length === 0) return

  loadingModels.value = true
  try {
    const modelsByAccount = await loadModelsWithConcurrency(accountIds)
    if (version !== requestVersion) return

    const firstModels = modelsByAccount[0] ?? []
    const commonIDs = new Set(firstModels.map(model => model.id))
    for (const models of modelsByAccount.slice(1)) {
      const modelIDs = new Set(models.map(model => model.id))
      for (const modelID of commonIDs) {
        if (!modelIDs.has(modelID)) commonIDs.delete(modelID)
      }
    }

    const labels = new Map(firstModels.map(model => [model.id, model.display_name || model.id]))
    modelOptions.value = [...commonIDs]
      .sort((a, b) => a.localeCompare(b))
      .map(id => ({ value: id, label: labels.get(id) || id }))
    selectedModelId.value = String(modelOptions.value[0]?.value || '')
  } catch (error) {
    if (version !== requestVersion) return
    console.error('Failed to load common account test models:', error)
    loadError.value = t('admin.accounts.bulkActions.loadTestModelsFailed')
  } finally {
    if (version === requestVersion) loadingModels.value = false
  }
}

async function loadModelsWithConcurrency(accountIds: number[]): Promise<ClaudeModel[][]> {
  const results = new Array<ClaudeModel[]>(accountIds.length)
  let nextIndex = 0
  const workers = Array.from({ length: Math.min(6, accountIds.length) }, async () => {
    while (nextIndex < accountIds.length) {
      const index = nextIndex++
      results[index] = await adminAPI.accounts.getAvailableModels(accountIds[index])
    }
  })
  await Promise.all(workers)
  return results
}

function confirm() {
  if (!selectedModelId.value || loadingModels.value || loadError.value || props.testing) return
  emit('confirm', selectedModelId.value)
}

function handleClose() {
  if (props.testing) return
  requestVersion += 1
  emit('close')
}
</script>
