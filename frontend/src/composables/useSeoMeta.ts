import { watchEffect } from 'vue'
import { computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { useAppStore } from '@/stores/app'

export interface SeoMetaOptions {
  title?: string
  titleKey?: string
  description?: string
  descriptionKey?: string
  keywords?: string
}

function setMeta(name: string, content: string, attr = 'name') {
  let el = document.querySelector<HTMLMetaElement>(`meta[${attr}="${name}"]`)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, name)
    document.head.appendChild(el)
  }
  el.content = content
}

export function useSeoMeta(options: SeoMetaOptions = {}) {
  const { t, locale } = useI18n()
  const appStore = useAppStore()

  const resolvedTitle = computed(() => {
    const siteName = appStore.siteName || 'Tasksetly'
    if (options.titleKey) return `${t(options.titleKey)} - ${siteName}`
    if (options.title) return `${options.title} - ${siteName}`
    return siteName
  })

  const resolvedDescription = computed(() => {
    if (options.descriptionKey) return t(options.descriptionKey)
    return options.description || 'AI API Gateway Platform for subscription quota distribution'
  })

  watchEffect(() => {
    document.title = resolvedTitle.value
    document.documentElement.lang = locale.value
    setMeta('description', resolvedDescription.value)
    if (options.keywords) setMeta('keywords', options.keywords)
    setMeta('og:title', resolvedTitle.value, 'property')
    setMeta('og:description', resolvedDescription.value, 'property')
  })
}
