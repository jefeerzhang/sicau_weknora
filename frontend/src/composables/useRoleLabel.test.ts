import assert from 'node:assert/strict'
import test from 'node:test'
import { createI18n } from 'vue-i18n'

import zhCN from '../i18n/locales/zh-CN.ts'
import { formatRoleLabel } from './formatRoleLabel.ts'

function tZh(): (key: string) => string {
  const i18n = createI18n({
    legacy: false,
    locale: 'zh-CN',
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    messages: { 'zh-CN': zhCN } as any,
  })
  return (key: string) => String(i18n.global.t(key))
}

test('formatRoleLabel maps the workspace owner to 教师', () => {
  const t = tZh()
  assert.equal(formatRoleLabel(t, 'owner'), '教师')
})

test('formatRoleLabel maps a superadmin owner to 超级管理员', () => {
  const t = tZh()
  assert.equal(formatRoleLabel(t, 'owner', 'superadmin'), '超级管理员')
})

test('formatRoleLabel maps viewer to 学生', () => {
  const t = tZh()
  assert.equal(formatRoleLabel(t, 'viewer'), '学生')
})
