<template>
  <div class="space-y-4">
    <!-- 概览：参考库、消耗、资格 -->
    <div
      v-if="overview"
      class="rounded-lg border border-gray-200 bg-gray-50 p-3 text-xs dark:border-gray-700 dark:bg-gray-800/50"
    >
      <div class="flex flex-wrap items-center gap-x-4 gap-y-1 text-gray-600 dark:text-gray-300">
        <span v-if="overview.reference_built_at">
          {{ t('admin.accounts.fingerprint.referenceBuiltAt') }}:
          {{ formatTime(overview.reference_built_at) }}
        </span>
        <span v-else-if="overview.reference_error" class="text-amber-600 dark:text-amber-400">
          {{ t('admin.accounts.fingerprint.referenceUnavailable') }}: {{ overview.reference_error }}
        </span>
        <span>
          {{ t('admin.accounts.fingerprint.monthlyTokens') }}:
          {{ overview.monthly_estimated_tokens.toLocaleString() }}
        </span>
      </div>

      <p v-if="overview.eligibility.proxy_bypassed" class="mt-2 text-amber-600 dark:text-amber-400">
        {{ t('admin.accounts.fingerprint.proxyBypassHint') }}
      </p>
    </div>

    <!-- 账号不支持：置灰并写明原因，不让人以为功能坏了 -->
    <div
      v-if="overview && !overview.eligibility.supported"
      data-testid="fingerprint-unsupported"
      class="rounded-lg border border-amber-200 bg-amber-50 p-4 text-sm text-amber-800 dark:border-amber-900/50 dark:bg-amber-900/20 dark:text-amber-200"
    >
      {{ t(`admin.accounts.fingerprint.reasons.${overview.eligibility.reason || 'unknown'}`) }}
    </div>

    <template v-else>
      <!-- 计划列表 -->
      <div v-if="loading" class="py-6 text-center text-sm text-gray-500">
        {{ t('common.loading') }}
      </div>

      <div
        v-else-if="plans.length === 0"
        class="py-6 text-center text-sm text-gray-500 dark:text-gray-400"
      >
        {{ t('admin.accounts.fingerprint.empty') }}
      </div>

      <div v-else class="space-y-2">
        <div
          v-for="plan in plans"
          :key="plan.id"
          class="rounded-lg border border-gray-200 dark:border-gray-700"
        >
          <div class="flex flex-wrap items-center gap-3 p-3">
            <div class="min-w-0 flex-1">
              <div class="flex items-center gap-2 text-sm font-medium text-gray-900 dark:text-gray-100">
                <span class="truncate">{{ plan.expected_model }}</span>
                <span
                  v-if="plan.consecutive_mismatches > 0"
                  class="rounded px-1.5 py-0.5 text-xs"
                  :class="mismatchCounterClass(plan)"
                >
                  {{
                    t('admin.accounts.fingerprint.consecutiveMismatches', {
                      n: plan.consecutive_mismatches,
                      threshold: plan.mismatch_action_threshold
                    })
                  }}
                </span>
              </div>
              <div class="mt-0.5 truncate text-xs text-gray-500 dark:text-gray-400">
                {{ t('admin.accounts.fingerprint.testModel') }}: {{ plan.test_model }} ·
                {{ plan.cron_expression }} ·
                {{ t('admin.accounts.fingerprint.challenges', { n: plan.challenge_count }) }}
              </div>
            </div>

            <Toggle
              :model-value="plan.enabled"
              @update:model-value="(value) => toggleEnabled(plan, value)"
            />

            <button
              type="button"
              class="rounded border border-gray-300 px-2 py-1 text-xs hover:bg-gray-50 disabled:opacity-50 dark:border-gray-600 dark:hover:bg-gray-700"
              :disabled="runningPlanId === plan.id"
              data-testid="fingerprint-run-now"
              @click="runNow(plan)"
            >
              {{ t('admin.accounts.fingerprint.runNow') }}
            </button>
            <button
              type="button"
              class="rounded border border-gray-300 px-2 py-1 text-xs hover:bg-gray-50 dark:border-gray-600 dark:hover:bg-gray-700"
              @click="toggleExpand(plan.id)"
            >
              {{ t('admin.accounts.fingerprint.results') }}
            </button>
            <button
              type="button"
              class="rounded border border-red-300 px-2 py-1 text-xs text-red-600 hover:bg-red-50 dark:border-red-800 dark:text-red-400 dark:hover:bg-red-900/20"
              @click="remove(plan)"
            >
              {{ t('common.delete') }}
            </button>
          </div>

          <!-- 结果列表 -->
          <div
            v-if="expandedPlanId === plan.id"
            class="border-t border-gray-200 p-3 dark:border-gray-700"
          >
            <div v-if="loadingResults" class="text-xs text-gray-500">{{ t('common.loading') }}</div>
            <div v-else-if="results.length === 0" class="text-xs text-gray-500">
              {{ t('admin.accounts.fingerprint.noResults') }}
            </div>
            <ul v-else class="space-y-2">
              <li
                v-for="result in results"
                :key="result.id"
                class="rounded border border-gray-100 p-2 text-xs dark:border-gray-700"
                data-testid="fingerprint-result"
              >
                <div class="flex flex-wrap items-center gap-2">
                  <span
                    class="rounded px-1.5 py-0.5 font-medium"
                    :class="verdictClass(result.verdict)"
                    :data-verdict="result.verdict"
                  >
                    {{ t(`admin.accounts.fingerprint.verdict.${result.verdict}`) }}
                  </span>
                  <span class="text-gray-600 dark:text-gray-300">
                    {{ formatTime(result.created_at) }}
                  </span>
                  <span v-if="result.predicted_model" class="text-gray-700 dark:text-gray-200">
                    {{ t('admin.accounts.fingerprint.predicted') }}:
                    {{ result.predicted_model }}
                    <template v-if="result.probability !== null">
                      ({{ (result.probability * 100).toFixed(0) }}%)
                    </template>
                  </span>
                  <span v-if="result.decision" class="text-gray-400">
                    {{ result.decision }}
                  </span>
                  <span class="text-gray-400">
                    {{ t('admin.accounts.fingerprint.cost', { n: result.estimated_output_tokens }) }}
                  </span>
                </div>

                <!-- 无法判定 / 检测失败：必须显示具体原因，不能沉默 -->
                <p
                  v-if="result.error_message"
                  class="mt-1 break-all text-amber-600 dark:text-amber-400"
                  data-testid="fingerprint-error-message"
                >
                  {{ result.error_message }}
                </p>
                <p
                  v-else-if="result.skip_reason"
                  class="mt-1 text-amber-600 dark:text-amber-400"
                  data-testid="fingerprint-skip-reason"
                >
                  {{ t(`admin.accounts.fingerprint.skip.${result.skip_reason}`) }}
                </p>

                <!-- 完整候选排名 -->
                <ul
                  v-if="candidatesOf(result).length > 0"
                  class="mt-1 space-y-0.5 text-gray-500 dark:text-gray-400"
                  data-testid="fingerprint-candidates"
                >
                  <li v-for="candidate in candidatesOf(result)" :key="candidate.model">
                    {{ candidate.display_name || candidate.model }}
                    <template v-if="candidate.probability !== null && candidate.probability !== undefined">
                      — {{ (candidate.probability * 100).toFixed(1) }}%
                    </template>
                  </li>
                </ul>
              </li>
            </ul>
          </div>
        </div>
      </div>

      <!-- 新建计划 -->
      <div class="rounded-lg border border-dashed border-gray-300 p-3 dark:border-gray-600">
        <div class="grid gap-3 sm:grid-cols-2">
          <Input
            v-model="form.test_model"
            :label="t('admin.accounts.fingerprint.testModel')"
            :hint="t('admin.accounts.fingerprint.testModelHint')"
            :placeholder="t('admin.accounts.fingerprint.testModelPlaceholder')"
          />
          <div>
            <label class="mb-1 block text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.fingerprint.expectedModel') }}
            </label>
            <Select
              v-model="form.expected_model"
              :options="modelOptions"
              :disabled="modelOptions.length === 0"
              :placeholder="
                modelOptions.length === 0
                  ? t('admin.accounts.fingerprint.referenceUnavailable')
                  : t('admin.accounts.fingerprint.expectedModelPlaceholder')
              "
              data-testid="fingerprint-expected-model"
            />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.fingerprint.expectedModelHint') }}
            </p>
          </div>
          <Input
            v-model="form.cron_expression"
            :label="t('admin.accounts.fingerprint.cron')"
            :hint="t('admin.accounts.fingerprint.cronHint')"
          />
          <div>
            <label class="mb-1 block text-sm text-gray-700 dark:text-gray-300">
              {{ t('admin.accounts.fingerprint.challengeCount') }}
            </label>
            <Select v-model="form.challenge_count" :options="challengeOptions" />
            <p class="mt-1 text-xs text-gray-500 dark:text-gray-400">
              {{ t('admin.accounts.fingerprint.challengeCountHint') }}
            </p>
          </div>
        </div>

        <div class="mt-3 space-y-2 text-sm">
          <p class="text-xs text-gray-500 dark:text-gray-400">
            {{
              t('admin.accounts.fingerprint.failureActionsHint', {
                threshold: form.mismatch_action_threshold
              })
            }}
          </p>
          <label class="flex items-center gap-2">
            <input v-model="form.failure_disable" type="checkbox" class="rounded" />
            <span>{{ t('admin.accounts.fingerprint.failureDisable') }}</span>
          </label>
          <label class="flex items-center gap-2">
            <span>{{ t('admin.accounts.fingerprint.failureTempUnschedulable') }}</span>
            <input
              v-model.number="form.failure_temp_unschedulable_minutes"
              type="number"
              min="0"
              class="w-20 rounded border border-gray-300 px-2 py-1 text-sm dark:border-gray-600 dark:bg-gray-800"
            />
            <span class="text-xs text-gray-500">{{ t('admin.accounts.fingerprint.minutes') }}</span>
          </label>
        </div>

        <button
          type="button"
          class="mt-3 rounded bg-blue-600 px-3 py-1.5 text-sm text-white hover:bg-blue-700 disabled:opacity-50"
          :disabled="creating || !canSubmit"
          data-testid="fingerprint-create"
          @click="create"
        >
          {{ t('admin.accounts.fingerprint.create') }}
        </button>
      </div>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'

