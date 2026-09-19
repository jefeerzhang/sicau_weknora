import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import test from 'node:test'
import { fileURLToPath } from 'node:url'
import { createSSRApp, h } from 'vue'
import { renderToString } from '@vue/server-renderer'
import { createI18n } from 'vue-i18n'

import zhCN from '../../i18n/locales/zh-CN.ts'
import enUS from '../../i18n/locales/en-US.ts'
import ruRU from '../../i18n/locales/ru-RU.ts'
import koKR from '../../i18n/locales/ko-KR.ts'
import { NewConversationEmptyState } from './newConversationEmptyState.ts'

const here = dirname(fileURLToPath(import.meta.url))

const SUPPORTED_LOCALES = [
  { name: 'zh-CN', messages: zhCN },
  { name: 'en-US', messages: enUS },
  { name: 'ru-RU', messages: ruRU },
  { name: 'ko-KR', messages: koKR },
]

async function renderEmptyState(locale: string, messages: unknown): Promise<string> {
  const i18n = createI18n({
    legacy: false,
    locale,
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    messages: { [locale]: messages } as any,
  })

  const app = createSSRApp({
    setup() {
      return () =>
        h(
          NewConversationEmptyState,
          {},
          {
            questions: () =>
              h('div', { class: 'suggested-questions-container' }, [
                h('div', { class: 'suggested-questions-inner' }, 'q'),
              ]),
            composer: () => h('div', { 'data-testid': 'composer-stub' }, 'composer'),
          },
        )
    },
  })
  app.use(i18n)
  return renderToString(app)
}

test('new-conversation empty state does not render the slogan as a page title', async () => {
  for (const { name, messages } of SUPPORTED_LOCALES) {
    const html = await renderEmptyState(name, messages)
    assert.doesNotMatch(html, /dialogue-title/, `${name} should not render a centered title`)
    assert.doesNotMatch(html, /让你的知识触手可及/, `${name} should not render the slogan`)
    assert.match(html, /suggested-questions-container/)
    assert.match(html, /data-testid="composer-stub"/)
  }
})

test('creatChat.vue wires the shared empty-state shell', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')
  assert.match(source, /NewConversationEmptyState/)
  assert.match(source, /suggested-questions-container/)
  assert.match(source, /<InputField\b/)
})

test('empty-state layout pins questions and composer to the bottom', () => {
  const lessPath = join(here, 'newConversationEmptyState.less')
  const less = readFileSync(lessPath, 'utf8')
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

test('creatChat.vue does not reintroduce broken :deep dialogue-wrap layout', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-wrap\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-answers\s*\)/)
  assert.doesNotMatch(source, /:deep\(\s*\.dialogue-title\s*\)/)
  assert.match(source, /newConversationEmptyState\.less/)
})
