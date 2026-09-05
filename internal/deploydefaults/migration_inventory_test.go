package deploydefaults_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Empty-DB drill companion: pin the teaching/schema SQL inventory that a
// fresh AUTO_MIGRATE=true boot on the v0.8.0 sync line must be able to apply.
// This is not a live compose up; it fails closed if renames drop files.

func TestTeachingMigrationSQLPresent(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	required := []string{
		"migrations/versioned/000091_tenant_default_agent.up.sql",
		"migrations/versioned/000094_platform_identity_flags.up.sql",
		"migrations/versioned/000097_tenant_notes.up.sql",
		"migrations/versioned/000098_announcements.up.sql",
		"migrations/sqlite/000013_platform_identity_flags.up.sql",
		"migrations/sqlite/000016_tenant_default_agent.up.sql",
		"migrations/sqlite/000017_tenant_notes.up.sql",
		"migrations/sqlite/000018_announcements.up.sql",
	}
	for _, rel := range required {
		p := filepath.Join(root, rel)
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("missing teaching migration %s: %v", rel, err)
		}
		b, err := os.ReadFile(p)
		if err != nil || len(strings.TrimSpace(string(b))) == 0 {
			t.Fatalf("teaching migration %s empty or unreadable", rel)
		}
	}
}
