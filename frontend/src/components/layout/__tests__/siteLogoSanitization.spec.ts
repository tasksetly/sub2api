import { readFileSync } from 'node:fs'
import { dirname, resolve } from 'node:path'
import { fileURLToPath } from 'node:url'

import { describe, expect, it } from 'vitest'

const dir = dirname(fileURLToPath(import.meta.url))
const sidebarSource = readFileSync(resolve(dir, '../AppSidebar.vue'), 'utf8')
const authLayoutSource = readFileSync(resolve(dir, '../AuthLayout.vue'), 'utf8')
const homeViewSource = readFileSync(resolve(dir, '../../../views/HomeView.vue'), 'utf8')
const keyUsageViewSource = readFileSync(resolve(dir, '../../../views/KeyUsageView.vue'), 'utf8')

describe('site_logo sanitization', () => {
  it('AppSidebar imports sanitizeUrl and applies it to siteLogo', () => {
    expect(sidebarSource).toContain("import { sanitizeUrl } from '@/utils/url'")
    expect(sidebarSource).toContain('sanitizeUrl(appStore.siteLogo')
  })

  it('AuthLayout uses the page content logo with a site logo fallback', () => {
    expect(authLayoutSource).toContain('appStore.cachedPublicSettings?.site_content_logo || appStore.siteContentLogo || appStore.siteLogo')
    expect(authLayoutSource).toContain('sanitizeUrl')
  })

  it('HomeView applies sanitizeUrl to both navigation and content logos', () => {
    expect(homeViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
    expect(homeViewSource).toContain('appStore.cachedPublicSettings?.site_content_logo || appStore.siteContentLogo || appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('KeyUsageView applies sanitizeUrl to siteLogo', () => {
    expect(keyUsageViewSource).toContain('sanitizeUrl(appStore.cachedPublicSettings?.site_logo || appStore.siteLogo')
  })

  it('all three pass allowRelative and allowDataUrl options', () => {
    for (const src of [sidebarSource, homeViewSource, keyUsageViewSource]) {
      expect(src).toContain('allowRelative: true')
      expect(src).toContain('allowDataUrl: true')
    }
  })
})
