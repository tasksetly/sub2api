<template>
  <BaseDialog
    :show="show"
    :title="t('admin.upstreamProviders.provisionTitle', { name: provider?.name ?? '' })"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <p class="text-sm text-gray-600 dark:text-gray-300">
        {{ t('admin.upstreamProviders.provisionHint') }}
      </p>

      <!-- 建号结果：有结果就优先展示，逐个分组独立成败 -->
      <div v-if="results.length > 0" class="space-y-2">
        <h4 class="text-sm font-medium text-gray-900 dark:text-white">
          {{ t('admin.upstreamProviders.provisionResultTitle') }}
        </h4>
        <ul class="divide-y divide-gray-200 rounded border border-gray-200 dark:divide-gray-700 dark:border-gray-700">
          <li
            v-for="item in results"
            :key="item.remote_group_id"
            class="flex items-start justify-between gap-3 px-3 py-2 text-sm"
          >
            <div>
              <div class="text-gray-900 dark:text-white">{{ item.group_name }}</div>
              <div v-if="item.error" class="text-xs text-red-600 dark:text-red-400">
                {{ item.error }}
              </div>
              <div v-else class="text-xs text-gray-500 dark:text-gray-400">
                {{ item.account_name }}
              </div>
            </div>
            <span :class="item.error ? 'badge badge-danger' : 'badge badge-success'">
              {{
                item.error
                  ? t('admin.upstreamProviders.provisionResultFailed')
                  : t('admin.upstreamProviders.provisionResultOk')
              }}
            </span>
          </li>
        </ul>
        <div class="flex justify-end pt-2">
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            {{ t('common.close') }}
          </button>
        </div>
      </div>

      <form v-else class="space-y-4" @submit.prevent="handleSubmit">
        <div>
          <label class="input-label">{{ t('admin.upstreamProviders.provisionSelectGroups') }}</label>
          <div v-if="groupsLoading" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('common.loading') }}
          </div>
          <div v-else-if="groups.length === 0" class="text-sm text-gray-500 dark:text-gray-400">
            {{ t('admin.upstreamProviders.groupsEmpty') }}
          </div>
          <div
            v-else
            class="max-h-64 divide-y divide-gray-200 overflow-y-auto rounded border border-gray-200 dark:divide-gray-700 dark:border-gray-700"
          >
            <label
              v-for="group in sortedGroups"
              :key="group.id"
              class="flex cursor-pointer items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-800/50"
            >
              <input
                type="checkbox"
                class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :value="group.remote_group_id"
                :checked="selected.has(group.remote_group_id)"
                @change="toggleGroup(group.remote_group_id)"
              />
              <span class="flex-1 text-sm">
                <span class="text-gray-900 dark:text-white">{{ group.name }}</span>
                <span class="ml-2 text-xs text-gray-500 dark:text-gray-400">
                  {{ group.platform || '—' }}
                </span>
              </span>
              <span class="text-sm font-medium text-gray-900 dark:text-white">
                ×{{ formatRate(group.comparable_rate) }}
              </span>
              <span class="w-24 text-right text-xs text-gray-500 dark:text-gray-400">
                {{ group.daily_limit_usd !== null ? `$${group.daily_limit_usd.toFixed(2)}/d` : '' }}
              </span>
            </label>
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.upstreamProviders.provisionLocalGroups') }}</label>
          <div
            v-if="localGroups.length > 0"
            class="max-h-40 divide-y divide-gray-200 overflow-y-auto rounded border border-gray-200 dark:divide-gray-700 dark:border-gray-700"
          >
            <label
              v-for="group in localGroups"
              :key="group.id"
              class="flex cursor-pointer items-center gap-3 px-3 py-2 hover:bg-gray-50 dark:hover:bg-gray-800/50"
            >
              <input
                type="checkbox"
                class="h-4 w-4 cursor-pointer rounded border-gray-300 text-primary-600 focus:ring-primary-500"
                :checked="selectedLocal.has(group.id)"
                @change="toggleLocalGroup(group.id)"
              />
              <span class="flex-1 text-sm text-gray-900 dark:text-white">{{ group.name }}</span>
              <span class="text-xs text-gray-500 dark:text-gray-400">
                ×{{ formatRate(group.rate_multiplier) }}
              </span>
            </label>
          </div>
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.upstreamProviders.provisionLocalGroupsHelp') }}
          </p>
        </div>

        <div class="grid grid-cols-2 gap-3">
          <div>
            <label class="input-label">{{ t('admin.upstreamProviders.provisionConcurrency') }}</label>
            <input v-model.number="form.concurrency" type="number" min="1" class="input" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.upstreamProviders.provisionConcurrencyHelp') }}
            </p>
          </div>
          <div>
            <label class="input-label">{{ t('admin.upstreamProviders.provisionPriority') }}</label>
            <input v-model.number="form.priority" type="number" min="0" class="input" />
          </div>
        </div>

        <div>
          <label class="input-label">{{ t('admin.upstreamProviders.provisionKeyPrefix') }}</label>
          <input v-model="form.key_name_prefix" type="text" class="input" :placeholder="provider?.name ?? ''" />
          <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
            {{ t('admin.upstreamProviders.provisionKeyPrefixHelp') }}
          </p>
        </div>

        <div class="flex justify-end gap-2 pt-2">
          <button type="button" class="btn btn-secondary" @click="emit('close')">
            {{ t('common.cancel') }}
          </button>
          <button
            type="submit"
            class="btn btn-primary"
            :disabled="submitting || selected.size === 0"
          >
            {{
              submitting
                ? t('admin.upstreamProviders.provisionRunning')
                : t('admin.upstreamProviders.provisionSubmit')
            }}
          </button>
        </div>
      </form>
    </div>
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { AdminGroup } from '@/types'
import type {
  UpstreamProviderWithStats,
  UpstreamGroup,
  ProvisionedAccount,
  ProvisionAccountsRequest
} from '@/types/upstreamProvider'
import BaseDialog from '@/components/common/BaseDialog.vue'

