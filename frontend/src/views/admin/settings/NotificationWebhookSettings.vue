<template>
  <div class="space-y-6">
    <section class="card space-y-5 p-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.webhook.title') }}</h2>
        <button type="button" class="btn btn-ghost" :disabled="busy" @click="loadSettings">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
          {{ t('common.refresh') }}
        </button>
      </div>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.description') }}</p>
      <p v-if="loadError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ loadError }}</p>
      <p v-if="conflict" role="alert" class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.webhook.conflict') }}</p>
      <p v-if="loaded && !saved?.encryption_key_configured" role="alert" class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.webhook.encryptionKeyRequired') }}</p>
      <fieldset :disabled="busy || !loaded || !saved?.encryption_key_configured" class="space-y-5">
        <label class="flex items-center justify-between gap-4 text-sm text-gray-900 dark:text-white">
          {{ t('admin.settings.webhook.enabled') }}
          <Toggle v-model="enabled" :aria-label="t('admin.settings.webhook.enabled')" />
        </label>
        <div class="rounded-lg bg-gray-50 p-4 text-sm dark:bg-dark-700">
          <p class="font-medium text-gray-900 dark:text-white">{{ t('admin.settings.webhook.event') }}</p>
          <code class="mt-1 block break-all text-gray-600 dark:text-gray-300">payment.recharge.completed</code>
          <p class="mt-2 text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.eventHint') }}</p>
        </div>
        <div>
          <label for="notification-webhook-url" class="input-label">{{ t('admin.settings.webhook.url') }}</label>
          <input id="notification-webhook-url" v-model="url" type="url" maxlength="4096" class="input font-mono" placeholder="https://example.com/webhook" autocomplete="off" @keydown.enter.prevent="save" />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.urlHint') }}</p>
        </div>
        <div>
          <label for="notification-webhook-secret" class="input-label">{{ t('admin.settings.webhook.secret') }}</label>
          <input id="notification-webhook-secret" v-model="secret" type="password" maxlength="512" class="input" autocomplete="new-password" :disabled="clearSecret" :placeholder="saved?.has_secret ? t('admin.settings.webhook.secretConfigured') : t('admin.settings.webhook.secretOptional')" @keydown.enter.prevent="save" />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.signatureHint') }}</p>
          <label v-if="saved?.has_secret" class="mt-2 flex items-center gap-2 text-sm text-gray-600 dark:text-gray-300">
            <input v-model="clearSecret" type="checkbox" @change="secret = ''" />
            {{ t('admin.settings.webhook.clearSecret') }}
          </label>
        </div>
        <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.deliveryHint') }}</p>
        <p class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.webhook.changeHint') }}</p>
        <div class="flex flex-wrap gap-3">
          <button type="button" class="btn btn-primary" :disabled="!canSave" @click="save">
            {{ saving ? t('common.saving') : t('common.save') }}
          </button>
          <button type="button" class="btn btn-secondary" :disabled="!canTest" @click="sendTest">
            <Icon v-if="testing" name="refresh" size="sm" class="animate-spin" />
            {{ t('admin.settings.webhook.test') }}
          </button>
        </div>
        <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.testHint') }}</p>
      </fieldset>
    </section>

    <section class="card space-y-4 p-6">
      <div class="flex flex-wrap items-center justify-between gap-3">
        <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.webhook.deliveries') }}</h2>
        <button type="button" class="btn btn-ghost" :disabled="historyLoading" @click="loadHistory">
          <Icon name="refresh" size="sm" :class="{ 'animate-spin': historyLoading }" />
          {{ t('common.refresh') }}
        </button>
      </div>
      <p class="text-xs text-gray-500 dark:text-gray-400">{{ t('admin.settings.webhook.historyHint') }}</p>
      <p v-if="historyError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ historyError }}</p>
      <div class="overflow-x-auto">
        <table class="w-full text-left text-sm">
          <thead class="text-gray-500 dark:text-gray-400">
            <tr>
              <th class="whitespace-nowrap py-2 pr-4">{{ t('admin.settings.webhook.order') }}</th>
              <th class="whitespace-nowrap py-2 pr-4">{{ t('common.status') }}</th>
              <th class="whitespace-nowrap py-2 pr-4">{{ t('admin.settings.webhook.attempts') }}</th>
              <th class="whitespace-nowrap py-2 pr-4">{{ t('admin.settings.webhook.time') }}</th>
              <th class="py-2">{{ t('admin.settings.webhook.result') }}</th>
            </tr>
          </thead>
          <tbody class="divide-y divide-gray-100 text-gray-700 dark:divide-dark-700 dark:text-gray-300">
            <tr v-for="item in deliveries" :key="item.id">
              <td class="py-3 pr-4 font-mono">#{{ item.order_id }}</td>
              <td class="whitespace-nowrap py-3 pr-4">{{ t(`admin.settings.webhook.status.${item.status}`) }}</td>
              <td class="py-3 pr-4">{{ item.attempts }} / 5</td>
              <td class="whitespace-nowrap py-3 pr-4">{{ formatDate(item.created_at) }}</td>
              <td class="py-3">{{ item.last_error || (item.last_status_code ? `HTTP ${item.last_status_code}` : '—') }}</td>
            </tr>
          </tbody>
        </table>
      </div>
      <p v-if="!historyLoading && !historyError && !deliveries.length" class="py-4 text-center text-sm text-gray-500">{{ t('admin.settings.webhook.empty') }}</p>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'
