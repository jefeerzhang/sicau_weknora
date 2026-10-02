import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const root = new URL('./', import.meta.url)
const read = (rel) => readFileSync(new URL(rel, root), 'utf8')

test('students have no browser selector, preview, or enabled browser request', () => {
  const input = read('./Input-field.vue')
  const chat = read('../views/chat/index.vue')
  for (const src of [input, chat]) {
    assert.match(src, /canUseLocalBrowser = computed\(\(\) => shellIdentityOf\(authStore\.user\) !== 'student' && authStore\.hasRole\('contributor'\)\)/)
  }
  assert.match(input, /v-if="canUseLocalBrowser && browserConnection\.enabled && settingsStore\.isAgentStreamMode"/)
  assert.match(input, /if \(canUseLocalBrowser\.value\) browserConnection\.watchStatus\(\)/)
  assert.match(chat, /BrowserTaskPreview v-if="session_id && canUseLocalBrowser"/)
  assert.match(chat, /local_browser_enabled: canUseLocalBrowser\.value &&/)
})

test('viewer cannot see chat attachment upload control', () => {
  const src = read('./Input-field.vue')
  assert.match(src, /attachmentsBlocked = !authStore\.hasRole\('contributor'\)/)
  assert.match(src, /v-if="authStore\.hasRole\('contributor'\)"[^>]*placement="top"[^>]*input-field-tooltip/)
  assert.match(src, /applyWorkspaceDefaultAgent/)
  assert.match(src, /getDefaultAgentId/)
})

test('answer toolbars hide add-to-knowledge for students', () => {
  const bot = read('../views/chat/components/botmsg.vue')
  const stream = read('../views/chat/components/AgentStreamDisplay.vue')
  assert.match(bot, /canMutateCourseFiles/)
  assert.match(stream, /canMutateCourseFiles/)
  assert.match(bot, /v-if="canMutateCourseFiles"[^>]*handleAddToKnowledge/)
  assert.match(stream, /v-if="canMutateCourseFiles"[^>]*handleAddToKnowledge/)
})

test('artifact drawer gates downloads to contributor+', () => {
  const src = read('../views/chat/components/ChatArtifactsDrawer.vue')
  assert.match(src, /canDownloadFiles/)
  assert.match(src, /hasRole\('contributor'\)/)
  assert.match(src, /v-if="canDownloadFiles"/)
  assert.match(src, /downloadDisabled/)
})

test('tenant API exposes workspace default-agent KV helpers', () => {
  const src = read('../api/tenant/index.ts')
  assert.match(src, /\/api\/v1\/tenants\/kv\/default-agent-id/)
  assert.match(src, /export async function getDefaultAgentId/)
  assert.match(src, /export async function putDefaultAgentId/)
})
