import assert from 'node:assert/strict'
import test from 'node:test'

import { countOwners, teachingMembershipView } from './teachingMembership'

test('owner is a teacher unless the roster says superadmin', () => {
  assert.deepEqual(teachingMembershipView('owner'), {
    kind: 'teacher',
    labelKey: 'tenantMember.teaching.teacher',
  })
  assert.deepEqual(teachingMembershipView('owner', 'superadmin'), {
    kind: 'superadmin',
    labelKey: 'tenantMember.teaching.superadmin',
  })
})

test('viewer is a student even without a roster label', () => {
  assert.deepEqual(teachingMembershipView('viewer'), {
    kind: 'student',
    labelKey: 'tenantMember.teaching.student',
  })
})

test('does not disguise admin/contributor as student or teacher', () => {
  for (const role of ['admin', 'contributor'] as const) {
    const view = teachingMembershipView(role)
    assert.equal(view.kind, 'legacy')
    assert.equal(view.labelKey, `tenantMember.role.${role}`)
    assert.equal(view.warningKey, 'tenantMember.teaching.legacyWarning')
  }
})

test('countOwners counts only active owner rows when status is present', () => {
  assert.equal(
    countOwners([
      { role: 'owner', status: 'active' },
      { role: 'owner', status: 'removed' },
      { role: 'viewer', status: 'active' },
      { role: 'admin', status: 'active' },
    ]),
    1,
  )
})
