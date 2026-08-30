<template>
  <BaseDialog
    :show="show"
    :title="editing ? t('admin.upstreamProviders.editProvider') : t('admin.upstreamProviders.createProvider')"
    @close="emit('close')"
  >
    <form class="space-y-4" @submit.prevent="handleSubmit">
      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formName') }}</label>
        <input v-model="form.name" type="text" class="input" required />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamProviders.formNameHelp') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formBaseURL') }}</label>
        <input
          v-model="form.base_url"
          type="url"
          class="input"
          placeholder="https://example.com"
          required
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamProviders.formBaseURLHelp') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formUsername') }}</label>
        <input v-model="form.username" type="email" class="input" required />
      </div>

      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formPassword') }}</label>
        <input
          v-model="form.password"
          type="password"
          class="input"
          autocomplete="new-password"
          :placeholder="editing ? t('admin.upstreamProviders.formPasswordPlaceholderEdit') : ''"
          :required="!editing"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{
            editing
              ? t('admin.upstreamProviders.formPasswordHelpEdit')
              : t('admin.upstreamProviders.formPasswordHelpCreate')
          }}
        </p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formTotpSecret') }}</label>
        <input
          v-model="form.totp_secret"
          type="password"
          class="input"
          autocomplete="off"
          :placeholder="editing && editing.has_totp_secret ? t('admin.upstreamProviders.formPasswordPlaceholderEdit') : ''"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">{{ t('admin.upstreamProviders.formTotpSecretHelp') }}</p>
      </div>

      <div>
        <label class="input-label">{{ t('admin.upstreamProviders.formNotes') }}</label>
        <textarea v-model="form.notes" rows="2" class="input"></textarea>
      </div>

      <div v-if="editing">
        <label class="input-label">{{ t('admin.upstreamProviders.formStatus') }}</label>
        <Select v-model="form.status" :options="statusOptions" />
      </div>

      <label class="flex cursor-pointer items-start gap-2">
        <input
          v-model="form.sync_enabled"
          type="checkbox"
          class="mt-0.5 h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
        />
        <span>
          <span class="text-sm text-gray-900 dark:text-white">
            {{ t('admin.upstreamProviders.formSyncEnabled') }}
          </span>
          <span class="mt-1 block text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.upstreamProviders.formSyncEnabledHelp') }}
          </span>
        </span>
      </label>

      <div class="flex justify-end gap-2 pt-2">
        <button type="button" class="btn btn-secondary" @click="emit('close')">
          {{ t('common.cancel') }}
        </button>
        <button type="submit" class="btn btn-primary" :disabled="submitting">
          {{ submitting ? t('common.saving') : t('common.save') }}
        </button>
      </div>
    </form>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type {
  UpstreamProviderWithStats,
  CreateUpstreamProviderRequest,
  UpdateUpstreamProviderRequest
} from '@/types/upstreamProvider'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Select from '@/components/common/Select.vue'

interface Props {
  show: boolean
  editing: UpstreamProviderWithStats | null
  submitting: boolean
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: CreateUpstreamProviderRequest | UpdateUpstreamProviderRequest): void
}>()

const { t } = useI18n()

function emptyForm() {
  return {
    name: '',
    base_url: '',
    username: '',
    password: '',
    totp_secret: '',
    notes: '',
    status: 'active' as 'active' | 'inactive',
    sync_enabled: true
  }
}

const form = ref(emptyForm())

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

// 打开弹窗时回填。密码和 TOTP 密钥永远留空——后端不回显明文，
// 留空也正好表示「不修改」。
watch(
  () => [props.show, props.editing] as const,
  ([show, editing]) => {
    if (!show) return
    if (editing) {
      form.value = {
        name: editing.name,
        base_url: editing.base_url,
        username: editing.username,
        password: '',
        totp_secret: '',
        notes: editing.notes ?? '',
        status: editing.status,
        sync_enabled: editing.sync_enabled
      }
    } else {
      form.value = emptyForm()
    }
  },
  { immediate: true }
)

function handleSubmit() {
  const notes = form.value.notes.trim()
  const base = {
    name: form.value.name.trim(),
    base_url: form.value.base_url.trim(),
    username: form.value.username.trim(),
    notes: notes === '' ? null : notes,
    sync_enabled: form.value.sync_enabled
  }

  if (props.editing) {
    const payload: UpdateUpstreamProviderRequest = {
      ...base,
      status: form.value.status
    }
    // 空字符串表示不修改，不要传上去覆盖成空
    if (form.value.password !== '') payload.password = form.value.password
    if (form.value.totp_secret !== '') payload.totp_secret = form.value.totp_secret
    emit('submit', payload)
    return
  }

  const payload: CreateUpstreamProviderRequest = {
    ...base,
    password: form.value.password
  }
  if (form.value.totp_secret !== '') payload.totp_secret = form.value.totp_secret
  emit('submit', payload)
}
</script>