import Input from '@/components/common/Input.vue'
import Select from '@/components/common/Select.vue'
import Toggle from '@/components/common/Toggle.vue'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import type {
  FingerprintCandidate,
  FingerprintOverview,
  FingerprintPlan,
  FingerprintRefModel,
  FingerprintResult,
  FingerprintVerdict
} from '@/types'

const props = defineProps<{ accountId: number }>()

const { t } = useI18n()
const appStore = useAppStore()

const loading = ref(false)
const creating = ref(false)
const loadingResults = ref(false)
const runningPlanId = ref<number | null>(null)
const expandedPlanId = ref<number | null>(null)

const plans = ref<FingerprintPlan[]>([])
const results = ref<FingerprintResult[]>([])
const refModels = ref<FingerprintRefModel[]>([])
const overview = ref<FingerprintOverview | null>(null)

const form = reactive({
  test_model: '',
  expected_model: '',
  cron_expression: '0 */6 * * *',
  challenge_count: 3,
  mismatch_action_threshold: 2,
  failure_disable: false,
  failure_temp_unschedulable_minutes: 0
})

/** 预期模型只能从参考库里选，所以下拉为空时不允许建计划。 */
const modelOptions = computed(() =>
  refModels.value.map((model) => ({
    value: model.id,
    label: `${model.family_name} / ${model.display_name || model.id}`
  }))
)

