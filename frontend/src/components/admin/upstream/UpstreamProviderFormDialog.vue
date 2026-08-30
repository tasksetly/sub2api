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
        <label class="input-label">{{ t('admin.upstreamProviders.formRateCorrection') }}</label>
        <input
          v-model="form.rate_correction"
          type="number"
          class="input"
          min="0"
          step="0.000001"
          placeholder="1"
        />
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{ t('admin.upstreamProviders.formRateCorrectionHelp') }}
        </p>
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
    // 字符串而非 number：type="number" 的 v-model 给的是字符串，清空时是 ''，
    // 留到提交时统一收敛成 1（不修正）
    rate_correction: '1',
    totp_secret: '',
    notes: '',
    status: 'active' as 'active' | 'inactive',
    sync_enabled: true
  }
}

// 非法/清空一律按「不修正」处理，与后端 NormalizeRateCorrection 口径一致。
// 不能传 0：后端 binding 是 gt=0，且 0 会让所有分组比价倍率变成 0 排到最前。
function normalizedRateCorrection(): number {
  const parsed = Number.parseFloat(form.value.rate_correction)
  if (!Number.isFinite(parsed) || parsed <= 0) return 1
  return parsed
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
        rate_correction: String(editing.rate_correction ?? 1),
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
    rate_correction: normalizedRateCorrection(),
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
