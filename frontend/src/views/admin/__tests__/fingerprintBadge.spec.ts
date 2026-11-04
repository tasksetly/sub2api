import { describe, expect, it } from 'vitest'

import {
  fingerprintBadgeClass,
  fingerprintBadgeHint,
  fingerprintBadgeStatus
} from '../fingerprintBadge'

describe('fingerprintBadgeStatus', () => {
  it('不符时返回 mismatch', () => {
    expect(fingerprintBadgeStatus({ fingerprint_status: 'mismatch' })).toBe('mismatch')
  })

  it('无法判定与检测失败各自独立，不并进存疑', () => {
    expect(fingerprintBadgeStatus({ fingerprint_status: 'inconclusive' })).toBe('inconclusive')
    expect(fingerprintBadgeStatus({ fingerprint_status: 'error' })).toBe('error')
  })

  it('相符时返回 match，标记随最近一次结果自动更新', () => {
    expect(fingerprintBadgeStatus({ fingerprint_status: 'match' })).toBe('match')
  })

  it('没测过或值不认识都归为 unknown', () => {
    expect(fingerprintBadgeStatus(undefined)).toBe('unknown')
    expect(fingerprintBadgeStatus(null)).toBe('unknown')
    expect(fingerprintBadgeStatus({})).toBe('unknown')
    expect(fingerprintBadgeStatus({ fingerprint_status: 'something-else' })).toBe('unknown')
    expect(fingerprintBadgeStatus({ fingerprint_status: 42 })).toBe('unknown')
  })
})

describe('fingerprintBadgeClass', () => {
  it('四态各有不同配色', () => {
    const tones = (['match', 'mismatch', 'inconclusive', 'error'] as const).map(fingerprintBadgeClass)
    expect(new Set(tones).size).toBe(4)
    tones.forEach((tone) => expect(tone).not.toBe(''))
  })

  it('unknown 不上色', () => {
    expect(fingerprintBadgeClass('unknown')).toBe('')
  })
})

describe('fingerprintBadgeHint', () => {
  it('带上实判模型与检测时间', () => {
    const hint = fingerprintBadgeHint(
      {
        fingerprint_predicted_model: 'deepseek-v4-flash',
        fingerprint_checked_at: '2026-11-04T01:00:09Z'
      },
      '实判模型'
    )
    expect(hint).toContain('实判模型: deepseek-v4-flash')
    expect(hint).toContain('·')
  })

  it('时间非法时原样显示而不是 Invalid Date', () => {
    const hint = fingerprintBadgeHint({ fingerprint_checked_at: 'not-a-date' }, '实判模型')
    expect(hint).toBe('not-a-date')
  })

  it('字段缺失时返回空串而不炸', () => {
    expect(fingerprintBadgeHint(undefined, '实判模型')).toBe('')
    expect(fingerprintBadgeHint({}, '实判模型')).toBe('')
    expect(fingerprintBadgeHint({ fingerprint_predicted_model: 123 }, '实判模型')).toBe('')
  })
})
