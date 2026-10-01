/**
 * Announcement delete-button visibility. Backend authorizes by user_id
 * (author or admin/owner); the UI must use the same identity, not display name.
 */
export function canDeleteAnnouncement(opts: {
  announcementUserId: string
  viewerUserId: string | undefined
  isAdminOrOwner: boolean
}): boolean {
  if (opts.isAdminOrOwner) return true
  if (!opts.viewerUserId || !opts.announcementUserId) return false
  return opts.announcementUserId === opts.viewerUserId
}