const challengeOptions = [
  { value: 3, label: '3' },
  { value: 2, label: '2' },
  { value: 1, label: '1' }
]

const canSubmit = computed(
  () =>
    // 概览拉不到时资格未知，不能放行提交——后端会拒，但管理员看到的会是一句看不懂的报错。
    overview.value?.eligibility.supported === true &&
    form.test_model.trim() !== '' &&
    form.expected_model !== '' &&
    modelOptions.value.length > 0
)

function formatTime(value: string): string {
  if (!value) return ''
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString()
}

function verdictClass(verdict: FingerprintVerdict): string {
  switch (verdict) {
    case 'match':
      return 'bg-green-100 text-green-700 dark:bg-green-900/30 dark:text-green-300'
    case 'mismatch':
      return 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    case 'inconclusive':
      return 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'
    default:
      return 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
  }
}

function mismatchCounterClass(plan: FingerprintPlan): string {
  return plan.consecutive_mismatches >= plan.mismatch_action_threshold
    ? 'bg-red-100 text-red-700 dark:bg-red-900/30 dark:text-red-300'
    : 'bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-300'
}

/** 从 analysis_json 解析候选排名，解析不了就当没有，不要让面板炸掉。 */
function candidatesOf(result: FingerprintResult): FingerprintCandidate[] {
  if (!result.analysis_json) return []
  try {
    const parsed = JSON.parse(result.analysis_json)
    // 不截断：规格要求管理员能复核完整候选排名，截断就等于拿不出完整证据。
    return Array.isArray(parsed?.results) ? (parsed.results as FingerprintCandidate[]) : []
  } catch {
    return []
  }
}

