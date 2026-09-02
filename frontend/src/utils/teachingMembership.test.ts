import assert from 'node:assert/strict'
import test from 'node:test'

import { countOwners, teachingMembershipView } from './teachingMembership'

test('#17 maps single owner to workspace lead', () => {
  assert.deepEqual(teachingMembershipView('owner', 1), {
    kind: 'lead',
    labelKey: 'tenantMember.teaching.lead',
  })
})

test('#17 maps viewer to student', () => {
  assert.deepEqual(teachingMembershipView('viewer'), {
    kind: 'student',
    labelKey: 'tenantMember.teaching.student',
  })
})

test('#17 does not disguise admin/contributor as student or teacher', () => {
  for (const role of ['admin', 'contributor'] as const) {
    const view = teachingMembershipView(role, 1)
    assert.equal(view.kind, 'legacy')
    assert.equal(view.labelKey, `tenantMember.role.${role}`)
    assert.equal(view.warningKey, 'tenantMember.teaching.legacyWarning')
  }
})

test('#17 treats zero or multiple owners as legacy (ambiguous lead)', () => {
  assert.equal(teachingMembershipView('owner', 0).kind, 'legacy')
  assert.equal(teachingMembershipView('owner', 2).kind, 'legacy')
  assert.equal(
    teachingMembershipView('owner', 2).warningKey,
    'tenantMember.teaching.legacyWarning',
  )
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
