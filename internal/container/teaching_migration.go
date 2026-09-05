package container

import (
	"context"

	"github.com/Tencent/WeKnora/internal/application/service"
	"gorm.io/gorm"
)

// ensureTeachingMigrationState runs the teaching data migration phase and
// records the outcome in the shared blocked-state cache. Startup invokes it
// in every deployment mode — after application migrations, and after
// externally applied migrations alike (#22).
//
// A failure is propagated to the caller (#24): initDatabase returns the
// error so the process fails closed instead of serving teaching permissions
// that may still carry un-normalized elevated memberships. Docker restarts
// the container (compose healthcheck / Helm probes watch /health, which
// also reports the blocked state), and the next startup retries; committed
// state transitions are never re-audited.
func ensureTeachingMigrationState(ctx context.Context, db *gorm.DB) error {
	phase := service.NewTeachingMigrationPhase(db, nil)
	// RunAndRecord already logs the failure once and caches the blocked
	// state; the returned error only propagates the fail-closed signal.
	_, err := phase.RunAndRecord(ctx)
	return err
}
