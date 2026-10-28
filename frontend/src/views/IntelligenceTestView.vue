<template>
  <div class="intelligence-test" :class="{ 'intelligence-test--embedded': isEmbedded }">
    <header v-if="!isEmbedded" class="test-header">
      <div class="test-header__inner">
        <RouterLink to="/home" class="brand-link" aria-label="返回首页">
          <span class="brand-mark"><img :src="siteLogo || '/logo.svg'" alt="" /></span>
          <span class="brand-name">{{ siteName }}</span>
        </RouterLink>
        <div class="header-actions">
          <RouterLink to="/login" class="header-link">登录</RouterLink>
          <button class="icon-button" type="button" :title="isDark ? '切换到浅色模式' : '切换到深色模式'" @click="toggleTheme">
            <Icon :name="isDark ? 'sun' : 'moon'" size="sm" />
          </button>
        </div>
      </div>
    </header>

    <main class="test-shell">
      <section class="test-intro">
        <div>
          <p class="eyebrow"><Icon name="brain" size="xs" /> {{ copy.eyebrow }}</p>
          <h1>{{ config.title || copy.title }}</h1>
          <p class="intro-description">{{ config.description || copy.description }}</p>
        </div>
        <button class="refresh-button" type="button" :disabled="loading" @click="loadResults">
          <Icon name="refresh" size="sm" :class="{ 'spin-once': loading }" />
          {{ loading ? copy.refreshing : copy.refresh }}
        </button>
      </section>

      <section class="prompt-panel" aria-labelledby="prompt-heading">
        <div class="prompt-panel__icon"><Icon name="lightbulb" size="md" /></div>
        <div class="prompt-panel__content">
          <div class="prompt-panel__heading">
            <h2 id="prompt-heading">{{ config.prompt_name || copy.currentPrompt }}</h2>
            <span v-if="config.question_id" class="meta-pill">#{{ config.question_id }}</span>
          </div>
          <p>{{ config.prompt_text || config.question_text || copy.promptPlaceholder }}</p>
        </div>
        <div class="prompt-panel__meta">
          <span>{{ copy.groupCount }}</span><strong>{{ groups.length }}</strong>
          <span>{{ copy.workCount }}</span><strong>{{ totalCount }}</strong>
        </div>
      </section>

      <div class="toolbar" role="toolbar" :aria-label="copy.filters">
        <div class="filter-tabs" role="tablist" :aria-label="copy.groups">
          <button
            type="button"
            class="filter-tab"
            :class="{ active: selectedGroup === ALL_GROUP }"
            role="tab"
            :aria-selected="selectedGroup === ALL_GROUP"
            @click="selectedGroup = ALL_GROUP"
          >
            {{ copy.allGroups }} <span>{{ totalCount }}</span>
          </button>
          <button
            v-for="group in groups"
            :key="group.name"
            type="button"
            class="filter-tab"
            :class="{ active: selectedGroup === group.name }"
            role="tab"
            :aria-selected="selectedGroup === group.name"
            @click="selectedGroup = group.name"
          >
            {{ group.name }} <span>{{ group.items.length }}</span>
          </button>
        </div>
        <div class="toolbar-controls">
          <label class="select-wrap">
            <span class="sr-only">{{ copy.statusFilter }}</span>
            <select v-model="statusFilter" :aria-label="copy.statusFilter">
              <option value="all">{{ copy.allStatus }}</option>
              <option value="ok">{{ copy.validated }}</option>
              <option value="recent">{{ copy.recent }}</option>
            </select>
            <Icon name="chevronDown" size="xs" />
          </label>
        </div>
      </div>

      <div v-if="loading && !hasLoaded" class="state-panel">
        <div class="loader"></div>
        <p>{{ copy.loading }}</p>
      </div>
      <div v-else-if="error" class="state-panel state-panel--error">
        <Icon name="exclamationCircle" size="lg" />
        <h2>{{ copy.loadFailed }}</h2>
        <p>{{ error }}</p>
        <button type="button" class="secondary-button" @click="loadResults">{{ copy.retry }}</button>
      </div>
      <div v-else-if="visibleGroups.length === 0" class="state-panel">
        <Icon name="beaker" size="lg" />
        <h2>{{ copy.noResults }}</h2>
        <p>{{ copy.noResultsDescription }}</p>
      </div>
      <template v-else>
        <section v-for="group in visibleGroups" :key="group.name" class="group-section">
          <div class="section-heading">
            <div>
              <h2>{{ group.name }}</h2>
              <span>{{ group.items.length }} {{ copy.works }}</span>
            </div>
            <div class="section-arrows">
              <button type="button" :aria-label="`${copy.previous} ${group.name}`" @click="scrollGroup(group.name, -1)">
                <Icon name="chevronLeft" size="sm" />
              </button>
              <button type="button" :aria-label="`${copy.next} ${group.name}`" @click="scrollGroup(group.name, 1)">
                <Icon name="chevronRight" size="sm" />
              </button>
            </div>
          </div>
          <div :ref="(el) => setGroupRef(group.name, el)" class="works-row">
            <div v-if="group.items.length === 0" class="empty-row">{{ copy.emptyGroup }}</div>
            <button
              v-for="item in group.items"
              :key="item.id || `${group.name}-${item.svg || item.created_at}`"
              type="button"
              class="work-card"
              @click="openPreview(item, group.name)"
            >
              <span class="work-preview">
                <img :src="getAssetUrl(item)" :alt="`${group.name} ${item.model_name || item.model || item.model_id || copy.svgWork}`" loading="lazy" />
                <span v-if="item.check || item.status === 'validated'" class="validated-mark" :title="copy.validated">
                  <Icon name="check" size="xs" />
                </span>
              </span>
              <span class="work-meta">
                <span class="work-model">{{ item.model_name || item.model || item.model_id || copy.unknownModel }}</span>
                <span class="work-tags">
                  <span>{{ copy.reasoning }}：{{ formatEffort(item.effort) }}</span>
                  <span>{{ copy.elapsed }}：{{ formatLatency(item.latency_ms) || '—' }}</span>
                  <span>{{ formatDate(item) }}</span>
                </span>
              </span>
            </button>
          </div>
        </section>
      </template>
    </main>

    <div v-if="preview" class="preview-backdrop" role="dialog" aria-modal="true" :aria-label="copy.preview" @click.self="closePreview">
      <section class="preview-modal">
        <header class="preview-header">
          <div>
            <strong>{{ preview.group }}</strong>
            <span>{{ preview.item.model_name || preview.item.model || preview.item.model_id || copy.unknownModel }}</span>
          </div>
          <button type="button" class="icon-button" :aria-label="copy.close" :title="copy.close" @click="closePreview">
            <Icon name="x" size="sm" />
          </button>
        </header>
        <div class="preview-canvas">
          <img :src="getAssetUrl(preview.item)" :alt="copy.svgWork" />
        </div>
        <footer class="preview-footer">
          <span>{{ formatDate(preview.item) }}</span>
          <span>{{ copy.reasoning }}：{{ formatEffort(preview.item.effort) }}</span>
          <span>{{ copy.elapsed }}：{{ formatLatency(preview.item.latency_ms) || '—' }}</span>
          <span v-if="preview.item.check || preview.item.status === 'validated'" class="validated-label"><Icon name="check" size="xs" /> {{ copy.validated }}</span>
        </footer>
      </section>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import Icon from '@/components/icons/Icon.vue'
