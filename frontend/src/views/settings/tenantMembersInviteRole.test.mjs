import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import test from 'node:test'

const here = dirname(fileURLToPath(import.meta.url))
const src = readFileSync(join(here, 'TenantMembers.vue'), 'utf8')

test('share-link and email invite default to viewer (student)', () => {
  assert.match(src, /shareLinkForm = reactive<\{ role: TenantRole \}>\(\{ role: 'viewer' \}\)/)
  assert.match(src, /role: 'viewer',\s*\n\}\)/)
  assert.match(src, /shareLinkForm\.role = 'viewer'/)
  assert.match(src, /addForm\.role = 'viewer'/)
  assert.doesNotMatch(src, /shareLinkForm\.role = 'contributor'/)
  assert.doesNotMatch(src, /shareLinkForm = reactive<\{ role: TenantRole \}>\(\{ role: 'contributor' \}\)/)
})

test('invite / share-link selectors use viewer-only options', () => {
  assert.match(src, /inviteRoleOptions/)
  assert.match(src, /:options="inviteRoleOptions"/)
  assert.match(
    src,
    /const inviteRoleOptions = computed\(\(\) => \[\s*\{ label: t\('tenantMember\.role\.viewer'\), value: 'viewer' \},\s*\]\)/,
  )
})