interface Props {
  show: boolean
  provider: UpstreamProviderWithStats | null
  groups: UpstreamGroup[]
  groupsLoading: boolean
  localGroups: AdminGroup[]
  submitting: boolean
  results: ProvisionedAccount[]
}

const props = defineProps<Props>()

const emit = defineEmits<{
  (e: 'close'): void
  (e: 'submit', payload: ProvisionAccountsRequest): void
}>()

const { t } = useI18n()

const selected = ref<Set<number>>(new Set())
const selectedLocal = ref<Set<number>>(new Set())
const form = ref({
  concurrency: undefined as number | undefined,
  priority: undefined as number | undefined,
  key_name_prefix: ''
})

// 按比价倍率升序，最便宜的排最前
const sortedGroups = computed(() =>
  [...props.groups].sort((a, b) => a.comparable_rate - b.comparable_rate)
)

watch(
  () => props.show,
  (show) => {
    if (!show) return
    selected.value = new Set()
    selectedLocal.value = new Set()
    form.value = { concurrency: undefined, priority: undefined, key_name_prefix: '' }
  }
)

function toggleGroup(remoteGroupID: number) {
  const next = new Set(selected.value)
  if (next.has(remoteGroupID)) {
    next.delete(remoteGroupID)
  } else {
    next.add(remoteGroupID)
  }
  selected.value = next
}

function toggleLocalGroup(groupID: number) {
  const next = new Set(selectedLocal.value)
  if (next.has(groupID)) {
    next.delete(groupID)
  } else {
    next.add(groupID)
  }
  selectedLocal.value = next
}

function formatRate(value: number): string {
  return Number(value.toFixed(4)).toString()
}

function handleSubmit() {
  if (selected.value.size === 0) return
  const payload: ProvisionAccountsRequest = {
    remote_group_ids: Array.from(selected.value)
  }
  if (selectedLocal.value.size > 0) {
    payload.local_group_ids = Array.from(selectedLocal.value)
  }
  // 并发留空表示用平台默认值，避免超卖上游的共享并发额度
  if (form.value.concurrency && form.value.concurrency > 0) {
    payload.concurrency = form.value.concurrency
  }
  if (form.value.priority !== undefined && form.value.priority >= 0) {
    payload.priority = form.value.priority
  }
  const prefix = form.value.key_name_prefix.trim()
  if (prefix !== '') {
    payload.key_name_prefix = prefix
  }
  emit('submit', payload)
}
</script>
