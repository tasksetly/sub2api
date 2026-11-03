import { useHead } from '@vueuse/head'
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

export function useSeoMeta(options: SeoMetaOptions = {}) {
  const { t, locale } = useI18n()
  const appStore = useAppStore()

  const resolvedTitle = computed(() => {
    const siteName = appStore.siteName || 'Tasksetly'
    if (options.titleKey) {
      const translated = t(options.titleKey)
      return `${translated} - ${siteName}`
    }
    if (options.title) {
      return `${options.title} - ${siteName}`
    }
    return siteName
  })

  const resolvedDescription = computed(() => {
    if (options.descriptionKey) {
      return t(options.descriptionKey)
    }
    return options.description || 'AI API Gateway Platform for subscription quota distribution'
  })

  useHead({
    title: resolvedTitle,
    htmlAttrs: { lang: locale },
    meta: [
      { name: 'description', content: resolvedDescription },
      ...(options.keywords ? [{ name: 'keywords', content: options.keywords }] : []),
      { property: 'og:title', content: resolvedTitle },
      { property: 'og:description', content: resolvedDescription },
    ],
  })
}