import { useAppStore } from '@/stores/app'
import { getIntelligenceTest, type IntelligenceTestItem, type IntelligenceTestResponse } from '@/api/intelligenceTest'

const ALL_GROUP = '__all__'
const route = useRoute()
const appStore = useAppStore()

const loading = ref(true)
const hasLoaded = ref(false)
const error = ref('')
const selectedGroup = ref(ALL_GROUP)
const statusFilter = ref<'all' | 'ok' | 'recent'>('all')
const response = ref<IntelligenceTestResponse>({})
const preview = ref<{ item: IntelligenceTestItem; group: string } | null>(null)
const groupRefs = new Map<string, HTMLElement>()

const isEmbedded = computed(() => route.query.ui_mode === 'embedded' || route.query.embedded === '1')
const isDark = ref(route.query.theme === 'dark' || (
  route.query.theme !== 'light' &&
  typeof document !== 'undefined' &&
  document.documentElement.classList.contains('dark')
))
const siteName = computed(() => appStore.siteName || 'Sub2API')
const siteLogo = computed(() => appStore.siteLogo || '')
const query = computed(() => ({
  user_id: asQueryString(route.query.user_id),
  token: asQueryString(route.query.token),
  question_id: asQueryString(route.query.question_id),
  prompt_id: asQueryString(route.query.prompt_id),
}))

