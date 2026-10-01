import type { PlatformIdentity } from '@/api/auth'

/**
 * The known platform identity values (user identity label).
 * Single source of truth for the value set shared by the user-info page and
 * the teacher-management roster.
 */
export const PLATFORM_IDENTITY_VALUES: PlatformIdentity[] = [
  'superadmin',
  'teacher',
  'student',
  'unset',
]

/**
 * Returns the vue-i18n key that localizes a platform identity label.
 *
 * Any input outside the known value set (including missing/empty/unknown)
 * maps to `unset` (身份未设置). It deliberately NEVER infers a teacher or
 * superadmin identity from a missing classification — see CONTEXT.md
 * 「身份未设置」: "出现时不能推断为教师或超级管理员".
 */
export function platformIdentityKey(identity?: string): string {
  const safe = PLATFORM_IDENTITY_VALUES.includes(identity as PlatformIdentity)
    ? (identity as PlatformIdentity)
    : 'unset'
  return `userProfile.platformIdentity.values.${safe}`
}