import {
  getNotificationWebhookSettings, updateNotificationWebhookSettings,
  listNotificationWebhookDeliveries, testNotificationWebhook,
  type NotificationWebhookSettings, type NotificationWebhookDelivery
} from '@/api/admin/webhooks'

const { t, locale } = useI18n()
const appStore = useAppStore()
const saved = ref<NotificationWebhookSettings | null>(null)
const enabled = ref(false)
const url = ref('')
const secret = ref('')
const clearSecret = ref(false)
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const testing = ref(false)
const conflict = ref(false)
const loadError = ref('')
const deliveries = ref<NotificationWebhookDelivery[]>([])
const historyLoading = ref(false)
const historyError = ref('')
const busy = computed(() => loading.value || saving.value || testing.value)
const dirty = computed(() => !!saved.value && (
  enabled.value !== saved.value.enabled || url.value.trim() !== saved.value.webhook_url || secret.value !== '' || clearSecret.value
))
const canSave = computed(() => loaded.value && saved.value?.encryption_key_configured && !busy.value && !conflict.value && dirty.value && (!enabled.value || !!url.value.trim()))
const canTest = computed(() => loaded.value && saved.value?.encryption_key_configured && !busy.value && !conflict.value && !dirty.value && !!saved.value?.webhook_url)

function applySettings(value: NotificationWebhookSettings) {
  saved.value = value
  enabled.value = value.enabled
  url.value = value.webhook_url
  secret.value = ''
  clearSecret.value = false
  conflict.value = false
}

async function loadSettings() {
  if (busy.value) return
  loading.value = true
  loaded.value = false
  loadError.value = ''
  try {
    applySettings(await getNotificationWebhookSettings())
    loaded.value = true
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.settings.webhook.loadFailed'))
  } finally {
    loading.value = false
  }
}

async function loadHistory() {
  if (historyLoading.value) return
  historyLoading.value = true
  historyError.value = ''
  try {
    deliveries.value = await listNotificationWebhookDeliveries()
  } catch (error) {
    historyError.value = extractApiErrorMessage(error, t('admin.settings.webhook.historyFailed'))
  } finally {
    historyLoading.value = false
  }
}

async function save() {
  if (!canSave.value || !saved.value) return
  saving.value = true
  try {
    applySettings(await updateNotificationWebhookSettings({
      enabled: enabled.value, webhook_url: url.value.trim(), secret: secret.value,
      clear_secret: clearSecret.value, version: saved.value.version
    }))
    appStore.showSuccess(t('common.saved'))
    void loadHistory()
  } catch (error) {
    if (extractApiErrorCode(error) === 'WEBHOOK_VERSION_CONFLICT') {
      conflict.value = true
    } else {
      appStore.showError(extractApiErrorMessage(error, t('admin.settings.webhook.saveFailed')))
    }
  } finally {
    saving.value = false
  }
}

async function sendTest() {
  if (!canTest.value) return
  testing.value = true
  try {
    await testNotificationWebhook()
    appStore.showSuccess(t('admin.settings.webhook.testSuccess'))
  } catch (error) {
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.webhook.testFailed')))
  } finally {
    testing.value = false
  }
}

function formatDate(value: string) {
  return new Intl.DateTimeFormat(locale.value, { dateStyle: 'short', timeStyle: 'short' }).format(new Date(value))
}

onMounted(() => {
  void loadSettings()
  void loadHistory()
})
</script>
