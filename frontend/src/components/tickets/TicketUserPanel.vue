<template>
  <section class="space-y-4 rounded-md border border-gray-200 bg-white p-4 dark:border-dark-700 dark:bg-dark-800">
    <div class="flex items-center justify-between gap-3">
      <h2 class="font-semibold text-gray-900 dark:text-white">{{ t('tickets.userActions.title') }}</h2>
      <button type="button" class="btn btn-ghost" :disabled="loadingUser || loadingUsage" @click="refresh">
        <Icon name="refresh" size="sm" :class="{ 'animate-spin': loadingUser || loadingUsage }" />
        {{ t('common.refresh') }}
      </button>
    </div>

    <p v-if="loadingUser" class="text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
    <p v-else-if="userError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ userError }}</p>
    <template v-else-if="user">
      <dl class="grid gap-3 text-sm sm:grid-cols-2 lg:grid-cols-4">
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('tickets.userActions.userId') }}</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">#{{ user.id }}</dd>
        </div>
        <div class="min-w-0">
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.email') }}</dt>
          <dd class="mt-1 break-all text-gray-900 dark:text-white">{{ user.email }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.username') }}</dt>
          <dd class="mt-1 break-all text-gray-900 dark:text-white">{{ user.username || '—' }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('common.status') }}</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">{{ user.deleted_at ? t('admin.usage.userDeletedBadge') : t(user.status === 'active' ? 'common.active' : 'common.disabled') }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.currentBalance') }}</dt>
          <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCurrency(user.balance) }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.columns.concurrency') }}</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">{{ user.concurrency }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.form.rpmLimit') }}</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">{{ user.rpm_limit || t('admin.users.bulkLimits.unlimited') }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.createdAt') }}</dt>
          <dd class="mt-1 text-gray-900 dark:text-white">{{ formatDateTime(user.created_at) }}</dd>
        </div>
        <div v-if="user.notes" class="sm:col-span-2 lg:col-span-4">
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.users.notes') }}</dt>
          <dd class="mt-1 whitespace-pre-wrap break-words text-gray-900 dark:text-white">{{ user.notes }}</dd>
        </div>
      </dl>

      <div class="flex flex-wrap gap-2">
        <template v-if="!user.deleted_at">
          <button type="button" class="btn btn-secondary" @click="openBalance('add')">{{ t('admin.users.deposit') }}</button>
          <button type="button" class="btn btn-secondary" @click="openBalance('subtract')">{{ t('admin.users.withdraw') }}</button>
          <button type="button" class="btn btn-secondary" @click="dialog = 'limits'">{{ t('tickets.userActions.limits') }}</button>
          <button type="button" class="btn btn-secondary" @click="dialog = 'keys'">{{ t('admin.users.viewApiKeys') }}</button>
        </template>
        <button type="button" class="btn btn-secondary" @click="dialog = 'history'">{{ t('tickets.userActions.balanceHistory') }}</button>
      </div>

      <UserBalanceModal
        :show="dialog === 'balance'"
        :user="user"
        :operation="balanceOperation"
        :context-note="t('tickets.userActions.balanceContext', { id: ticketId })"
        @close="dialog = null"
        @success="refresh"
      />
      <UserBalanceHistoryModal
        :show="dialog === 'history'"
        :user="user"
        :hide-actions="!!user.deleted_at"
        @close="dialog = null"
        @deposit="openBalance('add')"
        @withdraw="openBalance('subtract')"
      />
      <BulkEditUserModal
        :show="dialog === 'limits'"
        :title="t('tickets.userActions.limits')"
        :selected-ids="[user.id]"
        :user="user"
        @close="dialog = null"
        @success="refresh"
      />
      <UserApiKeysModal :show="dialog === 'keys'" :user="user" @close="dialog = null" />
    </template>

    <div class="space-y-3 border-t border-gray-200 pt-4 dark:border-dark-700">
      <div class="flex flex-wrap items-center justify-between gap-2">
        <h3 class="text-sm font-medium text-gray-900 dark:text-white">{{ t('tickets.userActions.usageTitle') }}</h3>
        <RouterLink class="text-sm text-primary-600 hover:underline dark:text-primary-400" :to="usageLink">
          {{ t('tickets.userActions.usageDetails') }}
        </RouterLink>
      </div>
      <p v-if="loadingUsage" class="text-sm text-gray-500" role="status">{{ t('common.loading') }}</p>
      <p v-else-if="usageError" class="text-sm text-red-600 dark:text-red-400" role="alert">{{ usageError }}</p>
      <dl v-else-if="usage" class="grid grid-cols-3 gap-3 text-sm">
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.totalRequests') }}</dt>
          <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatNumber(usage.total_requests) }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.totalTokens') }}</dt>
          <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatNumber(usage.total_tokens) }}</dd>
        </div>
        <div>
          <dt class="text-gray-500 dark:text-gray-400">{{ t('admin.dashboard.actualSpending') }}</dt>
          <dd class="mt-1 font-semibold text-gray-900 dark:text-white">{{ formatCurrency(usage.total_actual_cost) }}</dd>
        </div>
      </dl>
    </div>
  </section>
