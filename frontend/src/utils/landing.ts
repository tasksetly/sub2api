export function getHomePrimaryDestination(
  isAuthenticated: boolean,
  dashboardPath: string,
): string {
  return isAuthenticated ? dashboardPath : '/register'
}

export function getHomeDocsUrl(configuredUrl: string): string {
  return configuredUrl.trim() || '/docs/quickstart.html'
}

const GENERIC_ENGLISH_SUBTITLES = new Set([
  'AI API Gateway Platform',
  'Subscription to API Conversion Platform',
])

export function resolveHomeSubtitle(
  configuredSubtitle: string,
  locale: string,
  localizedDefault: string,
): string {
  const subtitle = configuredSubtitle.trim()
  if (locale.toLowerCase().startsWith('zh') && GENERIC_ENGLISH_SUBTITLES.has(subtitle)) {
    return localizedDefault
  }
  return subtitle || localizedDefault
}
