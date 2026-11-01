// Exact defaults created by backend/internal/repository/simple_mode_default_groups.go.
const defaultGroupKeys = new Map([
  ['anthropic-default', 'anthropic'],
  ['openai-default', 'openai'],
  ['gemini-default', 'gemini'],
  ['grok-default', 'grok'],
  ['antigravity-default-1', 'antigravity1'],
  ['antigravity-default-2', 'antigravity2']
])

export function groupDisplayName(name: string, t: (key: string) => string): string {
  const key = defaultGroupKeys.get(name)
  return key ? t(`modelPlaza.groupNames.${key}`) : name
}
