package container

import (
	"context"
	"fmt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/gorm"
)

// requiredTeachingMigrationTables must exist before the teaching data
// migration is allowed to run. With AUTO_MIGRATE=false the schema is
// applied by an external tool; running the migration against a half-applied
// schema would surface as raw SQL errors instead of an actionable blocked
// state, so the phase verifies first and fails closed (#22).
var requiredTeachingMigrationTables = []string{
	"tenants",
	"tenant_members",
	"tenant_invitations",
	"workspace_ownership_anomalies",
	"audit_logs",
}

// teachingMigrationRunner narrows the migrator surface used by the startup
// phase so tests can inject run outcomes without a database.
type teachingMigrationRunner interface {
	Run(ctx context.Context) (*types.TeachingRoleMigrationReport, error)
}

// verifyTeachingMigrationTables checks that every table the teaching data
// migration reads or writes exists.
func verifyTeachingMigrationTables(db *gorm.DB) error {
	for _, table := range requiredTeachingMigrationTables {
		if !db.Migrator().HasTable(table) {
			return fmt.Errorf(
				"teaching data migration requires table %q; apply the schema migrations first and restart",
				table)
		}
	}
	return nil
}

// runTeachingDataMigration executes one teaching normalization pass:
// pending legacy invitations are downgraded to viewer and legacy
// admin/contributor memberships are demoted with per-item success audits
// (#21/#22). A non-nil error means the deployment must not be advertised
// as upgraded; committed state transitions are never re-audited on rerun
// (guarded updates) and failed items retry on the next startup.
func runTeachingDataMigration(ctx context.Context, db *gorm.DB, runner teachingMigrationRunner) error {
	if err := verifyTeachingMigrationTables(db); err != nil {
		return err
	}
	if runner == nil {
		runner = service.NewTeachingRoleMigrator(db, nil)
	}
	report, err := runner.Run(ctx)
	if err != nil {
		return err
	}
	if report.Failed > 0 {
		return fmt.Errorf(
			"teaching data migration left %d item(s) unresolved; committed items stay normalized, retry on next startup",
			report.Failed)
	}
	if report.Downgraded > 0 || report.InvitationsDowngraded > 0 || len(report.AnomalyTenantIDs) > 0 {
		logger.Infof(ctx,
			"Teaching data migration: downgraded=%d invitations=%d skipped=%d anomalies=%d",
			report.Downgraded, report.InvitationsDowngraded, report.Skipped, len(report.AnomalyTenantIDs))
	}
	return nil
}

// ensureTeachingMigrationState runs the teaching data migration and caches
// the outcome for the system info endpoint. Startup invokes it in every
// deployment mode — after application migrations, and after externally
// applied migrations alike — so security invariants never depend on who
// ran the schema (#22).
func ensureTeachingMigrationState(ctx context.Context, db *gorm.DB) {
	if err := runTeachingDataMigration(ctx, db, nil); err != nil {
		database.CacheTeachingMigrationError(err.Error())
		logger.Errorf(ctx, "Teaching data migration failed (recorded as blocked state): %v", err)
		return
	}
	database.CacheTeachingMigrationError("")
}