const config = computed(() => ({
  ...response.value,
  ...(response.value.config || {}),
}))

const isEnglish = computed(() => String(route.query.lang || '').toLowerCase().startsWith('en'))
const copy = computed(() => isEnglish.value ? {
  eyebrow: 'Capability benchmark', title: 'SVG intelligence benchmark', description: 'Compare SVG generation quality across configured API key groups.', refresh: 'Refresh', refreshing: 'Refreshing', currentPrompt: 'Current prompt', promptPlaceholder: 'The prompt configured by your administrator will appear here.', groupCount: 'Groups', workCount: 'Works', filters: 'Result filters', groups: 'Groups', allGroups: 'All groups', statusFilter: 'Status', allStatus: 'All status', validated: 'Validated', recent: 'Last hour', loading: 'Loading recent SVG works…', loadFailed: 'Unable to load results', retry: 'Try again', noResults: 'No works yet', noResultsDescription: 'Configure a prompt and API key group in the admin panel, then results will appear here.', emptyGroup: 'No works match the current filter.', works: 'works', previous: 'Previous', next: 'Next', svgWork: 'SVG work', unknownModel: 'Unknown model', preview: 'SVG preview', close: 'Close', reasoning: 'Effort', elapsed: 'Elapsed',
} : {
  eyebrow: '能力测试', title: 'SVG 生成能力测试', description: '对比不同 API Key 分组的 SVG 生成质量，观察模型的推理与执行能力。', refresh: '刷新', refreshing: '刷新中', currentPrompt: '当前提示词', promptPlaceholder: '后台配置的提示词会显示在这里。', groupCount: '分组', workCount: '作品', filters: '结果筛选', groups: '分组', allGroups: '全部分组', statusFilter: '状态', allStatus: '全部状态', validated: '已通过校验', recent: '最近 1 小时', loading: '正在加载最近的 SVG 作品…', loadFailed: '结果加载失败', retry: '重新加载', noResults: '还没有测试作品', noResultsDescription: '请先在后台配置提示词和 API Key 分组，生成结果后会显示在这里。', emptyGroup: '当前筛选条件下暂无作品', works: '张作品', previous: '上一组', next: '下一组', svgWork: 'SVG 作品', unknownModel: '未知模型', preview: 'SVG 作品预览', close: '关闭', reasoning: '推理', elapsed: '耗时',
})

interface DisplayGroup { name: string; items: IntelligenceTestItem[] }

const normalizedItems = computed(() => {
  const direct = (response.value.items || []).map((item) => ({ ...item }))
  const runs = (response.value.runs || []).flatMap((run) => (run.items || []).map((item) => ({
    ...item,
    ts: item.ts || item.created_at || run.ts || run.created_at,
  })))
  const all = direct.length ? direct : runs
  const keepCount = Math.max(1, Number(config.value.keep_count || response.value.keep_count || 20))
  const configuredGroups = new Map<string, IntelligenceTestItem[]>()
  for (const configuredGroup of response.value.groups || []) {
    const name = typeof configuredGroup === 'string'
      ? configuredGroup
      : configuredGroup.name || String(configuredGroup.id || '')
    if (name) configuredGroups.set(name, [])
  }
  return all
    .filter((item) => item.svg || item.svg_url || item.svg_content || item.image_url)
    .sort((a, b) => dateValue(b) - dateValue(a))
    .reduce((groups, item) => {
      const name = item.group_name || item.group || String(item.group_id || (isEnglish.value ? 'Ungrouped' : '未分组'))
      const group = groups.get(name) || []
      if (group.length < keepCount) group.push(item)
      groups.set(name, group)
      return groups
    }, configuredGroups)
})

