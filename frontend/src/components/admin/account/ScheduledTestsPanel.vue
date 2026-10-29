<template>
  <BaseDialog
    :show="show"
    :title="t('admin.scheduledTests.title')"
    width="wide"
    @close="emit('close')"
  >
    <div class="space-y-4">
      <!-- Add Plan Button -->
      <div class="flex items-center justify-between">
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.scheduledTests.title') }}
        </p>
        <button
          @click="showAddForm = !showAddForm"
          class="btn btn-primary flex items-center gap-1.5 text-sm"
        >
          <Icon name="plus" size="sm" :stroke-width="2" />
          {{ t('admin.scheduledTests.addPlan') }}
        </button>
      </div>

      <!-- Add Plan Form -->
      <div
        v-if="showAddForm"
        class="rounded-xl border border-primary-200 bg-primary-50/50 p-4 dark:border-primary-800 dark:bg-primary-900/20"
      >
        <div class="mb-3 text-sm font-medium text-gray-700 dark:text-gray-300">
          {{ t('admin.scheduledTests.addPlan') }}
        </div>
        <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.model') }}
            </label>
            <Select
              v-model="newPlan.model_id"
              :options="modelOptions"
              :placeholder="t('admin.scheduledTests.model')"
              :searchable="modelOptions.length > 5"
            />
          </div>
          <div>
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">推理等级</label>
            <Select
              v-model="newPlan.reasoning_effort"
              :options="[{ value: '', label: '模型默认' }, { value: 'low', label: '低 (low)' }, { value: 'medium', label: '中 (medium)' }, { value: 'high', label: '高 (high)' }, { value: 'xhigh', label: '极高 (xhigh)' }, { value: 'max', label: '最大 (max)' }]"
            />
          </div>
          <div>
            <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.cronExpression') }}
              <HelpTooltip>
                <template #trigger>
                  <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                    ?
                  </span>
                </template>
                <div class="space-y-1.5">
                  <p class="font-medium">{{ t('admin.scheduledTests.cronTooltipTitle') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipMeaning') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleEvery30Min') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleHourly') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleDaily') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipExampleWeekly') }}</p>
                  <p>{{ t('admin.scheduledTests.cronTooltipRange') }}</p>
                </div>
              </HelpTooltip>
            </label>
            <Input
              v-model="newPlan.cron_expression"
              :placeholder="'*/30 * * * *'"
              :hint="t('admin.scheduledTests.cronHelp')"
            />
          </div>
          <div>
            <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.maxResults') }}
              <HelpTooltip>
                <template #trigger>
                  <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                    ?
                  </span>
                </template>
                <div class="space-y-1.5">
                  <p class="font-medium">{{ t('admin.scheduledTests.maxResultsTooltipTitle') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipMeaning') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipBody') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipExample') }}</p>
                  <p>{{ t('admin.scheduledTests.maxResultsTooltipRange') }}</p>
                </div>
              </HelpTooltip>
            </label>
            <Input
              v-model="newPlan.max_results"
              type="number"
              placeholder="100"
            />
          </div>
          <div class="sm:col-span-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">定时提示词（可选）</label>
            <textarea v-model="newPlan.prompt_text" rows="2" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800" placeholder="要求模型只输出数字或文本"></textarea>
            <Input v-model="newPlan.expected_answer" class="mt-2" label="期望答案" placeholder="与模型输出完全一致" />
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div class="sm:col-span-2"><div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-400">失败：移除账号当前绑定的分组</div><div v-if="boundGroups.length" class="flex flex-wrap gap-2"><label v-for="group in boundGroups" :key="group.id" class="flex items-center gap-1.5 text-sm"><input v-model="newPlan.failure_remove_group_ids" type="checkbox" :value="Number(group.id)" class="rounded border-gray-300" />{{ group.name }}</label></div><span v-else class="text-xs text-gray-400">该账号当前没有绑定分组</span></div>
              <Input v-model="newPlan.failure_temp_unschedulable_minutes" type="number" label="失败：临时不可调度（分钟，0=不操作）" />
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="newPlan.failure_disable" />失败：关闭账号</label>
              <Select v-model="newPlan.success_add_group_id" :options="[{ value: '', label: '成功：不加入分组' }, ...(groups || []).map(g => ({ value: String(g.id), label: '成功：加入 ' + g.name }))]" />
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="newPlan.success_enable" />成功：开启账号</label>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="newPlan.success_recover" />成功：恢复调度与错误状态</label>
            </div>
          </div>
          <div class="flex items-end">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="newPlan.enabled" />
              {{ t('admin.scheduledTests.enabled') }}
            </label>
          </div>
          <div class="flex items-end">
            <div>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                <Toggle v-model="newPlan.auto_recover" />
                {{ t('admin.scheduledTests.autoRecover') }}
              </label>
              <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                {{ t('admin.scheduledTests.autoRecoverHelp') }}
              </p>
            </div>
          </div>
          <div class="sm:col-span-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
              <Toggle v-model="newPlan.temp_unschedulable_enabled" />
              {{ t('admin.scheduledTests.tempUnschedulable') }}
            </label>
            <p class="mt-1 text-xs text-gray-400">{{ t('admin.scheduledTests.tempUnschedulableHelp') }}</p>
            <div v-if="newPlan.temp_unschedulable_enabled" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <Input v-model="newPlan.error_codes" :label="t('admin.scheduledTests.errorCodes')" :placeholder="'401, 429, 503'" />
              <Input v-model="newPlan.minutes" type="number" :label="t('admin.scheduledTests.pauseMinutes')" />
            </div>
          </div>
        </div>
        <div class="mt-3 flex justify-end gap-2">
          <button
            @click="showAddForm = false; resetNewPlan()"
            class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
          >
            {{ t('common.cancel') }}
          </button>
          <button
            @click="handleCreate"
            :disabled="!newPlan.model_id || !newPlan.cron_expression || creating"
            class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Icon v-if="creating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
            {{ t('common.save') }}
          </button>
        </div>
      </div>

      <!-- Loading State -->
      <div v-if="loading" class="flex items-center justify-center py-8">
        <Icon name="refresh" size="md" class="animate-spin text-gray-400" :stroke-width="2" />
        <span class="ml-2 text-sm text-gray-500">{{ t('common.loading') }}...</span>
      </div>

      <!-- Empty State -->
      <div
        v-else-if="plans.length === 0"
        class="rounded-xl border border-dashed border-gray-300 py-10 text-center dark:border-dark-600"
      >
        <Icon name="calendar" size="lg" class="mx-auto mb-2 text-gray-400" :stroke-width="1.5" />
        <p class="text-sm text-gray-500 dark:text-gray-400">
          {{ t('admin.scheduledTests.noPlans') }}
        </p>
      </div>

      <!-- Plans List -->
      <div v-else class="space-y-3">
        <div
          v-for="plan in plans"
          :key="plan.id"
          class="rounded-xl border border-gray-200 bg-white transition-all dark:border-dark-600 dark:bg-dark-800"
        >
          <!-- Plan Header -->
          <div
            class="flex cursor-pointer items-center justify-between px-4 py-3"
            @click="toggleExpand(plan.id)"
          >
            <div class="flex flex-1 items-center gap-4">
              <!-- Model -->
              <div class="min-w-0">
                <div class="text-sm font-medium text-gray-900 dark:text-gray-100">
                  {{ plan.model_id }}
                </div>
                <div class="mt-0.5 font-mono text-xs text-gray-500 dark:text-gray-400">
                  {{ plan.cron_expression }}
                </div>
              </div>

              <!-- Enabled Toggle -->
              <div class="flex items-center gap-1.5" @click.stop>
                <Toggle
                  :model-value="plan.enabled"
                  @update:model-value="(val: boolean) => handleToggleEnabled(plan, val)"
                />
                <span class="text-xs text-gray-500 dark:text-gray-400">
                  {{ plan.enabled ? t('admin.scheduledTests.enabled') : '' }}
                </span>
              </div>

              <!-- Auto Recover Badge -->
              <span
                v-if="plan.auto_recover"
                class="inline-flex items-center rounded-full bg-emerald-100 px-2 py-0.5 text-xs font-medium text-emerald-700 dark:bg-emerald-500/20 dark:text-emerald-400"
              >
                {{ t('admin.scheduledTests.autoRecover') }}
              </span>
            </div>

            <div class="flex items-center gap-3">
              <!-- Last Run -->
              <div v-if="plan.last_run_at" class="hidden text-right text-xs text-gray-500 dark:text-gray-400 sm:block">
                <div>{{ t('admin.scheduledTests.lastRun') }}</div>
                <div>{{ formatDateTime(plan.last_run_at) }}</div>
              </div>

              <!-- Next Run -->
              <div v-if="plan.next_run_at" class="hidden text-right text-xs text-gray-500 dark:text-gray-400 sm:block">
                <div>{{ t('admin.scheduledTests.nextRun') }}</div>
                <div>{{ formatDateTime(plan.next_run_at) }}</div>
              </div>

              <!-- Actions -->
              <div class="flex items-center gap-1" @click.stop>
                <button
                  @click="startEdit(plan)"
                  class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-blue-50 hover:text-blue-500 dark:hover:bg-blue-900/20"
                  :title="t('admin.scheduledTests.editPlan')"
                >
                  <Icon name="edit" size="sm" :stroke-width="2" />
                </button>
                <button
                  @click="confirmDeletePlan(plan)"
                  class="rounded-lg p-1.5 text-gray-400 transition-colors hover:bg-red-50 hover:text-red-500 dark:hover:bg-red-900/20"
                  :title="t('admin.scheduledTests.deletePlan')"
                >
                  <Icon name="trash" size="sm" :stroke-width="2" />
                </button>
              </div>

              <!-- Expand indicator -->
              <Icon
                name="chevronDown"
                size="sm"
                :class="[
                  'text-gray-400 transition-transform duration-200',
                  expandedPlanId === plan.id ? 'rotate-180' : ''
                ]"
              />
            </div>
          </div>

          <!-- Edit Form -->
          <div
            v-if="editingPlanId === plan.id"
            class="border-t border-blue-100 bg-blue-50/50 px-4 py-3 dark:border-blue-900 dark:bg-blue-900/10"
            @click.stop
          >
            <div class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.editPlan') }}
            </div>
            <div class="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div>
                <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.model') }}
                </label>
                <Select
                  v-model="editForm.model_id"
                  :options="modelOptions"
                  :placeholder="t('admin.scheduledTests.model')"
                  :searchable="modelOptions.length > 5"
                />
              </div>
              <div>
                <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">推理等级</label>
                <Select
                  v-model="editForm.reasoning_effort"
                  :options="[{ value: '', label: '模型默认' }, { value: 'low', label: '低 (low)' }, { value: 'medium', label: '中 (medium)' }, { value: 'high', label: '高 (high)' }, { value: 'xhigh', label: '极高 (xhigh)' }, { value: 'max', label: '最大 (max)' }]"
                />
              </div>
              <div>
                <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.cronExpression') }}
                  <HelpTooltip>
                    <template #trigger>
                      <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                        ?
                      </span>
                    </template>
                    <div class="space-y-1.5">
                      <p class="font-medium">{{ t('admin.scheduledTests.cronTooltipTitle') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipMeaning') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleEvery30Min') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleHourly') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleDaily') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipExampleWeekly') }}</p>
                      <p>{{ t('admin.scheduledTests.cronTooltipRange') }}</p>
                    </div>
                  </HelpTooltip>
                </label>
                <Input
                  v-model="editForm.cron_expression"
                  :placeholder="'*/30 * * * *'"
                  :hint="t('admin.scheduledTests.cronHelp')"
                />
              </div>
              <div>
                <label class="mb-1 flex items-center gap-1 text-xs font-medium text-gray-600 dark:text-gray-400">
                  {{ t('admin.scheduledTests.maxResults') }}
                  <HelpTooltip>
                    <template #trigger>
                      <span class="inline-flex h-4 w-4 cursor-help items-center justify-center rounded-full border border-gray-400/70 text-[10px] font-semibold text-gray-400 transition-colors hover:border-primary-500 hover:text-primary-600 dark:border-gray-500 dark:text-gray-500 dark:hover:border-primary-400 dark:hover:text-primary-400">
                        ?
                      </span>
                    </template>
                    <div class="space-y-1.5">
                      <p class="font-medium">{{ t('admin.scheduledTests.maxResultsTooltipTitle') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipMeaning') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipBody') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipExample') }}</p>
                      <p>{{ t('admin.scheduledTests.maxResultsTooltipRange') }}</p>
                    </div>
                  </HelpTooltip>
                </label>
                <Input
                  v-model="editForm.max_results"
                  type="number"
                  placeholder="100"
                />
              </div>
              <div class="sm:col-span-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
            <label class="mb-1 block text-xs font-medium text-gray-600 dark:text-gray-400">定时提示词（可选）</label>
            <textarea v-model="editForm.prompt_text" rows="2" class="w-full rounded-lg border border-gray-300 bg-white px-3 py-2 text-sm dark:border-dark-600 dark:bg-dark-800" placeholder="要求模型只输出数字或文本"></textarea>
            <Input v-model="editForm.expected_answer" class="mt-2" label="期望答案" placeholder="与模型输出完全一致" />
            <div class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
              <div class="sm:col-span-2"><div class="mb-1 text-xs font-medium text-gray-600 dark:text-gray-400">失败：移除账号当前绑定的分组</div><div v-if="boundGroups.length" class="flex flex-wrap gap-2"><label v-for="group in boundGroups" :key="group.id" class="flex items-center gap-1.5 text-sm"><input v-model="editForm.failure_remove_group_ids" type="checkbox" :value="Number(group.id)" class="rounded border-gray-300" />{{ group.name }}</label></div><span v-else class="text-xs text-gray-400">该账号当前没有绑定分组</span></div>
              <Input v-model="editForm.failure_temp_unschedulable_minutes" type="number" label="失败：临时不可调度（分钟，0=不操作）" />
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="editForm.failure_disable" />失败：关闭账号</label>
              <Select v-model="editForm.success_add_group_id" :options="[{ value: '', label: '成功：不加入分组' }, ...(groups || []).map(g => ({ value: String(g.id), label: '成功：加入 ' + g.name }))]" />
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="editForm.success_enable" />成功：开启账号</label>
              <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300"><Toggle v-model="editForm.success_recover" />成功：恢复调度与错误状态</label>
            </div>
          </div>
          <div class="flex items-end">
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <Toggle v-model="editForm.enabled" />
                  {{ t('admin.scheduledTests.enabled') }}
                </label>
              </div>
              <div class="flex items-end">
                <div>
                  <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                    <Toggle v-model="editForm.auto_recover" />
                    {{ t('admin.scheduledTests.autoRecover') }}
                  </label>
                  <p class="mt-0.5 text-xs text-gray-400 dark:text-gray-500">
                    {{ t('admin.scheduledTests.autoRecoverHelp') }}
                  </p>
                </div>
              </div>
              <div class="sm:col-span-2 rounded-lg border border-gray-200 p-3 dark:border-dark-600">
                <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-gray-300">
                  <Toggle v-model="editForm.temp_unschedulable_enabled" />
                  {{ t('admin.scheduledTests.tempUnschedulable') }}
                </label>
                <p class="mt-1 text-xs text-gray-400">{{ t('admin.scheduledTests.tempUnschedulableHelp') }}</p>
                <div v-if="editForm.temp_unschedulable_enabled" class="mt-3 grid grid-cols-1 gap-3 sm:grid-cols-2">
                  <Input v-model="editForm.error_codes" :label="t('admin.scheduledTests.errorCodes')" :placeholder="'401, 429, 503'" />
                  <Input v-model="editForm.minutes" type="number" :label="t('admin.scheduledTests.pauseMinutes')" />
                </div>
              </div>
            </div>
            <div class="mt-3 flex justify-end gap-2">
              <button
                @click="cancelEdit"
                class="rounded-lg bg-gray-100 px-3 py-1.5 text-sm font-medium text-gray-700 transition-colors hover:bg-gray-200 dark:bg-dark-600 dark:text-gray-300 dark:hover:bg-dark-500"
              >
                {{ t('common.cancel') }}
              </button>
              <button
                @click="handleEdit"
                :disabled="!editForm.model_id || !editForm.cron_expression || updating"
                class="flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-sm font-medium text-white transition-colors hover:bg-primary-600 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <Icon v-if="updating" name="refresh" size="sm" class="animate-spin" :stroke-width="2" />
                {{ t('common.save') }}
              </button>
            </div>
          </div>

          <!-- Expanded Results Section -->
          <div
            v-if="expandedPlanId === plan.id"
            class="border-t border-gray-100 px-4 py-3 dark:border-dark-700"
          >
            <div class="mb-2 text-xs font-medium text-gray-600 dark:text-gray-400">
              {{ t('admin.scheduledTests.results') }}
            </div>
            <button @click="runNow(plan)" :disabled="runningPlanId === plan.id" class="mb-3 flex items-center gap-1.5 rounded-lg bg-primary-500 px-3 py-1.5 text-xs font-medium text-white disabled:opacity-50"><Icon :name="runningPlanId === plan.id ? 'refresh' : 'play'" size="sm" :class="runningPlanId === plan.id ? 'animate-spin' : ''" />{{ runningPlanId === plan.id ? '测试中...' : '开始测试' }}</button>

            <!-- Results Loading -->
            <div v-if="loadingResults" class="flex items-center justify-center py-4">
              <Icon name="refresh" size="sm" class="animate-spin text-gray-400" :stroke-width="2" />
              <span class="ml-2 text-xs text-gray-500">{{ t('common.loading') }}...</span>
            </div>

            <!-- No Results -->
            <div
              v-else-if="results.length === 0"
              class="py-4 text-center text-xs text-gray-500 dark:text-gray-400"
            >
              {{ t('admin.scheduledTests.noResults') }}
            </div>

            <!-- Results List -->
            <div v-else class="max-h-64 space-y-2 overflow-y-auto">
              <div
                v-for="result in results"
                :key="result.id"
                class="rounded-lg border border-gray-100 bg-gray-50 p-3 dark:border-dark-700 dark:bg-dark-900"
              >
                <div class="flex items-center justify-between">
                  <div class="flex items-center gap-2">
                    <!-- Status Badge -->
                    <span
                      :class="[
                        'inline-flex items-center rounded-full px-2 py-0.5 text-xs font-medium',
                        result.status === 'success'
                          ? 'bg-green-100 text-green-700 dark:bg-green-500/20 dark:text-green-400'
                          : result.status === 'running'
                            ? 'bg-blue-100 text-blue-700 dark:bg-blue-500/20 dark:text-blue-400'
                            : 'bg-red-100 text-red-700 dark:bg-red-500/20 dark:text-red-400'
                      ]"
                    >
                      {{
                        result.status === 'success'
                          ? t('admin.scheduledTests.success')
                          : result.status === 'running'
                            ? t('admin.scheduledTests.running')
                            : t('admin.scheduledTests.failed')
                      }}
                    </span>

                    <!-- Latency -->
                    <span v-if="result.latency_ms > 0" class="text-xs text-gray-500 dark:text-gray-400">
                      {{ result.latency_ms }}ms
                    </span>
                  </div>

                  <!-- Started At -->
                  <span class="text-xs text-gray-400">
                    {{ formatDateTime(result.started_at) }}
                  </span>
                </div>

                <!-- Response / Error (collapsible) -->
                <div v-if="result.error_message" class="mt-2">
                  <div
                    class="cursor-pointer text-xs font-medium text-red-600 dark:text-red-400"
                    @click="toggleResultDetail(result.id)"
                  >
                    {{ t('admin.scheduledTests.errorMessage') }}
                    <Icon
                      name="chevronDown"
                      size="sm"
                      :class="[
                        'inline transition-transform duration-200',
                        expandedResultIds.has(result.id) ? 'rotate-180' : ''
                      ]"
                    />
                  </div>
                  <pre
                    v-if="expandedResultIds.has(result.id)"
                    class="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded bg-red-50 p-2 text-xs text-red-700 dark:bg-red-900/20 dark:text-red-300"
                  >{{ result.error_message }}</pre>
                </div>
                <div v-else-if="result.response_text" class="mt-2">
                  <div
                    class="cursor-pointer text-xs font-medium text-gray-600 dark:text-gray-400"
                    @click="toggleResultDetail(result.id)"
                  >
                    {{ t('admin.scheduledTests.responseText') }}
                    <Icon
                      name="chevronDown"
                      size="sm"
                      :class="[
                        'inline transition-transform duration-200',
                        expandedResultIds.has(result.id) ? 'rotate-180' : ''
                      ]"
                    />
                  </div>
                  <pre
                    v-if="expandedResultIds.has(result.id)"
                    class="mt-1 max-h-32 overflow-auto whitespace-pre-wrap rounded bg-gray-100 p-2 text-xs text-gray-700 dark:bg-dark-800 dark:text-gray-300"
                  >{{ result.response_text }}</pre>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- Delete Confirmation -->
    <ConfirmDialog
      :show="showDeleteConfirm"
      :title="t('admin.scheduledTests.deletePlan')"
      :message="t('admin.scheduledTests.confirmDelete')"
      :confirm-text="t('common.delete')"
      :cancel-text="t('common.cancel')"
      :danger="true"
      @confirm="handleDelete"
      @cancel="showDeleteConfirm = false"
    />
  </BaseDialog>