</template>

<script setup lang="ts">
import { computed, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { RouterLink } from 'vue-router'
import { adminAPI } from '@/api/admin'
import type { AdminUsageStatsResponse } from '@/api/admin/usage'
import type { AdminUser } from '@/types'
import Icon from '@/components/icons/Icon.vue'
import UserBalanceModal from '@/components/admin/user/UserBalanceModal.vue'
import UserBalanceHistoryModal from '@/components/admin/user/UserBalanceHistoryModal.vue'
import UserApiKeysModal from '@/components/admin/user/UserApiKeysModal.vue'
import BulkEditUserModal from '@/components/admin/user/BulkEditUserModal.vue'
import { extractApiErrorMessage } from '@/utils/apiError'
import { formatCurrency, formatDateLocalInput, formatDateTime, formatNumber, getBrowserTimeZone } from '@/utils/format'

const props = defineProps<{ userId: number; ticketId: number }>()
const { t } = useI18n()
const user = ref<AdminUser | null>(null)
const usage = ref<AdminUsageStatsResponse | null>(null)
const userError = ref('')
const usageError = ref('')
const loadingUser = ref(false)
const loadingUsage = ref(false)
const dialog = ref<'balance' | 'history' | 'limits' | 'keys' | null>(null)
const balanceOperation = ref<'add' | 'subtract'>('add')
let requestVersion = 0

function getUsageRange() {
  const end = new Date()
  const start = new Date(end)
  start.setDate(start.getDate() - 29)
  return { start_date: formatDateLocalInput(start), end_date: formatDateLocalInput(end) }
}

const usageRange = ref(getUsageRange())
const usageLink = computed(() => ({
  path: '/admin/usage',
  query: { user_id: props.userId, ...usageRange.value }
}))

function openBalance(operation: 'add' | 'subtract') {
  balanceOperation.value = operation
  dialog.value = 'balance'
}

async function refresh() {
  const version = ++requestVersion
  const id = props.userId
  loadingUser.value = true
  loadingUsage.value = true
  userError.value = ''
  usageError.value = ''
  usageRange.value = getUsageRange()
  const results = await Promise.allSettled([
    adminAPI.users.getById(id, true),
    adminAPI.usage.getStats({ user_id: id, ...usageRange.value, timezone: getBrowserTimeZone() })
  ])
  if (version !== requestVersion) return
  const [userResult, usageResult] = results
  if (userResult.status === 'fulfilled') {
    user.value = userResult.value
  } else {
    user.value = null
    userError.value = extractApiErrorMessage(userResult.reason, t('tickets.userActions.loadFailed'))
  }
  if (usageResult.status === 'fulfilled') {
    usage.value = usageResult.value
  } else {
    usage.value = null
    usageError.value = extractApiErrorMessage(usageResult.reason, t('tickets.userActions.usageFailed'))
  }
  loadingUser.value = false
  loadingUsage.value = false
}

watch(() => props.userId, () => {
  dialog.value = null
  user.value = null
  usage.value = null
  void refresh()
}, { immediate: true })
onUnmounted(() => { requestVersion++ })
</script>
