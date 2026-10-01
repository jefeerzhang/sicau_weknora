import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const read = (rel) => readFileSync(new URL(rel, import.meta.url), 'utf8')

test('login page speaks as 川农知识库, not a WeKnora product pitch', () => {
  const src = read('../views/auth/Login.vue')
  const zh = read('../i18n/locales/zh-CN.ts')
  assert.match(src, /sicau\.edu\.cn/)
  assert.match(src, /sicau-crest\.png/)
  assert.match(src, /川农知识库/)
  assert.match(src, /jefeerzhang\.github\.io/)
  assert.match(src, /common\.teacherHome/)
  assert.match(
    src,
    /<a[^>]*href="https:\/\/www\.sicau\.edu\.cn"[^>]*class="header-logo"/,
  )
  assert.doesNotMatch(src, /github\.com\/Tencent\/WeKnora/)
  assert.doesNotMatch(src, /weknora\.weixin\.qq\.com/)
  assert.doesNotMatch(src, /loginFeature/)
  assert.doesNotMatch(src, /platform\.note/)
  assert.doesNotMatch(src, /platform\.rag\b/)
  assert.match(zh, /subtitle: '供校内课程检索教学与科研资料。通过邀请加入，不能自行注册。'/)
  assert.match(zh, /subtitle: '使用已有账户登录。'/)
  assert.match(zh, /motto: '追求真理 造福社会 自强不息'/)
  assert.match(zh, /雅安校区 · 成都校区 · 都江堰校区/)
  assert.match(src, /platform\.motto/)
  assert.doesNotMatch(zh, /firstTime: '[^']*WeKnora/)
  assert.doesNotMatch(zh, /registerSubtitle: '[^']*WeKnora/)
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
  assert.match(src, /class="logo_box" @click="router\.push\('\/platform\/creatChat'\)"/)
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
