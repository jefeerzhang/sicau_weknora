import { teachingMembershipView } from '../utils/teachingMembership.ts'

/**
 * Resolve a tenant role into the user-facing teaching label.
 *
 * `ownerCount` must be passed when the caller knows how many active owners
 * the workspace has. Ambiguous counts (0 or >1) map to the legacy owner
 * label instead of inventing a single workspace lead (#17).
 */
export function formatRoleLabel(
  t: (key: string) => string,
  role: string | null | undefined,
  ownerCount = 1,
): string {
  if (!role) return ''
  const key = teachingMembershipView(role, ownerCount).labelKey
  const label = t(key)
  return label === key ? role : label
}
