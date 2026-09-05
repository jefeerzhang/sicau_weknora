import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const root = new URL('./', import.meta.url)
const read = (rel) => readFileSync(new URL(rel, root), 'utf8')

test('viewer cannot see chat attachment upload control', () => {
  const src = read('./Input-field.vue')
  assert.match(src, /attachmentsBlocked = !authStore\.hasRole\('contributor'\)/)
  assert.match(src, /v-if="authStore\.hasRole\('contributor'\)"[^>]*placement="top"[^>]*input-field-tooltip/)
  assert.match(src, /applyWorkspaceDefaultAgent/)
  assert.match(src, /getDefaultAgentId/)
})

test('viewer cannot see add-to-knowledge buttons', () => {
  const bot = read('../views/chat/components/botmsg.vue')
  const stream = read('../views/chat/components/AgentStreamDisplay.vue')
  assert.match(bot, /v-if="authStore\.hasRole\('contributor'\)"[^>]*handleAddToKnowledge/)
  assert.match(stream, /v-if="authStore\.hasRole\('contributor'\)"[^>]*handleAddToKnowledge/)
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
