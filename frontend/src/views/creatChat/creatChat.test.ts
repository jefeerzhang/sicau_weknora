import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { createI18n } from 'vue-i18n'

import zhCN from '../../i18n/locales/zh-CN.ts'
import enUS from '../../i18n/locales/en-US.ts'
import ruRU from '../../i18n/locales/ru-RU.ts'
import koKR from '../../i18n/locales/ko-KR.ts'

const here = dirname(fileURLToPath(import.meta.url))

const SUPPORTED_LOCALES = [
  { name: 'zh-CN', messages: zhCN },
  { name: 'en-US', messages: enUS },
  { name: 'ru-RU', messages: ruRU },
  { name: 'ko-KR', messages: koKR },
]

// Resolve the title through the same vue-i18n seam the empty-state component uses,
// so we assert the rendered/localized output instead of the internal locale shape.
function resolveNewConversationTitle(locale: string, messages: unknown): string {
  const i18n = createI18n({
    legacy: false,
    locale,
    // The locale objects are plain message trees; this cast keeps the test focused
    // on resolved output rather than the exact vue-i18n message type surface.
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    messages: { [locale]: messages } as any,
  })
  return i18n.global.t('createChat.title')
}

// Markers for the former greeting, first-person self-introduction, or product name.
const FORBIDDEN_TITLE_MARKERS: RegExp[] = [
  /WeKnora/i,
  /^Hi[,，]?\s/i,
  /^Привет[,]?\s/i,
  /^안녕하세요[,]?\s/i,
  /^我(?:是)?/,
  /^я\s/i,
  /입니다/,
]

test('new-conversation empty-state title is simplified in every supported locale', () => {
  for (const { name, messages } of SUPPORTED_LOCALES) {
    const title = resolveNewConversationTitle(name, messages)

    assert.ok(title.length > 0, `${name} new-conversation title should not be empty`)
    for (const marker of FORBIDDEN_TITLE_MARKERS) {
      assert.doesNotMatch(
        title,
        marker,
        `${name} title should drop the greeting, self-introduction, and product name`,
      )
    }
  }
})

test('Simplified Chinese new-conversation title is exactly the product slogan', () => {
  const title = resolveNewConversationTitle('zh-CN', zhCN)
  assert.equal(title, '让你的知识触手可及')
})

test('the empty-state component keeps the suggested questions and composer', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')

  assert.match(source, /suggested-questions-container/)
  assert.match(source, /<InputField\b/)
})
