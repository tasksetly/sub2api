<template>
  <section class="card space-y-4 p-6">
    <div class="flex flex-wrap items-center justify-between gap-3">
      <h2 class="text-lg font-semibold text-gray-900 dark:text-white">{{ t('admin.settings.globalIPBlacklist.title') }}</h2>
      <button type="button" class="btn btn-ghost" :disabled="loading || saving" @click="load">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loading }" />
        {{ t('common.refresh') }}
      </button>
    </div>
    <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.globalIPBlacklist.description') }}</p>
    <p v-if="loading" class="text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
    <p v-if="loadError" role="alert" class="text-sm text-red-600 dark:text-red-400">{{ loadError }}</p>
    <p v-if="conflict" role="alert" class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.globalIPBlacklist.conflict') }}</p>
    <fieldset :disabled="!loaded || loading || saving" class="space-y-4">
      <label class="flex items-center justify-between gap-4 text-sm text-gray-900 dark:text-white">
        {{ t('admin.settings.globalIPBlacklist.enabled') }}
        <Toggle v-model="enabled" :aria-label="t('admin.settings.globalIPBlacklist.enabled')" />
      </label>
      <p v-if="currentIP" class="text-sm text-gray-700 dark:text-gray-300">
        {{ t('admin.settings.globalIPBlacklist.currentIP') }} <code class="break-all font-mono">{{ currentIP }}</code>
      </p>
      <p class="text-sm text-amber-700 dark:text-amber-400">{{ t('admin.settings.globalIPBlacklist.proxyHint') }}</p>
      <label class="block text-sm text-gray-700 dark:text-gray-300">
        {{ t('admin.settings.globalIPBlacklist.rules') }}
        <textarea
          v-model="rulesText"
          rows="8"
          class="input mt-2 font-mono"
          spellcheck="false"
          :placeholder="t('admin.settings.globalIPBlacklist.placeholder')"
        />
      </label>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.globalIPBlacklist.rulesHint', { max: maxRules }) }}</p>
      <p class="text-sm text-gray-500 dark:text-gray-400">{{ t('admin.settings.globalIPBlacklist.effectHint') }}</p>
      <p v-if="rules.length > maxRules" role="alert" class="text-sm text-red-600 dark:text-red-400">
        {{ t('admin.settings.globalIPBlacklist.tooMany', { max: maxRules }) }}
      </p>
      <button type="button" class="btn btn-primary" :disabled="!canSave" @click="requestSave">
        {{ saving ? t('common.saving') : t('common.save') }}
      </button>
    </fieldset>
    <ConfirmDialog
      :show="confirmVisible"
      :title="t('admin.settings.globalIPBlacklist.title')"
      :message="t('admin.settings.globalIPBlacklist.confirm', { count: rules.length })"
      danger
      @confirm="save"
      @cancel="confirmVisible = false"
    />
  </section>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import Toggle from '@/components/common/Toggle.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { getGlobalIPBlacklistSettings, updateGlobalIPBlacklistSettings } from '@/api/admin/settings'
import { useAppStore } from '@/stores/app'
import { extractApiErrorCode, extractApiErrorMessage } from '@/utils/apiError'

const { t } = useI18n()
const appStore = useAppStore()
const enabled = ref(false)
const rulesText = ref('')
const currentIP = ref('')
const version = ref('')
const conflict = ref(false)
const maxRules = ref(5000)
const loaded = ref(false)
const loading = ref(false)
const saving = ref(false)
const loadError = ref('')
const confirmVisible = ref(false)
const rules = computed(() => rulesText.value.split(/\r?\n/).map(rule => rule.trim()).filter(Boolean))
const canSave = computed(() => loaded.value && !loading.value && !saving.value && !conflict.value && rules.value.length <= maxRules.value)

async function load() {
  loading.value = true
  loaded.value = false
  loadError.value = ''
  try {
    const result = await getGlobalIPBlacklistSettings()
    enabled.value = result.enabled
    rulesText.value = result.rules.join('\n')
    currentIP.value = result.current_ip
    maxRules.value = result.max_rules
    version.value = result.version
    conflict.value = false
    loaded.value = true
  } catch (error) {
    loadError.value = extractApiErrorMessage(error, t('admin.settings.globalIPBlacklist.loadFailed'))
  } finally {
    loading.value = false
  }
}

function requestSave() {
  if (!canSave.value) return
  if (enabled.value && rules.value.length > 0) {
    confirmVisible.value = true
  } else {
    void save()
  }
}

async function save() {
  if (!canSave.value) return
  confirmVisible.value = false
  saving.value = true
  try {
    const result = await updateGlobalIPBlacklistSettings({ enabled: enabled.value, rules: rules.value, version: version.value })
    enabled.value = result.enabled
    rulesText.value = result.rules.join('\n')
    currentIP.value = result.current_ip
    maxRules.value = result.max_rules
    version.value = result.version
    appStore.showSuccess(t('common.saved'))
  } catch (error) {
    if (extractApiErrorCode(error) === 'IP_BLACKLIST_VERSION_CONFLICT') {
      conflict.value = true
      return
    }
    appStore.showError(extractApiErrorMessage(error, t('admin.settings.globalIPBlacklist.saveFailed')))
  } finally {
    saving.value = false
  }
}

onMounted(load)
</script>
