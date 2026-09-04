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

test('formatRoleLabel maps single owner to 空间负责人', () => {
  const t = tZh()
  assert.equal(formatRoleLabel(t, 'owner', 1), '空间负责人')
})

test('formatRoleLabel maps viewer to 学生', () => {
  const t = tZh()
  assert.equal(formatRoleLabel(t, 'viewer'), '学生')
})

test('formatRoleLabel does not label ambiguous owner counts as 空间负责人', () => {
  const t = tZh()
  assert.notEqual(formatRoleLabel(t, 'owner', 0), '空间负责人')
  assert.notEqual(formatRoleLabel(t, 'owner', 2), '空间负责人')
  assert.equal(formatRoleLabel(t, 'owner', 0), '历史负责人（待处理）')
  assert.equal(formatRoleLabel(t, 'owner', 2), '历史负责人（待处理）')
})
