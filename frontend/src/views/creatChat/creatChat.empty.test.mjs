import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'

const here = dirname(fileURLToPath(import.meta.url))

test('creatChat.vue wires the shared empty-state shell', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')
  assert.match(source, /NewConversationEmptyState/)
  assert.match(source, /suggested-questions-container/)
  assert.match(source, /<InputField\b/)
  assert.doesNotMatch(source, /dialogue-title/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-wrap\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-answers\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-title\s*\)/)
  assert.match(source, /newConversationEmptyState\.less/)
})

test('empty-state layout pins questions and composer to the bottom', () => {
  const less = readFileSync(join(here, 'newConversationEmptyState.less'), 'utf8')
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*flex:\s*1/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*display:\s*flex/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*flex-direction:\s*column/s)
  assert.match(less, /\.dialogue-wrap\s*\{[^}]*justify-content:\s*flex-end/s)
  assert.doesNotMatch(less, /justify-content:\s*center/)
  assert.match(less, /\.dialogue-answers\s*\{/s)
  assert.doesNotMatch(less, /\.dialogue-title\s*\{/s)
})

test('ongoing chat pins suggested questions with the composer', () => {
  const source = readFileSync(join(here, '../chat/index.vue'), 'utf8')
  const scrollAt = source.indexOf('class="chat_scroll_box"')
  const inputAt = source.indexOf('class="input-container"')
  const questionsAt = source.indexOf('suggested-questions-container')
  assert.ok(scrollAt >= 0 && inputAt > scrollAt && questionsAt > inputAt)
  assert.doesNotMatch(source.slice(scrollAt, inputAt), /suggested-questions-container/)
})

test('campus slogan stays in locale and is not a WeKnora product pitch title', () => {
  const zh = readFileSync(join(here, '../../i18n/locales/zh-CN.ts'), 'utf8')
  const m = zh.match(/createChat:\s*\{([\s\S]*?)\n\s*\},/)
  assert.ok(m, 'createChat block missing')
  const block = m[1]
  assert.match(block, /title: '让你的知识触手可及'/)
  assert.doesNotMatch(block, /WeKnora/)
})