const groups = computed<DisplayGroup[]>(() => Array.from(normalizedItems.value.entries()).map(([name, items]) => ({ name, items })))
const visibleGroups = computed(() => groups.value
  .filter((group) => selectedGroup.value === ALL_GROUP || selectedGroup.value === group.name)
  .map((group) => ({ ...group, items: group.items.filter((item) => matchesFilter(item)) })))
const totalCount = computed(() => groups.value.reduce((total, group) => total + group.items.length, 0))

function asQueryString(value: unknown) {
  return typeof value === 'string' && value.trim() ? value : undefined
}

function parseDate(value: string) {
  const normalized = value.includes('T') ? value : value.replace(' ', 'T')
  const hasTimezone = /(?:Z|[+-]\d{2}:?\d{2})$/i.test(normalized)
  return Date.parse(hasTimezone ? normalized : `${normalized}+08:00`)
}

function dateValue(item: IntelligenceTestItem) {
  const value = item.ts || item.created_at || item.updated_at || ''
  const parsed = parseDate(value)
  return Number.isFinite(parsed) ? parsed : 0
}

function matchesFilter(item: IntelligenceTestItem) {
  if (statusFilter.value === 'ok' && !(item.check || item.status === 'validated')) return false
  if (statusFilter.value === 'recent' && Date.now() - dateValue(item) > 60 * 60 * 1000) return false
  return true
}

