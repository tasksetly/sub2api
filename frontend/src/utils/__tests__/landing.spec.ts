import { describe, expect, it } from 'vitest'
import {
  getHomeDocsUrl,
  getHomePrimaryDestination,
  resolveHomeSubtitle,
} from '@/utils/landing'

describe('getHomePrimaryDestination', () => {
  it('sends unauthenticated visitors directly to registration', () => {
    expect(getHomePrimaryDestination(false, '/dashboard')).toBe('/register')
  })

  it('keeps authenticated visitors on their dashboard path', () => {
    expect(getHomePrimaryDestination(true, '/admin/dashboard')).toBe('/admin/dashboard')
  })
})

describe('resolveHomeSubtitle', () => {
  it('replaces the generic English default with localized copy on Chinese pages', () => {
    expect(
      resolveHomeSubtitle(
        'Subscription to API Conversion Platform',
        'zh-CN',
        '一个 API Key，接入你常用的 AI 模型',
      ),
    ).toBe('一个 API Key，接入你常用的 AI 模型')
  })

  it('preserves an administrator-defined subtitle', () => {
    expect(resolveHomeSubtitle('团队专属 AI 网关', 'zh-CN', '默认文案')).toBe(
      '团队专属 AI 网关',
    )
  })
})

describe('getHomeDocsUrl', () => {
  it('uses the local quickstart when no administrator URL is configured', () => {
    expect(getHomeDocsUrl('')).toBe('/docs/quickstart.html')
  })

  it('preserves an administrator-defined documentation URL', () => {
    expect(getHomeDocsUrl('https://docs.example.com/start')).toBe(
      'https://docs.example.com/start',
    )
  })
})
