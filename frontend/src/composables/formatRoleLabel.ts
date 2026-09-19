import { teachingMembershipView } from '../utils/teachingMembership.ts'

/**
 * Resolve a tenant role into the course-roster label.
 *
 * Pass rosterIdentity from the member list when present so a super
 * administrator who owns the space is not shown as a plain teacher.
 */
export function formatRoleLabel(
  t: (key: string) => string,
  role: string | null | undefined,
  rosterIdentity?: string | null,
): string {
  if (!role && !rosterIdentity) return ''
  const key = teachingMembershipView(role, rosterIdentity).labelKey
  const label = t(key)
  return label === key ? (role || rosterIdentity || '') : label
}
