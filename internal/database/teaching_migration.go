package database

import (
	"sync"
)

// Teaching data migration state cache (#22). Mirrors the schema-migration
// state in migration.go so the system info endpoint can surface a blocked
// teaching service: while the message is non-empty the deployment must be
// treated as not-yet-upgraded because legacy elevated memberships or
// pending invitations may remain.
var (
	teachingMigrationStateMu sync.RWMutex
	teachingMigrationError   string
)

// CacheTeachingMigrationError records the outcome of the most recent
// teaching data migration attempt (startup phase or SuperAdmin manual
// retry). An empty message means success; a non-empty message is the
// human-readable failure reason. Messages must never embed invitation
// tokens or credentials — they are rendered by /system/info.
func CacheTeachingMigrationError(msg string) {
	teachingMigrationStateMu.Lock()
	defer teachingMigrationStateMu.Unlock()
	teachingMigrationError = msg
}

// CachedTeachingMigrationError returns the failure reason recorded for the
// most recent teaching data migration attempt. Empty string means the
// migration either succeeded or never ran (fresh deployment before
// startup phases).
func CachedTeachingMigrationError() string {
	teachingMigrationStateMu.RLock()
	defer teachingMigrationStateMu.RUnlock()
	return teachingMigrationError
}
