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

const FORBIDDEN_TITLE_MARKERS: RegExp[] = [
  /WeKnora/i,
  /^Hi[,，]?\s/i,
  /^Привет[,]?\s/i,
  /^안녕하세요[,]?\s/i,
  /^我(?:是)?/,
  /^я\s/i,
  /입니다/,
]

async function renderEmptyState(locale: string, messages: unknown): Promise<{
  title: string
  html: string
}> {
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
  const html = await renderToString(app)

  const titleMatch = html.match(/class="dialogue-title"[^>]*>[\s\S]*?<span[^>]*>([\s\S]*?)<\/span>/)
  const title = (titleMatch?.[1] ?? '').replace(/<!--[\s\S]*?-->/g, '').trim()

  return { title, html }
}

test('new-conversation empty-state title is simplified in every supported locale', async () => {
  for (const { name, messages } of SUPPORTED_LOCALES) {
    const { title } = await renderEmptyState(name, messages)
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

test('Simplified Chinese new-conversation title is exactly the product slogan', async () => {
  const { title } = await renderEmptyState('zh-CN', zhCN)
  assert.equal(title, '让你的知识触手可及')
})

test('rendered empty-state keeps suggested questions and composer seams', async () => {
  const { html } = await renderEmptyState('zh-CN', zhCN)
  assert.match(html, /suggested-questions-container/)
  assert.match(html, /data-testid="composer-stub"/)
})

test('creatChat.vue wires the shared empty-state shell', () => {
  const source = readFileSync(join(here, 'creatChat.vue'), 'utf8')
  assert.match(source, /NewConversationEmptyState/)
  assert.match(source, /suggested-questions-container/)
  assert.match(source, /<InputField\b/)
})
