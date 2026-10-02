package deploydefaults_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Empty-DB drill companion: pin the teaching/schema SQL inventory that a
// fresh AUTO_MIGRATE=true boot on the v0.8.2 sync line must be able to apply.
// This is not a live compose up; it fails closed if renames drop files.
//
// Teaching migrations live at versioned 111-116 / sqlite 31-36 because the
// upstream v0.8.2 rebase took 092-098 and 13-18; see 000117 for the schema
// replay that covers databases which already advanced on the old slots.

func TestTeachingMigrationSQLPresent(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))

	required := []string{
		"migrations/versioned/000111_tenant_default_agent.up.sql",
		"migrations/versioned/000112_tenant_notes.up.sql",
		"migrations/versioned/000113_announcements.up.sql",
		"migrations/versioned/000114_platform_identity_flags.up.sql",
		"migrations/versioned/000115_workspace_ownership_anomalies.up.sql",
		"migrations/versioned/000116_single_active_share_link.up.sql",
		"migrations/versioned/000117_replay_upstream_schema_behind_teaching_slots.up.sql",
		"migrations/sqlite/000031_tenant_default_agent.up.sql",
		"migrations/sqlite/000032_tenant_notes.up.sql",
		"migrations/sqlite/000033_announcements.up.sql",
		"migrations/sqlite/000034_platform_identity_flags.up.sql",
		"migrations/sqlite/000035_workspace_ownership_anomalies.up.sql",
		"migrations/sqlite/000036_single_active_share_link.up.sql",
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
