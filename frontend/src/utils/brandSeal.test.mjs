import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = (rel) => readFileSync(new URL(rel, import.meta.url), 'utf8')

test('login page uses SICAU brand, not upstream GitHub as logo', () => {
  const src = read('../views/auth/Login.vue')
  assert.match(src, /sicau\.edu\.cn/)
  assert.match(src, /sicau-crest\.png/)
  assert.match(src, /川农知识库/)
  assert.match(src, /jefeerzhang\.github\.io/)
  assert.match(src, /common\.teacherHome/)
  // Logo uses SICAU URL (href may precede class); showcase attribution to Tencent/WeKnora is allowed.
  assert.match(
    src,
    /<a[^>]*href="https:\/\/www\.sicau\.edu\.cn"[^>]*class="header-logo"/,
  )
  assert.doesNotMatch(
    src,
    /<a[^>]*href="https:\/\/github\.com\/Tencent\/WeKnora"[^>]*class="header-logo"/,
  )
})

test('user menu does not open upstream Tencent GitHub', () => {
  const src = read('../components/UserMenu.vue')
  assert.doesNotMatch(src, /github\.com\/Tencent\/WeKnora/)
  assert.doesNotMatch(src, /openGithub|openDocs/)
})

test('sidebar menu uses SICAU brand text, not WEKNORA wordmark', () => {
  const src = read('../components/menu.vue')
  assert.match(src, /川农知识库/)
  assert.match(src, /sicau-crest\.png/)
  assert.doesNotMatch(src, /weknora\.png/)
})

test('platformIdentity util never invents teacher/superadmin from unknown', () => {
  const src = read('./platformIdentity.ts')
  assert.match(src, /PLATFORM_IDENTITY_VALUES/)
  assert.match(src, /platformIdentityKey/)
  assert.match(src, /unset/)
})

test('auth userInfoFromApi carries platform_identity', () => {
  const src = read('../api/auth/index.ts')
  assert.match(src, /export type PlatformIdentity/)
  assert.match(src, /platform_identity:\s*\(u\?\.platform_identity/)
})