async function loadOverview() {
  try {
    overview.value = await adminAPI.fingerprint.overview(props.accountId)
  } catch {
    overview.value = null
  }
}

async function loadModels() {
  try {
    refModels.value = await adminAPI.fingerprint.referenceModels()
  } catch {
    refModels.value = []
  }
}

async function loadPlans() {
  loading.value = true
  try {
    plans.value = await adminAPI.fingerprint.listByAccount(props.accountId)
  } catch (error) {
    appStore.showError(String(error))
  } finally {
    loading.value = false
  }
}

async function toggleExpand(planId: number) {
  if (expandedPlanId.value === planId) {
    expandedPlanId.value = null
    return
  }
  expandedPlanId.value = planId
  loadingResults.value = true
  try {
    results.value = await adminAPI.fingerprint.listResults(planId)
  } catch (error) {
    appStore.showError(String(error))
  } finally {
    loadingResults.value = false
  }
}

async function toggleEnabled(plan: FingerprintPlan, enabled: boolean) {
  try {
    // 必须回传完整计划：后端对失败动作字段是按「传了才覆盖」处理的，
    // 但分组与分钟数若此处漏掉，仍会让界面与后端对这些值的认知分叉。
    await adminAPI.fingerprint.update(plan.id, {
      test_model: plan.test_model,
      expected_model: plan.expected_model,
      api: plan.api,
      challenge_count: plan.challenge_count,
      cron_expression: plan.cron_expression,
      max_results: plan.max_results,
      mismatch_action_threshold: plan.mismatch_action_threshold,
      failure_disable: plan.failure_disable,
      failure_temp_unschedulable_minutes: plan.failure_temp_unschedulable_minutes,
      failure_remove_group_ids: plan.failure_remove_group_ids ?? [],
      success_add_group_ids: plan.success_add_group_ids ?? [],
      enabled
    })
    plan.enabled = enabled
  } catch (error) {
    appStore.showError(String(error))
  }
}

async function create() {
  creating.value = true
  try {
    await adminAPI.fingerprint.create({
      account_id: props.accountId,
      test_model: form.test_model.trim(),
      expected_model: form.expected_model,
      cron_expression: form.cron_expression,
      challenge_count: form.challenge_count,
      mismatch_action_threshold: form.mismatch_action_threshold,
      failure_disable: form.failure_disable,
      failure_temp_unschedulable_minutes: form.failure_temp_unschedulable_minutes
    })
    appStore.showSuccess(t('admin.accounts.fingerprint.created'))
    form.test_model = ''
    form.expected_model = ''
    await loadPlans()
  } catch (error) {
    appStore.showError(String(error))
  } finally {
    creating.value = false
  }
}

async function runNow(plan: FingerprintPlan) {
  runningPlanId.value = plan.id
  try {
    await adminAPI.fingerprint.runNow(plan.id)
    appStore.showSuccess(t('admin.accounts.fingerprint.runStarted'))
  } catch (error) {
    appStore.showError(String(error))
  } finally {
    runningPlanId.value = null
  }
}

async function remove(plan: FingerprintPlan) {
  try {
    await adminAPI.fingerprint.delete(plan.id)
    await loadPlans()
  } catch (error) {
    appStore.showError(String(error))
  }
}

onMounted(async () => {
  await Promise.all([loadOverview(), loadModels(), loadPlans()])
})
</script>
