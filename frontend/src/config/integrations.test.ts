import assert from 'node:assert/strict'
import test from 'node:test'

import {
  INTEGRATION_PREVIEW_ITEMS,
  INTEGRATION_TAB_CAPABILITY,
  INTEGRATION_TAB_MIN_ROLE,
  INTEGRATION_TABS,
} from './integrations'

test('sicau-v1: publish-integration tabs require contributor+ (students sealed)', () => {
  // ADR-009-7 / issue #5: IM, embed, Chrome, Claw are teacher-side only.
  // API stays owner (unchanged by this ticket).
  for (const tab of ['im', 'embed', 'chrome', 'claw'] as const) {
    assert.equal(INTEGRATION_TAB_MIN_ROLE[tab], 'contributor', tab)
  }
  assert.equal(INTEGRATION_TAB_MIN_ROLE.api, 'owner')
})

test('publish-integration preview list stays intact for teachers (not emptied)', () => {
  // Issue #3 baseline: do not seal students by clearing the whole preview list.
  assert.deepEqual(
    INTEGRATION_PREVIEW_ITEMS.map((item) => item.key),
    INTEGRATION_TABS,
  )
  assert.equal(INTEGRATION_TAB_CAPABILITY.im, 'integrations.im')
  assert.equal(INTEGRATION_TAB_CAPABILITY.embed, 'integrations.embed')
  assert.equal(INTEGRATION_TAB_CAPABILITY.api, 'integrations.api')
})
