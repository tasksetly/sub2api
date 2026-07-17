import { describe, expect, it } from 'vitest'

import { sanitizeRedeemStoreHtml } from '@/utils/sanitizeRedeemStoreHtml'

describe('sanitizeRedeemStoreHtml', () => {
  it('keeps text formatting and makes http links safe', () => {
    const html = sanitizeRedeemStoreHtml(
      '<p><strong>Buy now</strong> <a href="https://store.example.com/item">Store</a></p>'
    )

    expect(html).toContain('<strong>Buy now</strong>')
    expect(html).toContain('href="https://store.example.com/item"')
    expect(html).toContain('target="_blank"')
    expect(html).toContain('rel="noopener noreferrer"')
  })

  it('removes scripts, event handlers, images, and unsafe links', () => {
    const html = sanitizeRedeemStoreHtml(
      '<script>alert(1)</script><img src=x onerror=alert(1)><a href="javascript:alert(1)">Bad</a><p onclick="alert(1)">Text</p>'
    )

    expect(html).not.toContain('<script')
    expect(html).not.toContain('<img')
    expect(html).not.toContain('onclick')
    expect(html).not.toContain('javascript:')
    expect(html).not.toContain('<a')
    expect(html).toContain('Bad')
    expect(html).toContain('<p>Text</p>')
  })
})
