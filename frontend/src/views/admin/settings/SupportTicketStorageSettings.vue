<template>
  <section class="card" aria-labelledby="ticket-storage-title">
    <div class="border-b border-gray-100 px-4 py-4 dark:border-dark-700 sm:px-6">
      <div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div class="min-w-0">
          <div class="flex flex-wrap items-center gap-2">
            <h2 id="ticket-storage-title" class="text-lg font-semibold text-gray-900 dark:text-white">
              {{ t('admin.settings.features.ticketStorage.title') }}
            </h2>
            <span
              v-if="!loading"
              :class="[
                'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                form.enabled
                  ? 'bg-green-50 text-green-700 dark:bg-green-900/20 dark:text-green-300'
                  : 'bg-gray-100 text-gray-600 dark:bg-dark-700 dark:text-gray-300'
              ]"
            >
              {{ form.enabled
                ? t('admin.settings.features.ticketStorage.statusEnabled')
                : t('admin.settings.features.ticketStorage.statusDisabled') }}
            </span>
          </div>
          <p class="mt-1 text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.ticketStorage.description') }}
          </p>
        </div>
        <Icon name="cloud" size="md" class="hidden flex-shrink-0 text-gray-400 sm:block" />
      </div>
    </div>

    <div v-if="loading" class="flex min-h-32 items-center justify-center p-6">
      <Icon name="refresh" size="md" class="animate-spin text-gray-400" />
    </div>

    <div v-else-if="loadError" class="p-4 sm:p-6">
      <div class="rounded-md border border-red-200 bg-red-50 p-4 dark:border-red-900/40 dark:bg-red-950/20">
        <p class="text-sm text-red-700 dark:text-red-300">{{ loadError }}</p>
        <button type="button" class="btn btn-secondary btn-sm mt-3" @click="loadConfig">
          <Icon name="refresh" size="xs" />
          {{ t('common.retry') }}
        </button>
      </div>
    </div>

    <div v-else class="space-y-6 p-4 sm:p-6">
      <div class="flex items-start justify-between gap-4">
        <div>
          <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
            {{ t('admin.settings.features.ticketStorage.enabled') }}
          </label>
          <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.settings.features.ticketStorage.enabledHint') }}
          </p>
        </div>
        <Toggle v-model="form.enabled" />
      </div>

      <div v-if="form.enabled" class="space-y-5 border-t border-gray-100 pt-5 dark:border-dark-700">
        <div class="grid gap-4 lg:grid-cols-2">
          <div class="lg:col-span-2">
            <label for="ticket-storage-endpoint" class="input-label">
              {{ t('admin.settings.features.ticketStorage.endpoint') }}
            </label>
            <input
              id="ticket-storage-endpoint"
              v-model.trim="form.endpoint"
              type="url"
              inputmode="url"
              class="input"
              placeholder="https://<account_id>.r2.cloudflarestorage.com"
              autocomplete="off"
            />
            <p class="input-hint">{{ t('admin.settings.features.ticketStorage.endpointHint') }}</p>
          </div>

          <div>
            <label for="ticket-storage-region" class="input-label">
              {{ t('admin.settings.features.ticketStorage.region') }}
            </label>
            <input
              id="ticket-storage-region"
              v-model.trim="form.region"
              type="text"
              class="input"
              placeholder="auto"
              autocomplete="off"
            />
          </div>

          <div>
            <label for="ticket-storage-bucket" class="input-label">
              {{ t('admin.settings.features.ticketStorage.bucket') }}
            </label>
            <input
              id="ticket-storage-bucket"
              v-model.trim="form.bucket"
              type="text"
              class="input"
              autocomplete="off"
            />
          </div>

          <div>
            <label for="ticket-storage-access-key" class="input-label">
              {{ t('admin.settings.features.ticketStorage.accessKeyId') }}
            </label>
            <input
              id="ticket-storage-access-key"
              v-model.trim="form.access_key_id"
              type="text"
              class="input"
              autocomplete="off"
            />
          </div>

          <div>
            <label for="ticket-storage-secret-key" class="input-label">
              {{ t('admin.settings.features.ticketStorage.secretAccessKey') }}
            </label>
            <input
              id="ticket-storage-secret-key"
              v-model="form.secret_access_key"
              type="password"
              class="input"
              :placeholder="form.secret_configured
                ? t('admin.settings.features.ticketStorage.secretConfigured')
                : ''"
              autocomplete="new-password"
            />
            <p v-if="form.secret_configured" class="input-hint">
              {{ t('admin.settings.features.ticketStorage.secretHint') }}
            </p>
          </div>

          <div class="lg:col-span-2">
            <label for="ticket-storage-prefix" class="input-label">
              {{ t('admin.settings.features.ticketStorage.prefix') }}
            </label>
            <input
              id="ticket-storage-prefix"
              v-model.trim="form.prefix"
              type="text"
              class="input"
              placeholder="support-tickets"
              autocomplete="off"
            />
          </div>
        </div>

        <div class="rounded-md border border-gray-200 bg-gray-50 p-4 dark:border-dark-700 dark:bg-dark-800/50">
          <div class="flex items-start justify-between gap-4">
            <div>
              <label class="text-sm font-medium text-gray-700 dark:text-gray-300">
                {{ t('admin.settings.features.ticketStorage.forcePathStyle') }}
              </label>
              <p class="mt-0.5 text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.settings.features.ticketStorage.forcePathStyleHint') }}
              </p>
            </div>
            <Toggle v-model="form.force_path_style" />
          </div>
        </div>

        <div class="grid gap-4 sm:grid-cols-3">
          <div>
            <label for="ticket-storage-max-size" class="input-label">
              {{ t('admin.settings.features.ticketStorage.maxFileSize') }}
            </label>
            <input
              id="ticket-storage-max-size"
              v-model.number="form.max_file_size_mb"
              type="number"
              min="1"
              max="100"
              class="input"
            />
          </div>
          <div>
            <label for="ticket-storage-max-count" class="input-label">
              {{ t('admin.settings.features.ticketStorage.maxAttachments') }}
            </label>
            <input
              id="ticket-storage-max-count"
              v-model.number="form.max_attachments_per_message"
              type="number"
              min="1"
              max="10"
              class="input"
            />
          </div>
          <div>
            <label for="ticket-storage-url-expiry" class="input-label">
              {{ t('admin.settings.features.ticketStorage.urlExpiry') }}
            </label>
            <input
              id="ticket-storage-url-expiry"
              v-model.number="form.url_expiry_minutes"
              type="number"
              min="1"
              max="1440"
              class="input"
            />
          </div>
        </div>
      </div>

      <div class="flex flex-col-reverse gap-2 border-t border-gray-100 pt-5 dark:border-dark-700 sm:flex-row sm:justify-end">
        <button
          type="button"
          data-testid="ticket-storage-test"
          class="btn btn-secondary"
          :disabled="testing || saving || !form.enabled"
          @click="testConnection"
        >
          <Icon name="refresh" size="xs" :class="testing ? 'animate-spin' : ''" />
          {{ testing
            ? t('admin.settings.features.ticketStorage.testing')
            : t('admin.settings.features.ticketStorage.testConnection') }}
        </button>
        <button
          type="button"
          data-testid="ticket-storage-save"
          class="btn btn-primary"
          :disabled="saving || testing"
          @click="saveConfig"
        >
          <Icon v-if="saving" name="refresh" size="xs" class="animate-spin" />
          {{ saving
            ? t('admin.settings.features.ticketStorage.saving')
            : t('admin.settings.features.ticketStorage.save') }}
        </button>
      </div>
    </div>
  </section>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Icon from '@/components/icons/Icon.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import type { SupportTicketAttachmentStorageConfig } from '@/api/admin/tickets'
