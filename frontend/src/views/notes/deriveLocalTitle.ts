/**
 * sicau-v1 notes N-6: list title derivation (mirrors backend deriveNoteTitle).
 * First `#` heading in the head wins; otherwise the first non-empty line.
 * Truncated to 50 Unicode code points.
 */
export function deriveLocalTitle(text: string, maxRunes = 50): string {
  const lines = text.split('\n')
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed.startsWith('#')) continue
    const stripped = trimmed.replace(/^#+/, '').trim()
    if (!stripped) continue
    return [...stripped].slice(0, maxRunes).join('')
  }
  for (const line of lines) {
    const trimmed = line.trim()
    if (!trimmed) continue
    const stripped = trimmed.replace(/^#+/, '').trim()
    if (!stripped) continue
    return [...trimmed].slice(0, maxRunes).join('')
  }
  return ''
}
