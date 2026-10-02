export type ShellIdentity = 'student' | 'teacher' | 'superadmin'

/**
 * Workspace-scoped role key. Still used by a few toolbox/menu helpers that
 * combine platform identity with tenant role. Platform identity is authoritative
 * for student lockdown; workspace role is not a way for students to unlock
 * deployment panels.
 */
export type SettingsRoleKey = 'viewer' | 'contributor' | 'admin' | 'owner'

/**
 * Settings the signed-in person can see, by platform identity.
 *
 * This is the frontend source of truth for the settings nav and the avatar
 * shortcuts that lead into it. Workspace owner/admin/viewer stay on the
 * server. Hiding a section is not authorization.
 */
const STUDENT_SETTINGS = ['general', 'userprofile', 'mymemory', 'mymodels'] as const

const TEACHER_SETTINGS = [
  ...STUDENT_SETTINGS,
  'tenant',
  'members',
  'chathistory',
  'memory',
  'models',
  'envvars',
] as const

const DEPLOYMENT_SETTINGS = [
  'ollama',
  'weknoracloud',
  'websearch',
  'vectorstore',
  'parser',
  'storage',
  'sandbox',
  'skills',
  'mcp',
  'browserconnection',
  'integration-im',
  'integration-embed',
  'integration-api',
  'integration-chrome',
  'integration-claw',
] as const

export const SYSTEM_ADMIN_SETTINGS_SECTIONS = new Set([
  'user-directory',
  'system-global',
  'runtime-queues',
  'platform-api-keys',
  'system-audit-log',
])

const SUPERADMIN_SETTINGS = [
  ...TEACHER_SETTINGS,
  ...SYSTEM_ADMIN_SETTINGS_SECTIONS,
  ...DEPLOYMENT_SETTINGS,
] as const

/**
 * Legacy workspace-role floor for sections that still consult tenant role
 * (e.g. toolbox while sidebar identity wiring lands). Prefer
 * canSeeSettingsSection(shellIdentity, ...) for new call sites.
 */
export const SETTINGS_SECTION_MIN_ROLE: Record<string, SettingsRoleKey> = {
  general: 'viewer',
  ollama: 'admin',
  weknoracloud: 'admin',
  models: 'viewer',
  websearch: 'admin',
  chathistory: 'admin',
  vectorstore: 'admin',
  parser: 'admin',
  storage: 'admin',
  sandbox: 'admin',
  skills: 'admin',
  mcp: 'admin',
  system: 'viewer',
  userprofile: 'viewer',
  browserconnection: 'viewer',
  tenant: 'viewer',
  members: 'viewer',
  mymemory: 'viewer',
  mymodels: 'viewer',
  memory: 'admin',
  envvars: 'viewer',
}

export const SETTINGS_MANAGEMENT_SHORTCUT_MIN_ROLE = {
  members: 'owner',
  models: 'admin',
} as const satisfies Record<string, SettingsRoleKey>

export function shellIdentityOf(user?: {
  platform_identity?: string
  is_system_admin?: boolean
  is_teacher?: boolean
} | null): ShellIdentity {
  if (user?.is_system_admin === true || user?.platform_identity === 'superadmin') {
    return 'superadmin'
  }
  if (user?.is_teacher === true || user?.platform_identity === 'teacher') {
    return 'teacher'
  }
  return 'student'
}

export function visibleSettingsSections(identity: ShellIdentity): readonly string[] {
  if (identity === 'superadmin') return SUPERADMIN_SETTINGS
  if (identity === 'teacher') return TEACHER_SETTINGS
  return STUDENT_SETTINGS
}

export function canSeeSettingsSection(identity: ShellIdentity, section: string): boolean {
  return visibleSettingsSections(identity).includes(section)
}

/** Students use 新对话. 智能体 stays on the teacher and superadmin sidebars. */
export function sidebarShowsAgents(identity: ShellIdentity): boolean {
  return identity !== 'student'
}

/** Artifact library is Contributor+ on the API; hide the nav for students. */
export function sidebarShowsArtifacts(identity: ShellIdentity): boolean {
  return identity !== 'student'
}

/** Toolbox (skills / MCP / browser) is deployment-side; students never see it. */
export function sidebarShowsToolbox(identity: ShellIdentity): boolean {
  return identity !== 'student'
}

/**
 * Knowledge-base list's second rail (全部 / 我的 / 收藏 / 最近 / 共享空间).
 * Favorites and recents are not a reason to keep it. Students never see it;
 * superadmins always do; everyone else only after joining a shared space.
 */
export function showKnowledgeSpaceRail(identity: ShellIdentity, sharedSpaceCount: number): boolean {
  if (identity === 'student') return false
  if (identity === 'superadmin') return true
  return sharedSpaceCount > 0
}
