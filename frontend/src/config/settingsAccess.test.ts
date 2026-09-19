import assert from 'node:assert/strict'
import test from 'node:test'

import {
  SYSTEM_ADMIN_SETTINGS_SECTIONS,
  canSeeSettingsSection,
  shellIdentityOf,
  showKnowledgeSpaceRail,
  sidebarShowsAgents,
  visibleSettingsSections,
} from './settingsAccess'

const STUDENT = ['general', 'userprofile', 'mymemory']
const TEACHER_EXTRA = ['tenant', 'members', 'chathistory', 'memory', 'models', 'envvars']
const DEPLOYMENT = [
  'ollama',
  'weknoracloud',
  'websearch',
  'vectorstore',
  'parser',
  'storage',
  'sandbox',
  'skills',
  'mcp',
  'integration-im',
  'integration-embed',
  'integration-api',
  'integration-chrome',
  'integration-claw',
]
const SYSTEM_ADMIN = [
  'user-directory',
  'system-global',
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
]

function sorted(values: readonly string[]) {
  return [...values].sort()
}

test('unset and student identities see only the account settings', () => {
  assert.equal(shellIdentityOf(undefined), 'student')
  assert.equal(shellIdentityOf({ platform_identity: 'unset' }), 'student')
  assert.equal(shellIdentityOf({ platform_identity: 'student' }), 'student')
  assert.deepEqual(sorted(visibleSettingsSections('student')), sorted(STUDENT))
  assert.equal(sidebarShowsAgents('student'), false)
  for (const hidden of [...TEACHER_EXTRA, ...DEPLOYMENT, ...SYSTEM_ADMIN]) {
    assert.equal(canSeeSettingsSection('student', hidden), false, hidden)
  }
})

test('a teacher sees course tools, not the deployment catalog or system admin', () => {
  assert.equal(shellIdentityOf({ platform_identity: 'teacher' }), 'teacher')
  assert.equal(shellIdentityOf({ is_teacher: true, platform_identity: 'unset' }), 'teacher')
  assert.deepEqual(sorted(visibleSettingsSections('teacher')), sorted([...STUDENT, ...TEACHER_EXTRA]))
  assert.equal(sidebarShowsAgents('teacher'), true)
  assert.equal(canSeeSettingsSection('teacher', 'models'), true)
  assert.equal(canSeeSettingsSection('teacher', 'envvars'), true)
  assert.equal(canSeeSettingsSection('teacher', 'ollama'), false)
  assert.equal(canSeeSettingsSection('teacher', 'sandbox'), false)
  assert.equal(canSeeSettingsSection('teacher', 'user-directory'), false)
})

test('a superadmin sees the teacher catalog plus system admin and deployment', () => {
  assert.equal(shellIdentityOf({ platform_identity: 'superadmin' }), 'superadmin')
  assert.equal(
    shellIdentityOf({ is_system_admin: true, is_teacher: true, platform_identity: 'teacher' }),
    'superadmin',
  )
  assert.deepEqual(
    sorted(visibleSettingsSections('superadmin')),
    sorted([...STUDENT, ...TEACHER_EXTRA, ...SYSTEM_ADMIN, ...DEPLOYMENT]),
  )
  assert.equal(sidebarShowsAgents('superadmin'), true)
  for (const key of SYSTEM_ADMIN) {
    assert.equal(canSeeSettingsSection('superadmin', key), true, key)
    assert.equal(SYSTEM_ADMIN_SETTINGS_SECTIONS.has(key), true, key)
  }
  assert.equal(canSeeSettingsSection('superadmin', 'ollama'), true)
  assert.equal(canSeeSettingsSection('superadmin', 'integration-im'), true)
})

test('knowledge-base space rail follows shared spaces, not favorites', () => {
  assert.equal(showKnowledgeSpaceRail('student', 0), false)
  assert.equal(showKnowledgeSpaceRail('student', 3), false)
  assert.equal(showKnowledgeSpaceRail('teacher', 0), false)
  assert.equal(showKnowledgeSpaceRail('teacher', 1), true)
  assert.equal(showKnowledgeSpaceRail('superadmin', 0), true)
})
