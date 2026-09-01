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
        <label class="input-label">{{ t('admin.upstreamProviders.formToken') }}</label>
        <textarea
          v-model="form.token"
          rows="3"
          class="input font-mono text-xs"
          autocomplete="off"
          spellcheck="false"
          :placeholder="
            editing && editing.has_token
              ? t('admin.upstreamProviders.formPasswordPlaceholderEdit')
              : t('admin.upstreamProviders.formTokenPlaceholder')
          "
        ></textarea>
        <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
          {{
            editing
              ? t('admin.upstreamProviders.formTokenHelpEdit')
              : t('admin.upstreamProviders.formTokenHelp')
          }}
        </p>
        <!-- 只有 token 没有密码的上游过期后无法自愈，把到期时间摆在管理员眼前 -->
        <p v-if="tokenExpiryHint" class="mt-1 text-xs" :class="tokenExpiryClass">
          {{ tokenExpiryHint }}
        </p>
      </div>

      <!-- 新增时密码和 token 二选一，浏览器的 required 管不了这种跨字段约束 -->
      <p v-if="credentialsError" class="text-xs text-red-600 dark:text-red-400">
        {{ t('admin.upstreamProviders.formTokenRequired') }}
      </p>

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
    token: '',
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
const credentialsError = ref(false)

const statusOptions = computed(() => [
  { value: 'active', label: t('common.active') },
  { value: 'inactive', label: t('common.inactive') }
])

// 已存 access token 的到期提示。没有 refresh token 且没有密码的上游过期后
// 没有自动续期手段，得管理员再贴一个，所以这里额外点明。
const tokenExpiryHint = computed(() => {
  const provider = props.editing
  if (!provider?.has_token || !provider.token_expires_at) return ''

  const expiresAt = new Date(provider.token_expires_at)
  if (Number.isNaN(expiresAt.getTime())) return ''

  if (expiresAt.getTime() <= Date.now()) {
    return t('admin.upstreamProviders.tokenExpired')
  }
  const base = `${t('admin.upstreamProviders.tokenExpiresAt')}: ${expiresAt.toLocaleString()}`
  return provider.has_password || provider.has_refresh_token
    ? base
    : `${base} · ${t('admin.upstreamProviders.tokenNoAutoRenew')}`
})

const tokenExpiryClass = computed(() => {
  const provider = props.editing
  const expiresAt = provider?.token_expires_at ? new Date(provider.token_expires_at) : null
  const expired = expiresAt !== null && !Number.isNaN(expiresAt.getTime())
    && expiresAt.getTime() <= Date.now()
  return expired
    ? 'text-red-600 dark:text-red-400'
    : 'text-amber-600 dark:text-amber-400'
})

// 打开弹窗时回填。密码、TOTP 密钥和 token 永远留空——后端不回显明文，
// 留空也正好表示「不修改」。
watch(
  () => [props.show, props.editing] as const,
  ([show, editing]) => {
    if (!show) return
    credentialsError.value = false
    if (editing) {
      form.value = {
        name: editing.name,
        base_url: editing.base_url,
        username: editing.username,
        password: '',
        token: '',
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
  const token = form.value.token.trim()

  // 新增时二选一：能自动登录的填密码，被 CF 挡住的贴 token。
  // 编辑时都可留空（表示不修改），已有凭据不该被这条挡住。
  if (!props.editing && form.value.password === '' && token === '') {
    credentialsError.value = true
    return
  }
  credentialsError.value = false

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
    if (token !== '') payload.token = token
    emit('submit', payload)
    return
  }

  const payload: CreateUpstreamProviderRequest = { ...base }
  if (form.value.password !== '') payload.password = form.value.password
  if (form.value.totp_secret !== '') payload.totp_secret = form.value.totp_secret
  if (token !== '') payload.token = token
  emit('submit', payload)
}
</script>