import { useAppStore } from '@/stores'
import { extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const loading = ref(true)
const saving = ref(false)
const testing = ref(false)
const loadError = ref('')

const defaultConfig = (): SupportTicketAttachmentStorageConfig => ({
  enabled: false,
  endpoint: '',
  region: 'auto',
  bucket: '',
  access_key_id: '',
  secret_access_key: '',
  secret_configured: false,
  prefix: 'support-tickets',
  force_path_style: false,
  max_file_size_mb: 10,
  max_attachments_per_message: 4,
  url_expiry_minutes: 15
})

const form = reactive<SupportTicketAttachmentStorageConfig>(defaultConfig())

function applyConfig(config: SupportTicketAttachmentStorageConfig): void {
  Object.assign(form, defaultConfig(), config, { secret_access_key: '' })
}

async function loadConfig(): Promise<void> {
  loading.value = true
  loadError.value = ''
  try {
    applyConfig(await adminAPI.tickets.attachmentStorage())
  } catch (error: unknown) {
    loadError.value = extractApiErrorMessage(
      error,
      t('admin.settings.features.ticketStorage.loadFailed')
    )
  } finally {
    loading.value = false
  }
}

async function saveConfig(): Promise<void> {
  saving.value = true
  try {
    applyConfig(await adminAPI.tickets.updateAttachmentStorage({ ...form }))
    appStore.showSuccess(t('admin.settings.features.ticketStorage.saved'))
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.settings.features.ticketStorage.saveFailed'))
    )
  } finally {
    saving.value = false
  }
}

async function testConnection(): Promise<void> {
  testing.value = true
  try {
    const result = await adminAPI.tickets.testAttachmentStorage({ ...form })
    if (result.ok) {
      appStore.showSuccess(t('admin.settings.features.ticketStorage.testSuccess'))
    } else {
      appStore.showError(result.message || t('admin.settings.features.ticketStorage.testFailed'))
    }
  } catch (error: unknown) {
    appStore.showError(
      extractApiErrorMessage(error, t('admin.settings.features.ticketStorage.testFailed'))
    )
  } finally {
    testing.value = false
  }
}

onMounted(loadConfig)
</script>
