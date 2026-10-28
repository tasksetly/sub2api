<template>
  <AppLayout>
    <div class="mx-auto max-w-7xl space-y-6 p-4 sm:p-6">
      <div class="flex flex-col justify-between gap-4 sm:flex-row sm:items-end">
        <div>
          <p class="text-xs font-semibold uppercase tracking-wider text-primary-600">SVG Benchmark</p>
          <h1 class="mt-1 text-2xl font-bold text-gray-900 dark:text-white">SVG 能力测试</h1>
          <p class="mt-2 max-w-2xl text-sm text-gray-500 dark:text-dark-400">绑定 API Key 后，系统会自动使用该 Key 所属分组的可用账号执行提示词，并把最近结果展示到公开测试页。</p>
        </div>
        <div class="flex gap-2">
          <a href="/pelican-test" target="_blank" rel="noopener" class="btn btn-secondary">
            <Icon name="externalLink" size="sm" class="mr-2" />打开展示页
          </a>
          <button class="btn btn-primary" @click="openCreate">
            <Icon name="plus" size="sm" class="mr-2" />新建测试
          </button>
        </div>
      </div>

      <div v-if="error" class="rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700 dark:border-red-900/50 dark:bg-red-950/30 dark:text-red-300">{{ error }}</div>

      <div class="overflow-hidden rounded-xl border border-gray-200 bg-white shadow-sm dark:border-dark-700 dark:bg-dark-900">
        <div class="flex items-center justify-between border-b border-gray-200 px-4 py-3 dark:border-dark-700">
          <div><h2 class="font-semibold text-gray-900 dark:text-white">已配置测试</h2><p class="text-xs text-gray-500 dark:text-dark-400">每个配置最多保留 {{ configs[0]?.keep_count || 20 }} 条结果</p></div>
          <button class="btn btn-secondary" :disabled="loading" title="刷新" @click="load"><Icon name="refresh" size="sm" :class="loading ? 'animate-spin' : ''" /></button>
        </div>
        <div v-if="loading" class="p-10 text-center text-sm text-gray-500">加载中...</div>
        <div v-else-if="!configs.length" class="p-10 text-center text-sm text-gray-500">还没有测试配置。先绑定一个 API Key 分组并填写提示词。</div>
        <div v-else class="divide-y divide-gray-200 dark:divide-dark-700">
          <div v-for="item in configs" :key="item.id" class="flex flex-col gap-4 p-4 lg:flex-row lg:items-center lg:justify-between">
            <div class="min-w-0">
              <div class="flex flex-wrap items-center gap-2"><h3 class="font-semibold text-gray-900 dark:text-white">{{ item.name }}</h3><span class="rounded-full px-2 py-0.5 text-xs" :class="item.enabled ? 'bg-emerald-100 text-emerald-700' : 'bg-gray-100 text-gray-500'">{{ item.enabled ? '运行中' : '已停用' }}</span></div>
              <p class="mt-1 text-sm text-gray-500 dark:text-dark-400">{{ item.group_name || '未绑定分组' }} · {{ item.model_id }} · {{ item.cron_expression }}</p>
              <p class="mt-2 line-clamp-2 max-w-3xl text-xs text-gray-400">{{ item.prompt_text }}</p>
            </div>
            <div class="flex shrink-0 items-center gap-2">
              <button class="btn btn-secondary" :disabled="runningId === item.id" title="立即运行" @click="run(item)"><Icon name="play" size="sm" class="mr-1" />{{ runningId === item.id ? '运行中' : '立即运行' }}</button>
              <button class="btn btn-secondary" title="编辑" @click="openEdit(item)"><Icon name="edit" size="sm" class="mr-1" />编辑</button>
              <button class="btn btn-secondary text-red-600" title="删除" @click="remove(item)"><Icon name="trash" size="sm" /></button>
            </div>
          </div>
        </div>
      </div>

      <BaseDialog :show="dialogOpen" :title="editing ? '编辑 SVG 测试' : '新建 SVG 测试'" @close="dialogOpen = false">
        <form class="space-y-4" @submit.prevent="save">
          <div><label class="input-label">测试名称</label><input v-model="form.name" class="input" required placeholder="例如：鹈鹕 SVG 测试" /></div>
          <div class="grid gap-4 sm:grid-cols-2"><div><label class="input-label">模型 ID</label><input v-model="form.model_id" class="input" required placeholder="例如：gpt-4o" /></div><div><label class="input-label">API Key（自动绑定分组）</label><select v-model.number="form.api_key_id" class="input" required><option :value="0" disabled>选择 API Key</option><option v-for="key in apiKeys" :key="key.id" :value="key.id">{{ key.group_name }} · {{ key.name }} ({{ key.key_prefix }}...)</option></select></div></div>
          <div><label class="input-label">提示词</label><textarea v-model="form.prompt_text" class="input min-h-32" required placeholder="请输出一个完整的 SVG，不要附加 Markdown 代码块。" /></div>
          <div class="grid gap-4 sm:grid-cols-2"><div><label class="input-label">Cron（默认每小时）</label><input v-model="form.cron_expression" class="input" placeholder="0 * * * *" /></div><div><label class="input-label">保留结果数</label><input v-model.number="form.keep_count" type="number" min="1" max="100" class="input" /></div></div>
          <label class="flex items-center gap-2 text-sm text-gray-700 dark:text-dark-300"><input v-model="form.enabled" type="checkbox" class="h-4 w-4 rounded border-gray-300 text-primary-600" />启用定时测试</label>
          <div class="flex justify-end gap-2"><button type="button" class="btn btn-secondary" @click="dialogOpen = false">取消</button><button type="submit" class="btn btn-primary" :disabled="saving">{{ saving ? '保存中...' : '保存配置' }}</button></div>
        </form>
      </BaseDialog>
    </div>
  </AppLayout>
