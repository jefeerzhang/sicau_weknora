import type { TenantRole } from '@/api/tenant/members'

/**
 * Teaching-deployment presentation of a workspace membership.
 *
 * Internal RBAC still uses owner/admin/contributor/viewer. The course
 * roster only shows three labels: the workspace owner is a teacher
 * (or a super administrator when the API says so), invited members are
 * students. Legacy admin/contributor rows stay a non-editable warning.
 */
export type TeachingMembershipKind = 'teacher' | 'superadmin' | 'student' | 'legacy'

export interface TeachingMembershipView {
  kind: TeachingMembershipKind
  /** i18n key under tenantMember.* for the primary label */
  labelKey: string
  /** When kind === 'legacy', warning key for the inline hint */
  warningKey?: string
}

/**
 * Classify a membership row for the course roster.
 *
 * Owner rows are teachers unless rosterIdentity is superadmin. Viewers
 * are students. rosterIdentity comes from the list API so a super
 * administrator who owns the space is not relabeled as a plain teacher.
 */
export function teachingMembershipView(
  role: TenantRole | string | null | undefined,
  rosterIdentity?: string | null,
): TeachingMembershipView {
  if (rosterIdentity === 'superadmin') {
    return { kind: 'superadmin', labelKey: 'tenantMember.teaching.superadmin' }
  }
  if (rosterIdentity === 'teacher' || role === 'owner') {
    return { kind: 'teacher', labelKey: 'tenantMember.teaching.teacher' }
  }
  if (rosterIdentity === 'student' || role === 'viewer') {
    return { kind: 'student', labelKey: 'tenantMember.teaching.student' }
  }
  if (role === 'admin' || role === 'contributor') {
    return {
      kind: 'legacy',
      labelKey: `tenantMember.role.${role}`,
      warningKey: 'tenantMember.teaching.legacyWarning',
    }
  }
  return {
    kind: 'legacy',
    labelKey: 'tenantMember.teaching.unknown',
    warningKey: 'tenantMember.teaching.legacyWarning',
  }
}

/**
 * Count owners in a member list (skips non-active rows when status is set).
 */
export function countOwners(
  members: Array<{ role?: string; status?: string }>,
): number {
  return members.filter((m) => {
    if (m.role !== 'owner') return false
    if (m.status && m.status !== 'active') return false
    return true
  }).length
}