function getAssetUrl(item: IntelligenceTestItem) {
  if (item.svg_content) return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(item.svg_content)}`
  const source = item.image_url || item.svg_url || item.svg || ''
  if (!source) return ''
  if (/^\s*(?:<svg|<\?xml)/i.test(source)) return `data:image/svg+xml;charset=utf-8,${encodeURIComponent(source)}`
  if (/^(data:|blob:|https?:\/\/|\/)/i.test(source)) return source
  const base = config.value.asset_base_url || '/pelican-test/data/'
  return `${base.replace(/\/+$/, '')}/${source.replace(/^\/+/, '')}`
}

function formatEffort(value: string | null | undefined) {
  if (!value) return isEnglish.value ? 'default' : '默认强度'
  const labels: Record<string, string> = isEnglish.value ? { low: 'low', medium: 'medium', high: 'high', xhigh: 'xhigh', max: 'max' } : { low: '低', medium: '中', high: '高', xhigh: '极高', max: '最大' }
  return labels[value] || value
}

function formatLatency(value?: number) {
  return typeof value === 'number' && value > 0 ? `${(value / 1000).toFixed(1)}s` : ''
}

function formatDate(item: IntelligenceTestItem) {
  const value = item.ts || item.created_at || item.updated_at
  if (!value) return ''
  const date = new Date(parseDate(value))
  if (Number.isNaN(date.getTime())) return value.slice(5, 16)
  return new Intl.DateTimeFormat(isEnglish.value ? 'en-US' : 'zh-CN', { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' }).format(date)
}

async function loadResults() {
  loading.value = true
  error.value = ''
  try {
    response.value = await getIntelligenceTest(query.value)
    hasLoaded.value = true
  } catch (cause) {
    error.value = cause instanceof Error ? cause.message : (isEnglish.value ? 'The test service is unavailable.' : '测试服务暂时不可用，请稍后重试。')
  } finally {
    loading.value = false
  }
}

function setGroupRef(name: string, element: unknown) {
  if (element instanceof HTMLElement) groupRefs.set(name, element)
}

function scrollGroup(name: string, direction: number) {
  groupRefs.get(name)?.scrollBy({ left: direction * 520, behavior: 'smooth' })
}

function openPreview(item: IntelligenceTestItem, group: string) {
  preview.value = { item, group }
}

function closePreview() {
  preview.value = null
}

function toggleTheme() {
  isDark.value = !isDark.value
  document.documentElement.classList.toggle('dark', isDark.value)
  localStorage.setItem('theme', isDark.value ? 'dark' : 'light')
}

function onKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') closePreview()
}

onMounted(() => {
  void appStore.fetchPublicSettings()
  document.documentElement.classList.toggle('dark', isDark.value)
  document.addEventListener('keydown', onKeydown)
  void loadResults()
})

onBeforeUnmount(() => document.removeEventListener('keydown', onKeydown))
</script>

<style scoped>
.intelligence-test { min-height: 100vh; background: #f6f7f9; color: #111827; }
.intelligence-test--embedded { background: transparent; }
.test-header { border-bottom: 1px solid #e5e7eb; background: rgba(255,255,255,.92); }
.test-header__inner, .test-shell { width: min(1280px, calc(100% - 40px)); margin: 0 auto; }
.test-header__inner { display:flex; align-items:center; justify-content:space-between; min-height:64px; }
.brand-link, .header-actions, .header-link, .section-heading, .toolbar, .toolbar-controls, .filter-tabs, .prompt-panel, .prompt-panel__heading, .preview-header, .preview-header > div, .preview-footer { display:flex; align-items:center; }
.brand-link { gap:10px; color:inherit; text-decoration:none; }
.brand-mark { width:34px; height:34px; overflow:hidden; display:grid; place-items:center; border:1px solid #e5e7eb; border-radius:9px; background:white; }
.brand-mark img { width:100%; height:100%; object-fit:contain; }
.brand-name { font-size:15px; font-weight:650; }
.header-actions { gap:8px; }.header-link { font-size:13px; color:#6b7280; text-decoration:none; padding:7px 10px; }.header-link:hover { color:#111827; }
.icon-button { display:grid; place-items:center; width:34px; height:34px; border:1px solid #e5e7eb; border-radius:8px; color:#6b7280; background:#fff; cursor:pointer; }.icon-button:hover { color:#111827; background:#f9fafb; }
.test-shell { padding:34px 0 70px; }.test-intro { display:flex; justify-content:space-between; align-items:flex-end; gap:24px; margin-bottom:24px; }.eyebrow { display:flex; align-items:center; gap:6px; margin:0 0 8px; color:#2563eb; font-size:12px; font-weight:700; text-transform:uppercase; letter-spacing:.08em; }.test-intro h1 { margin:0; font-size:28px; line-height:1.2; letter-spacing:0; }.intro-description { max-width:670px; margin:8px 0 0; color:#6b7280; font-size:14px; }.refresh-button, .secondary-button { display:inline-flex; align-items:center; gap:8px; border:1px solid #dbe1e8; border-radius:8px; padding:9px 13px; background:#fff; color:#374151; font:inherit; font-size:13px; cursor:pointer; white-space:nowrap; }.refresh-button:hover, .secondary-button:hover { border-color:#93c5fd; color:#1d4ed8; }.refresh-button:disabled { opacity:.6; cursor:wait; }.spin-once { animation:spin 1s linear infinite; }
.prompt-panel { gap:14px; padding:16px 18px; margin-bottom:24px; border:1px solid #dbe5f2; border-radius:10px; background:#eef5ff; }.prompt-panel__icon { display:grid; place-items:center; flex:0 0 auto; width:38px; height:38px; border-radius:9px; color:#2563eb; background:#dbeafe; }.prompt-panel__content { min-width:0; flex:1; }.prompt-panel__heading { gap:8px; }.prompt-panel h2 { margin:0; font-size:14px; font-weight:700; }.prompt-panel__content p { margin:5px 0 0; color:#52627a; font-size:13px; line-height:1.55; display:-webkit-box; -webkit-line-clamp:2; -webkit-box-orient:vertical; overflow:hidden; }.prompt-panel__meta { display:grid; grid-template-columns:auto auto; gap:2px 10px; flex:0 0 auto; color:#64748b; font-size:11px; text-align:right; }.prompt-panel__meta strong { color:#1e3a8a; font-size:15px; }.meta-pill, .work-tags span { border-radius:5px; background:#e7edf5; color:#64748b; padding:2px 6px; font-size:11px; }
.toolbar { justify-content:space-between; gap:16px; margin-bottom:20px; }.filter-tabs { gap:7px; flex-wrap:wrap; }.filter-tab { border:1px solid #e1e5ea; border-radius:999px; padding:7px 13px; background:#fff; color:#4b5563; font:inherit; font-size:13px; cursor:pointer; }.filter-tab span { margin-left:4px; color:#9ca3af; }.filter-tab.active { border-color:#111827; background:#111827; color:#fff; }.filter-tab.active span { color:#d1d5db; }.select-wrap { position:relative; display:inline-flex; align-items:center; }.select-wrap select { appearance:none; min-width:112px; border:1px solid #e1e5ea; border-radius:8px; padding:8px 30px 8px 10px; background:#fff; color:#374151; font:inherit; font-size:13px; cursor:pointer; }.select-wrap svg { position:absolute; right:9px; pointer-events:none; color:#6b7280; }.sr-only { position:absolute; width:1px; height:1px; padding:0; margin:-1px; overflow:hidden; clip:rect(0,0,0,0); white-space:nowrap; border:0; }
.group-section { margin-bottom:30px; }.section-heading { justify-content:space-between; margin-bottom:10px; }.section-heading h2 { margin:0; font-size:16px; }.section-heading span { color:#9ca3af; font-size:12px; }.section-arrows { display:flex; gap:6px; }.section-arrows button { display:grid; place-items:center; width:30px; height:30px; border:1px solid #e1e5ea; border-radius:50%; background:#fff; color:#6b7280; cursor:pointer; }.section-arrows button:hover { background:#f3f4f6; color:#111827; }
.works-row { display:flex; gap:14px; overflow-x:auto; padding:4px 2px 13px; scroll-snap-type:x mandatory; scrollbar-width:thin; }.empty-row { align-self:center; min-width:300px; padding:30px; color:#9ca3af; font-size:13px; text-align:center; }.work-card { flex:0 0 320px; padding:0; overflow:hidden; scroll-snap-align:start; border:1px solid #e1e5ea; border-radius:10px; background:#fff; text-align:left; cursor:pointer; transition:transform .16s, box-shadow .16s; }.work-card:hover { transform:translateY(-3px); box-shadow:0 8px 22px rgba(15,23,42,.09); }.work-preview { display:block; position:relative; height:220px; background:#fafafa; border-bottom:1px solid #e8ebef; }.work-preview img { display:block; width:100%; height:100%; object-fit:contain; background:#fff; }.validated-mark { position:absolute; top:10px; right:10px; display:grid; place-items:center; width:23px; height:23px; border-radius:50%; color:#047857; background:#d1fae5; }.work-meta { display:block; padding:10px 12px 11px; }.work-model { display:block; overflow:hidden; color:#1f2937; font-size:13px; font-weight:650; text-overflow:ellipsis; white-space:nowrap; }.work-tags { display:flex; gap:5px; margin-top:7px; }.work-tags span { max-width:130px; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }.work-tags span:empty { display:none; }
.state-panel { display:flex; flex-direction:column; align-items:center; justify-content:center; min-height:300px; padding:30px; border:1px dashed #d7dde5; border-radius:10px; color:#9ca3af; background:rgba(255,255,255,.6); text-align:center; }.state-panel h2 { margin:13px 0 4px; color:#374151; font-size:16px; }.state-panel p { max-width:440px; margin:0; font-size:13px; line-height:1.6; }.state-panel--error { color:#dc2626; }.state-panel--error h2 { color:#991b1b; }.state-panel--error p { color:#6b7280; }.state-panel .secondary-button { margin-top:16px; }.loader { width:28px; height:28px; border:3px solid #dbeafe; border-top-color:#2563eb; border-radius:50%; animation:spin .8s linear infinite; }
.preview-backdrop { position:fixed; inset:0; z-index:20; display:grid; place-items:center; padding:20px; background:rgba(17,24,39,.62); }.preview-modal { width:min(1000px, 96vw); overflow:hidden; border-radius:12px; background:#fff; box-shadow:0 24px 80px rgba(0,0,0,.25); }.preview-header { justify-content:space-between; gap:12px; padding:11px 14px; border-bottom:1px solid #e5e7eb; }.preview-header > div { gap:10px; min-width:0; }.preview-header strong { font-size:14px; }.preview-header span { overflow:hidden; color:#6b7280; font-size:12px; text-overflow:ellipsis; white-space:nowrap; }.preview-canvas { display:grid; place-items:center; min-height:360px; max-height:78vh; background:#fafafa; }.preview-canvas img { display:block; width:100%; height:min(78vh, 720px); object-fit:contain; background:#fff; }.preview-footer { gap:8px; padding:10px 14px; border-top:1px solid #e5e7eb; color:#6b7280; font-size:12px; }.validated-label { display:inline-flex; align-items:center; gap:4px; color:#047857; }
@keyframes spin { to { transform:rotate(360deg); } }
@media (prefers-reduced-motion:reduce) { .work-card { transition:none; }.spin-once, .loader { animation:none; } }
@media (max-width:720px) { .test-header__inner, .test-shell { width:min(100% - 28px, 1280px); }.test-shell { padding-top:24px; }.test-intro { align-items:flex-start; flex-direction:column; gap:14px; }.test-intro h1 { font-size:24px; }.refresh-button { align-self:flex-start; }.prompt-panel { align-items:flex-start; flex-wrap:wrap; }.prompt-panel__content { flex-basis:calc(100% - 54px); }.prompt-panel__meta { width:100%; display:flex; gap:8px; align-items:center; padding-left:52px; text-align:left; }.prompt-panel__meta strong { margin-right:7px; }.toolbar { align-items:flex-start; flex-direction:column; }.toolbar-controls { width:100%; }.select-wrap, .select-wrap select { width:100%; }.work-card { flex-basis:min(320px, calc(100vw - 38px)); }.work-preview { height:210px; } }
.dark .intelligence-test { background:#0f172a; color:#f8fafc; }.dark .test-header { border-color:#1e293b; background:rgba(15,23,42,.92); }.dark .brand-mark, .dark .icon-button, .dark .refresh-button, .dark .secondary-button, .dark .filter-tab, .dark .select-wrap select, .dark .section-arrows button, .dark .work-card { border-color:#334155; background:#111c2e; color:#e2e8f0; }.dark .header-link, .dark .intro-description, .dark .prompt-panel__content p, .dark .section-heading span, .dark .state-panel p, .dark .preview-header span, .dark .preview-footer { color:#94a3b8; }.dark .eyebrow { color:#60a5fa; }.dark .prompt-panel { border-color:#1e40af; background:#172554; }.dark .prompt-panel__icon { background:#1e3a8a; color:#93c5fd; }.dark .prompt-panel__meta strong { color:#bfdbfe; }.dark .meta-pill, .dark .work-tags span { background:#243247; color:#94a3b8; }.dark .filter-tab { color:#cbd5e1; }.dark .filter-tab.active { border-color:#f8fafc; background:#f8fafc; color:#0f172a; }.dark .filter-tab.active span { color:#64748b; }.dark .work-preview, .dark .preview-canvas { background:#0b1220; }.dark .work-preview img, .dark .preview-canvas img { background:#fff; }.dark .work-model, .dark .state-panel h2, .dark .preview-header strong { color:#f8fafc; }.dark .state-panel { border-color:#334155; background:rgba(15,23,42,.55); }.dark .state-panel--error h2 { color:#fca5a5; }.dark .preview-modal { background:#111c2e; }.dark .preview-header, .dark .preview-footer { border-color:#334155; }.dark .preview-canvas { border-color:#334155; }
.dark .empty-row { color:#64748b; }
</style>