</template>

<script setup lang="ts">
import { onMounted, reactive, ref } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import BaseDialog from '@/components/common/BaseDialog.vue'
import Icon from '@/components/icons/Icon.vue'
import { adminAPI, type IntelligenceTestConfig, type IntelligenceTestAPIKeyOption } from '@/api/admin'

const configs = ref<IntelligenceTestConfig[]>([])
const apiKeys = ref<IntelligenceTestAPIKeyOption[]>([])
const loading = ref(false)
const saving = ref(false)
const runningId = ref<number | null>(null)
const dialogOpen = ref(false)
const editing = ref<IntelligenceTestConfig | null>(null)
const error = ref('')
const form = reactive({ name: '', model_id: '', prompt_text: '', api_key_id: 0, cron_expression: '0 * * * *', keep_count: 20, enabled: true })

async function load() {
  loading.value = true; error.value = ''
  try { [configs.value, apiKeys.value] = await Promise.all([adminAPI.intelligenceTest.listConfigs(), adminAPI.intelligenceTest.listAPIKeys()]) } catch (cause) { error.value = cause instanceof Error ? cause.message : '加载失败' } finally { loading.value = false }
}
function resetForm() { Object.assign(form, { name: '', model_id: '', prompt_text: '', api_key_id: apiKeys.value[0]?.id || 0, cron_expression: '0 * * * *', keep_count: 20, enabled: true }) }
function openCreate() { editing.value = null; resetForm(); dialogOpen.value = true }
function openEdit(item: IntelligenceTestConfig) { editing.value = item; Object.assign(form, { name: item.name, model_id: item.model_id, prompt_text: item.prompt_text, api_key_id: item.api_key_id, cron_expression: item.cron_expression, keep_count: item.keep_count, enabled: item.enabled }); dialogOpen.value = true }
async function save() { saving.value = true; error.value = ''; try { const input = { ...form }; if (editing.value) await adminAPI.intelligenceTest.update(editing.value.id, input); else await adminAPI.intelligenceTest.create(input); dialogOpen.value = false; await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '保存失败' } finally { saving.value = false } }
async function run(item: IntelligenceTestConfig) { runningId.value = item.id; error.value = ''; try { await adminAPI.intelligenceTest.runNow(item.id); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '运行失败' } finally { runningId.value = null } }
async function remove(item: IntelligenceTestConfig) { if (!window.confirm(`确定删除“${item.name}”吗？`)) return; try { await adminAPI.intelligenceTest.remove(item.id); await load() } catch (cause) { error.value = cause instanceof Error ? cause.message : '删除失败' } }
onMounted(load)
</script>
