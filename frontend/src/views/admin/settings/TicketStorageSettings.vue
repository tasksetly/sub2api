<template>
  <div class="space-y-6">
    <div v-if="loading" class="flex justify-center py-16">
      <Icon name="refresh" class="animate-spin text-primary-600" />
    </div>
    <template v-else>
      <section class="card">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <div class="flex items-center justify-between gap-4">
            <div>
              <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.ticketStorage.title') }}</h2>
              <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.ticketStorage.description') }}</p>
            </div>
            <Toggle v-model="form.enabled" />
          </div>
        </div>

        <div class="grid gap-5 p-6 md:grid-cols-2">
          <div class="md:col-span-2">
            <label class="input-label">{{ t('admin.settings.ticketStorage.endpoint') }}</label>
            <input v-model.trim="form.endpoint" class="input" placeholder="https://<account-id>.r2.cloudflarestorage.com" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.region') }}</label>
            <input v-model.trim="form.region" class="input" placeholder="auto" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.bucket') }}</label>
            <input v-model.trim="form.bucket" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.accessKey') }}</label>
            <input v-model.trim="form.access_key_id" class="input" autocomplete="off" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.secretKey') }}</label>
            <input
              v-model="form.secret_access_key"
              type="password"
              class="input"
              autocomplete="new-password"
              :placeholder="form.has_secret ? t('admin.settings.ticketStorage.secretConfigured') : ''"
            />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.prefix') }}</label>
            <input v-model.trim="form.prefix" class="input" placeholder="tickets/" />
          </div>
          <div class="flex items-end pb-2">
            <label class="flex items-center gap-3 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="form.force_path_style" />
              {{ t('admin.settings.ticketStorage.pathStyle') }}
            </label>
          </div>
        </div>
      </section>

      <section class="card">
        <div class="border-b border-gray-100 px-6 py-4 dark:border-dark-700">
          <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.ticketStorage.limits') }}</h2>
        </div>
        <div class="grid gap-5 p-6 md:grid-cols-2">
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.maxSize') }}</label>
            <input v-model.number="form.max_file_size_mb" type="number" min="1" max="50" class="input" />
          </div>
          <div>
            <label class="input-label">{{ t('admin.settings.ticketStorage.maxFiles') }}</label>
            <input v-model.number="form.max_files_per_message" type="number" min="1" max="10" class="input" />
          </div>
        </div>
      </section>

      <div class="flex flex-wrap justify-end gap-3">
        <button type="button" class="btn btn-secondary" :disabled="testing || saving" @click="testConnection">
          <Icon v-if="testing" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="beaker" size="sm" />
          {{ t('admin.settings.ticketStorage.test') }}
        </button>
        <button type="button" class="btn btn-primary" :disabled="saving || testing" @click="save">
          <Icon v-if="saving" name="refresh" size="sm" class="animate-spin" />
          <Icon v-else name="check" size="sm" />
          {{ t('common.save') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'
import type { TicketStorageConfig } from '@/types/ticket'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const form = ref<TicketStorageConfig>({
  enabled: false,
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  prefix: 'tickets/',
  force_path_style: false,
  max_file_size_mb: 10,
  max_files_per_message: 5
})

async function load() {
  loading.value = true
  try {
    form.value = { ...form.value, ...(await adminAPI.tickets.getStorageConfig()), secret_access_key: '' }
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.ticketStorage.loadFailed')))
  } finally {
    loading.value = false
  }
}

async function save() {
  saving.value = true
  try {
    form.value = { ...form.value, ...(await adminAPI.tickets.updateStorageConfig(form.value)), secret_access_key: '' }
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.ticketStorage.saveFailed')))
  } finally {
    saving.value = false
  }
}

async function testConnection() {
  testing.value = true
  try {
    const result = await adminAPI.tickets.testStorage(form.value)
    result.ok ? appStore.showSuccess(t('admin.settings.ticketStorage.testSuccess')) : appStore.showError(result.message)
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.ticketStorage.testFailed')))
  } finally {
    testing.value = false
  }
}

onMounted(load)
</script>