</template>

<script setup lang="ts">
import { ref, reactive, watch, onBeforeUnmount, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import BaseDialog from '@/components/common/BaseDialog.vue'
import ConfirmDialog from '@/components/common/ConfirmDialog.vue'
import HelpTooltip from '@/components/common/HelpTooltip.vue'
import Select, { type SelectOption } from '@/components/common/Select.vue'
import Input from '@/components/common/Input.vue'
import Toggle from '@/components/common/Toggle.vue'
import { Icon } from '@/components/icons'
import { adminAPI } from '@/api/admin'
import { useAppStore } from '@/stores/app'
import { formatDateTime } from '@/utils/format'
import type { ScheduledTestPlan, ScheduledTestResult, AdminGroup } from '@/types'

const { t } = useI18n()
const appStore = useAppStore()

const props = defineProps<{
  show: boolean
  accountId: number | null
  modelOptions: SelectOption[]
  groups?: AdminGroup[]
  accountGroupIds?: number[]
}>()

const emit = defineEmits<{
  (e: 'close'): void
}>()

// State
const loading = ref(false)
const creating = ref(false)
const loadingResults = ref(false)
const plans = ref<ScheduledTestPlan[]>([])
const results = ref<ScheduledTestResult[]>([])
const expandedPlanId = ref<number | null>(null)
let resultsRequestId = 0
watch(expandedPlanId, () => {
  resultsRequestId++
  results.value = []
  loadingResults.value = false
}, { flush: 'sync' })
onBeforeUnmount(() => { resultsRequestId++ })
const expandedResultIds = reactive(new Set<number>())
const showAddForm = ref(false)
const showDeleteConfirm = ref(false)
const deletingPlan = ref<ScheduledTestPlan | null>(null)
const editingPlanId = ref<number | null>(null)
const updating = ref(false)
const runningPlanId = ref<number | null>(null)
const boundGroups = computed(() => (props.groups || []).filter((group) => (props.accountGroupIds || []).includes(Number(group.id))))
const editForm = reactive({
  prompt_text: '',
  expected_answer: '',
  failure_remove_group_id: '',
  failure_remove_group_ids: [] as number[],
  failure_disable: false,
  failure_temp_unschedulable_minutes: '0',
  success_add_group_id: '',
  success_enable: false,
  success_recover: false,
  model_id: '' as string,
  reasoning_effort: '',
  cron_expression: '' as string,
  max_results: '100' as string,
  enabled: true,
  auto_recover: false,
  temp_unschedulable_enabled: false,
  error_codes: '',
  minutes: '5'
})

const newPlan = reactive({
  prompt_text: '',
  expected_answer: '',
  failure_remove_group_id: '',
  failure_remove_group_ids: [] as number[],
  failure_disable: false,
  failure_temp_unschedulable_minutes: '0',
  success_add_group_id: '',
  success_enable: false,
  success_recover: false,
  model_id: '' as string,
  reasoning_effort: '',
  cron_expression: '' as string,
  max_results: '100' as string,
  enabled: true,
  auto_recover: false,
  temp_unschedulable_enabled: false,
  error_codes: '',
  minutes: '5'
})

const resetNewPlan = () => {
  newPlan.prompt_text = ''
  newPlan.expected_answer = ''
  newPlan.failure_remove_group_id = ''
  newPlan.failure_remove_group_ids = []
  newPlan.failure_disable = false
  newPlan.failure_temp_unschedulable_minutes = '0'
  newPlan.success_add_group_id = ''
  newPlan.success_enable = false
  newPlan.success_recover = false
  newPlan.model_id = ''
  newPlan.reasoning_effort = ''
  newPlan.cron_expression = ''
  newPlan.max_results = '100'
  newPlan.enabled = true
  newPlan.auto_recover = false
  newPlan.temp_unschedulable_enabled = false
  newPlan.error_codes = ''
  newPlan.minutes = '5'
}

const parseErrorCodes = (value: string) => [...new Set(value.split(',').map((part) => Number(part.trim())).filter((code) => Number.isInteger(code) && code >= 100 && code <= 599))]

// Load plans when dialog opens
watch(
  () => props.show,
  async (visible) => {
    if (visible && props.accountId) {
      await loadPlans()
    } else {
      plans.value = []
      results.value = []
      expandedPlanId.value = null
      expandedResultIds.clear()
      showAddForm.value = false
      showDeleteConfirm.value = false
    }
  }
)

const loadPlans = async () => {
  if (!props.accountId) return
  loading.value = true
  try {
    plans.value = await adminAPI.scheduledTests.listByAccount(props.accountId)
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to load plans')
  } finally {
    loading.value = false
  }
}

const handleCreate = async () => {
  if (!props.accountId || !newPlan.model_id || !newPlan.cron_expression) return
  creating.value = true
  try {
    const maxResults = Number(newPlan.max_results) || 100
    await adminAPI.scheduledTests.create({
      account_id: props.accountId,
      model_id: newPlan.model_id,
      reasoning_effort: newPlan.reasoning_effort,
      prompt_text: newPlan.prompt_text,
      expected_answer: newPlan.expected_answer,
      failure_remove_group_ids: newPlan.failure_remove_group_ids,
      failure_disable: newPlan.failure_disable,
      failure_temp_unschedulable_minutes: Number(newPlan.failure_temp_unschedulable_minutes) || 0,
      success_add_group_id: newPlan.success_add_group_id ? Number(newPlan.success_add_group_id) : null,
      success_enable: newPlan.success_enable,
      success_recover: newPlan.success_recover,
      cron_expression: newPlan.cron_expression,
      enabled: newPlan.enabled,
      max_results: maxResults,
      auto_recover: newPlan.auto_recover,
      temp_unschedulable_enabled: newPlan.temp_unschedulable_enabled,
      temp_unschedulable_error_codes: parseErrorCodes(newPlan.error_codes),
      temp_unschedulable_minutes: Number(newPlan.minutes) || 5
    })
    appStore.showSuccess(t('admin.scheduledTests.createSuccess'))
    showAddForm.value = false
    resetNewPlan()
    await loadPlans()
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to create plan')
  } finally {
    creating.value = false
  }
}

const handleToggleEnabled = async (plan: ScheduledTestPlan, enabled: boolean) => {
  try {
    const updated = await adminAPI.scheduledTests.update(plan.id, { enabled })
    const index = plans.value.findIndex((p) => p.id === plan.id)
    if (index !== -1) {
      plans.value[index] = updated
    }
    appStore.showSuccess(t('admin.scheduledTests.updateSuccess'))
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to update plan')
  }
}

const startEdit = (plan: ScheduledTestPlan) => {
  editingPlanId.value = plan.id
  editForm.prompt_text = plan.prompt_text || ''
  editForm.expected_answer = plan.expected_answer || ''
  editForm.failure_remove_group_ids = plan.failure_remove_group_ids?.length ? [...plan.failure_remove_group_ids] : (plan.failure_remove_group_id ? [plan.failure_remove_group_id] : [])
  editForm.failure_disable = plan.failure_disable === true
  editForm.failure_temp_unschedulable_minutes = String(plan.failure_temp_unschedulable_minutes || 0)
  editForm.success_add_group_id = plan.success_add_group_id ? String(plan.success_add_group_id) : ''
  editForm.success_enable = plan.success_enable === true
  editForm.success_recover = plan.success_recover === true
  editForm.model_id = plan.model_id
  editForm.reasoning_effort = plan.reasoning_effort || ''
  editForm.cron_expression = plan.cron_expression
  editForm.max_results = String(plan.max_results)
  editForm.enabled = plan.enabled
  editForm.auto_recover = plan.auto_recover
  editForm.temp_unschedulable_enabled = plan.temp_unschedulable_enabled === true
  editForm.error_codes = (plan.temp_unschedulable_error_codes || []).join(', ')
  editForm.minutes = String(plan.temp_unschedulable_minutes || 5)
}

const cancelEdit = () => {
  editingPlanId.value = null
}

const handleEdit = async () => {
  if (!editingPlanId.value || !editForm.model_id || !editForm.cron_expression) return
  updating.value = true
  try {
    const updated = await adminAPI.scheduledTests.update(editingPlanId.value, {
      model_id: editForm.model_id,
      reasoning_effort: editForm.reasoning_effort,
      prompt_text: editForm.prompt_text,
      expected_answer: editForm.expected_answer,
      failure_remove_group_ids: editForm.failure_remove_group_ids,
      failure_disable: editForm.failure_disable,
      failure_temp_unschedulable_minutes: Number(editForm.failure_temp_unschedulable_minutes) || 0,
      success_add_group_id: editForm.success_add_group_id ? Number(editForm.success_add_group_id) : 0,
      success_enable: editForm.success_enable,
      success_recover: editForm.success_recover,
      cron_expression: editForm.cron_expression,
      max_results: Number(editForm.max_results) || 100,
      enabled: editForm.enabled,
      auto_recover: editForm.auto_recover,
      temp_unschedulable_enabled: editForm.temp_unschedulable_enabled,
      temp_unschedulable_error_codes: parseErrorCodes(editForm.error_codes),
      temp_unschedulable_minutes: Number(editForm.minutes) || 5
    })
    const index = plans.value.findIndex((p) => p.id === editingPlanId.value)
    if (index !== -1) {
      plans.value[index] = updated
    }
    appStore.showSuccess(t('admin.scheduledTests.updateSuccess'))
    editingPlanId.value = null
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to update plan')
  } finally {
    updating.value = false
  }
}

const confirmDeletePlan = (plan: ScheduledTestPlan) => {
  deletingPlan.value = plan
  showDeleteConfirm.value = true
}

const handleDelete = async () => {
  if (!deletingPlan.value) return
  try {
    await adminAPI.scheduledTests.delete(deletingPlan.value.id)
    appStore.showSuccess(t('admin.scheduledTests.deleteSuccess'))
    plans.value = plans.value.filter((p) => p.id !== deletingPlan.value!.id)
    if (expandedPlanId.value === deletingPlan.value.id) {
      expandedPlanId.value = null
      results.value = []
    }
  } catch (error: any) {
    appStore.showError(error?.message || 'Failed to delete plan')
  } finally {
    showDeleteConfirm.value = false
    deletingPlan.value = null
  }
}

const toggleExpand = async (planId: number) => {
  if (expandedPlanId.value === planId) {
    expandedPlanId.value = null
    results.value = []
    expandedResultIds.clear()
    return
  }

  expandedPlanId.value = planId
  expandedResultIds.clear()
  loadingResults.value = true
  const requestId = resultsRequestId
  try {
    const data = await adminAPI.scheduledTests.listResults(planId, 20)
    if (requestId !== resultsRequestId) return
    results.value = data
  } catch (error: any) {
    if (requestId !== resultsRequestId) return
    appStore.showError(error?.message || 'Failed to load results')
    results.value = []
  } finally {
    if (requestId === resultsRequestId) loadingResults.value = false
  }
}

const toggleResultDetail = (resultId: number) => {
  if (expandedResultIds.has(resultId)) {
    expandedResultIds.delete(resultId)
  } else {
    expandedResultIds.add(resultId)
  }
}

const runNow = async (plan: ScheduledTestPlan) => {
  runningPlanId.value = plan.id
  expandedPlanId.value = plan.id
  const startedAfter = Date.now() - 1000
  try {
    await adminAPI.scheduledTests.runNow(plan.id)
    const deadline = Date.now() + 5 * 60 * 1000
    while (Date.now() < deadline) {
      await new Promise((resolve) => window.setTimeout(resolve, 2000))
      const latest = await adminAPI.scheduledTests.listResults(plan.id, 20)
      const current = latest.find((item) => new Date(item.started_at).getTime() >= startedAfter)
      results.value = latest
      if (current) {
        expandedResultIds.clear()
        if (current.response_text || current.error_message) expandedResultIds.add(current.id)
        return
      }
    }
    appStore.showError('测试仍在后台运行，稍后刷新结果查看')
  } catch (error: any) {
    appStore.showError(error?.message || '测试失败')
  } finally {
    runningPlanId.value = null
  }
}
</script>
