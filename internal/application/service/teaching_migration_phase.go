package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/Tencent/WeKnora/internal/database"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// The teaching data migration phase (#22/#24/#25): verify the schema it
// depends on, run the normalization, and judge success with the SAME
// semantics for both callers — the startup path (every AUTO_MIGRATE mode)
// and the SuperAdmin manual retry share this one implementation so their
// verdicts cannot drift.

// requiredTeachingMigrationTables must exist before the teaching data
// migration is allowed to run. With AUTO_MIGRATE=false the schema is
// applied by an external tool; running the migration against a half-applied
// schema would surface as raw SQL errors instead of an actionable blocked
// state, so the phase verifies first and fails closed.
var requiredTeachingMigrationTables = []string{
	"tenants",
	"tenant_members",
	"tenant_invitations",
	"workspace_ownership_anomalies",
	"audit_logs",
}

// anomalyUniqueIndexNames lists the index name each creation path uses for
// the workspace_ownership_anomalies.tenant_id unique constraint — the
// versioned SQL migration and the gorm AutoMigrate model tag respectively.
// The anomaly upsert (clause.OnConflict on tenant_id) depends on it.
var anomalyUniqueIndexNames = []string{
	"uq_workspace_ownership_anomalies_tenant",
	"idx_workspace_ownership_anomalies_tenant_id",
}

// TeachingMigrationPhase runs the post-schema teaching data normalization.
// audit is optional: the startup path wires nil (audits persist directly on
// the transaction), the manual retry path wires the DI'd audit service.
type TeachingMigrationPhase struct {
	db    *gorm.DB
	audit interfaces.AuditLogService
}

// NewTeachingMigrationPhase constructs the phase.
func NewTeachingMigrationPhase(db *gorm.DB, audit interfaces.AuditLogService) *TeachingMigrationPhase {
	return &TeachingMigrationPhase{db: db, audit: audit}
}

// VerifySchema fails closed when any required table or the anomaly unique
// index is missing. Error messages name the missing object only — never
// invitation tokens or credentials.
func (p *TeachingMigrationPhase) VerifySchema() error {
	if p == nil || p.db == nil {
		return fmt.Errorf("teaching migration phase is not configured")
	}
	migrator := p.db.Migrator()
	for _, table := range requiredTeachingMigrationTables {
		if !migrator.HasTable(table) {
			return fmt.Errorf(
				"teaching data migration requires table %q; apply the schema migrations first and restart",
				table)
		}
	}
	if !p.hasAnomalyTenantUniqueIndex() {
		return fmt.Errorf(
			"teaching data migration requires a unique index on workspace_ownership_anomalies.tenant_id; apply the schema migrations first and restart")
	}
	return nil
}

// hasAnomalyTenantUniqueIndex accepts either known index name, then falls
// back to a dialect-level scan for any unique index covering tenant_id.
func (p *TeachingMigrationPhase) hasAnomalyTenantUniqueIndex() bool {
	migrator := p.db.Migrator()
	for _, name := range anomalyUniqueIndexNames {
		if migrator.HasIndex(&types.WorkspaceOwnershipAnomaly{}, name) {
			return true
		}
	}
	indexes, err := migrator.GetIndexes(&types.WorkspaceOwnershipAnomaly{})
	if err != nil {
		// Cannot enumerate indexes on this dialect; the named checks above
		// are the verdict rather than a false pass.
		return false
	}
	for _, idx := range indexes {
		unique, ok := idx.Unique()
		if !ok || !unique {
			continue
		}
		for _, col := range idx.Columns() {
			if strings.EqualFold(col, "tenant_id") {
				return true
			}
		}
	}
	return false
}

// judgeTeachingMigrationRun converts a migrator run outcome into the phase
// outcome: a run error propagates; unresolved items (per-row failures)
// fail the phase so the caller records a blocked state — an empty or
// partially done dataset is never treated as success.
func judgeTeachingMigrationRun(
	report *types.TeachingRoleMigrationReport,
	err error,
) (*types.TeachingRoleMigrationReport, error) {
	if err != nil {
		return report, err
	}
	if report != nil && report.Failed > 0 {
		return report, fmt.Errorf(
			"teaching data migration left %d item(s) unresolved; committed items stay normalized, retry on next startup",
			report.Failed)
	}
	return report, nil
}

// Run verifies the schema then performs one normalization pass: pending
// legacy invitations are downgraded to viewer and legacy admin/contributor
// memberships are demoted with per-item success audits (#21). Committed
// state transitions are never re-audited on rerun (guarded updates) and
// failed items retry on the next run.
func (p *TeachingMigrationPhase) Run(ctx context.Context) (*types.TeachingRoleMigrationReport, error) {
	if err := p.VerifySchema(); err != nil {
		return nil, err
	}
	report, err := NewTeachingRoleMigrator(p.db, p.audit).Run(ctx)
	report, err = judgeTeachingMigrationRun(report, err)
	if err == nil && report != nil &&
		(report.Downgraded > 0 || report.InvitationsDowngraded > 0 || len(report.AnomalyTenantIDs) > 0) {
		logger.Infof(ctx,
			"Teaching data migration: downgraded=%d invitations=%d skipped=%d anomalies=%d",
			report.Downgraded, report.InvitationsDowngraded, report.Skipped, len(report.AnomalyTenantIDs))
	}
	return report, err
}

// RunAndRecord runs the phase and records the outcome in the shared
// blocked-state cache consumed by /system/info and the /health readiness
// probe. Success clears the state; any failure (missing schema, run error,
// unresolved items) re-arms it with an actionable message.
func (p *TeachingMigrationPhase) RunAndRecord(ctx context.Context) (*types.TeachingRoleMigrationReport, error) {
	report, err := p.Run(ctx)
	if err != nil {
		database.CacheTeachingMigrationError(err.Error())
		logger.Errorf(ctx, "Teaching data migration failed (recorded as blocked state): %v", err)
		return report, err
	}
	database.CacheTeachingMigrationError("")
	return report, nil
}
