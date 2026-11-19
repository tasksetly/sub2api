import type { IntelligenceTestItem } from '@/api/intelligenceTest'

export function newestCandyRunsFirst(a: IntelligenceTestItem, b: IntelligenceTestItem) {
  return (b.created_at || '').localeCompare(a.created_at || '') || Number(b.id) - Number(a.id)
}

export function candyStatusKey(status?: string) {
  switch (status) {
    case 'validated': return 'intelligenceTest.validated'
    case 'incorrect': return 'intelligenceTest.incorrect'
    case 'running': return 'intelligenceTest.running'
    default: return 'intelligenceTest.failed'
  }
}

export function candyStatusColor(status?: string) {
  if (status === 'validated') return 'bg-emerald-500'
  if (status === 'incorrect') return 'bg-red-500'
  if (status === 'running') return 'bg-blue-300'
  return 'bg-gray-300 dark:bg-gray-600'
}

export function formatCandyRunDate(item: IntelligenceTestItem, locale: string) {
  const date = new Date(item.created_at || '')
  return Number.isNaN(date.getTime()) ? '—' : date.toLocaleString(locale, { month: '2-digit', day: '2-digit', hour: '2-digit', minute: '2-digit' })
}
