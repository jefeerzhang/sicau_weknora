package types

// PlatformIdentity is the unified platform identity classification shown as
// the user identity label. It is independent of any workspace membership
// role (Owner/Admin/Contributor/Viewer).
type PlatformIdentity string

const (
	// PlatformIdentitySuperAdmin marks the composite platform authority.
	PlatformIdentitySuperAdmin PlatformIdentity = "superadmin"
	// PlatformIdentityTeacher marks an appointed teaching identity.
	PlatformIdentityTeacher PlatformIdentity = "teacher"
	// PlatformIdentityStudent is the default non-teaching identity.
	PlatformIdentityStudent PlatformIdentity = "student"
	// PlatformIdentityUnset marks missing or unrecognizable identity data.
	PlatformIdentityUnset PlatformIdentity = "unset"
)
