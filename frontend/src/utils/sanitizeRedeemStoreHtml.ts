import DOMPurify from 'dompurify'

const ALLOWED_TAGS = [
  'a',
  'b',
  'br',
  'em',
  'h2',
  'h3',
  'h4',
  'i',
  'li',
  'ol',
  'p',
  'span',
  'strong',
  'u',
  'ul'
]

export function sanitizeRedeemStoreHtml(value: string): string {
  const sanitized = DOMPurify.sanitize(String(value || ''), {
    ALLOWED_TAGS,
    ALLOWED_ATTR: ['href', 'title']
  })
  if (!sanitized || typeof document === 'undefined') return sanitized

  const template = document.createElement('template')
  template.innerHTML = sanitized
  template.content.querySelectorAll('a').forEach((link) => {
    const href = link.getAttribute('href')?.trim() || ''
    if (!/^https?:\/\//i.test(href)) {
      link.replaceWith(...Array.from(link.childNodes))
      return
    }
    link.setAttribute('target', '_blank')
    link.setAttribute('rel', 'noopener noreferrer')
  })
  return template.innerHTML
}
