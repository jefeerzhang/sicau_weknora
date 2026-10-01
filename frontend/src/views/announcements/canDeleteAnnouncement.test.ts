import assert from 'node:assert/strict'
import test from 'node:test'
import { canDeleteAnnouncement } from './canDeleteAnnouncement'

test('author can delete own announcement', () => {
  assert.equal(
    canDeleteAnnouncement({
      announcementUserId: 'u-1',
      viewerUserId: 'u-1',
      isAdminOrOwner: false,
    }),
    true,
  )
})

test('other member cannot delete by coincidence of display name', () => {
  assert.equal(
    canDeleteAnnouncement({
      announcementUserId: 'u-author',
      viewerUserId: 'u-other',
      isAdminOrOwner: false,
    }),
    false,
  )
})

test('admin/owner can delete any', () => {
  assert.equal(
    canDeleteAnnouncement({
      announcementUserId: 'u-author',
      viewerUserId: 'u-admin',
      isAdminOrOwner: true,
    }),
    true,
  )
})

test('missing ids deny', () => {
  assert.equal(
    canDeleteAnnouncement({
      announcementUserId: '',
      viewerUserId: 'u-1',
      isAdminOrOwner: false,
    }),
    false,
  )
})
