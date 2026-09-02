import type { TenantRole } from '@/api/tenant/members'

/**
 * Teaching-deployment presentation of a workspace membership (#17).
 *
 * Internal RBAC still uses owner/admin/contributor/viewer. Education UI
 * only admits two active relationships — workspace lead (owner) and
 * student (viewer). Legacy admin/contributor and ambiguous owner counts
 * surface as a non-editable warning rather than being relabeled as
 * Teacher or Student.
 */
export type TeachingMembershipKind = 'lead' | 'student' | 'legacy'

export interface TeachingMembershipView {
  kind: TeachingMembershipKind
  /** i18n key under tenantMember.* for the primary label */
  labelKey: string
  /** When kind === 'legacy', warning key for the inline hint */
  warningKey?: string
}

/**
 * Classify a membership row for the education roster.
 *
 * @param role - Internal tenant role from the API
 * @param ownerCount - Number of active owner rows visible for this workspace.
 *   Ambiguous owner count (0 or >1) is treated as legacy so the UI never
 *   invents a single workspace lead.
 */
export function teachingMembershipView(
  role: TenantRole | string | null | undefined,
  ownerCount = 1,
): TeachingMembershipView {
  if (role === 'viewer') {
    return { kind: 'student', labelKey: 'tenantMember.teaching.student' }
  }
  if (role === 'owner') {
    if (ownerCount === 1) {
      return { kind: 'lead', labelKey: 'tenantMember.teaching.lead' }
    }
    return {
      kind: 'legacy',
      labelKey: 'tenantMember.teaching.legacyOwner',
      warningKey: 'tenantMember.teaching.legacyWarning',
    }
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
