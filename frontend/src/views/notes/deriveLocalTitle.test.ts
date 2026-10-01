import assert from 'node:assert/strict'
import test from 'node:test'
import { deriveLocalTitle } from './deriveLocalTitle'

test('deriveLocalTitle: heading wins over earlier plain line (N-6)', () => {
  assert.equal(deriveLocalTitle('乡村建设行动方案\n# 真正的标题'), '真正的标题')
})

test('deriveLocalTitle: first heading text when only headings', () => {
  assert.equal(deriveLocalTitle('# 会议纪要\n正文'), '会议纪要')
})

test('deriveLocalTitle: falls back to first non-empty line', () => {
  assert.equal(deriveLocalTitle('乡村建设行动方案\n后面没有井号'), '乡村建设行动方案')
})

test('deriveLocalTitle: bare hashes skipped', () => {
  assert.equal(deriveLocalTitle('###\n正文开始'), '正文开始')
})

test('deriveLocalTitle: truncates to 50 runes', () => {
  assert.equal(deriveLocalTitle('长'.repeat(80)), '长'.repeat(50))
})

test('deriveLocalTitle: empty', () => {
  assert.equal(deriveLocalTitle(''), '')
})
