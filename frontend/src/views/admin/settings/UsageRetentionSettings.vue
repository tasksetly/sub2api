<template>
  <section class="card p-6 space-y-4">
    <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.usageRetention.title') }}</h2>
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.usageRetention.scope') }}</p>
    <p v-if="loadError" role="alert" class="text-sm text-red-600">{{ t('admin.settings.usageRetention.loadFailed') }}</p>
    <fieldset :disabled="!loaded || saving" class="space-y-4">
      <label class="flex items-center justify-between gap-4 text-gray-900 dark:text-white">
        {{ t('admin.settings.usageRetention.enabled') }}
        <Toggle v-model="policy.enabled" :aria-label="t('admin.settings.usageRetention.enabled')" />
      </label>
      <label class="block text-sm text-gray-700 dark:text-gray-300">
        {{ t('admin.settings.usageRetention.days') }}
        <input v-model.number="policy.retention_days" type="number" min="1" max="3650" step="1" class="input mt-1 w-32" />
      </label>
      <p class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.usageRetention.warning') }}</p>
      <button type="button" class="btn btn-primary" :disabled="!valid" @click="requestSave">{{ t('common.save') }}</button>
    </fieldset>
    <ConfirmDialog :show="confirmVisible" :title="t('admin.settings.usageRetention.title')" :message="t('admin.settings.usageRetention.confirm', { days: policy.retention_days })" danger @confirm="save" @cancel="confirmVisible = false" />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import { getAdminUsageRetentionSettings, saveUsageRetentionSettings, type UsageRetentionSettings } from '@/api/usageRetention'
import { useAppStore } from '@/stores/app'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const policy = ref<UsageRetentionSettings>({ enabled: false, retention_days: 90 })
const saved = ref<UsageRetentionSettings>({ ...policy.value })
const loaded = ref(false)
const loadError = ref(false)
const saving = ref(false)
const confirmVisible = ref(false)
const valid = computed(() => Number.isInteger(policy.value.retention_days) && policy.value.retention_days >= 1 && policy.value.retention_days <= 3650)

onMounted(async () => {
  try {
    policy.value = await getAdminUsageRetentionSettings()
    saved.value = { ...policy.value }
    loaded.value = true
  } catch {
    loadError.value = true
  }
})

function requestSave() {
  if (policy.value.enabled && (!saved.value.enabled || policy.value.retention_days < saved.value.retention_days)) {
    confirmVisible.value = true
  } else {
    void save()
  }
}

async function save() {
  if (saving.value || !loaded.value || !valid.value) return
  confirmVisible.value = false
  saving.value = true
  try {
    policy.value = await saveUsageRetentionSettings({ ...policy.value })
    saved.value = { ...policy.value }
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.usageRetention.saveFailed')))
  } finally {
    saving.value = false
  }
}
</script>
