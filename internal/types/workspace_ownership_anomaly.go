package types

import (
	"time"
)

// WorkspaceOwnershipAnomalyKind classifies why a teaching workspace needs
// SuperAdmin recovery (#18/#19). Migration never auto-picks an owner.
type WorkspaceOwnershipAnomalyKind string

const (
	// WorkspaceOwnershipAnomalyZeroOwner: no active owner membership.
	WorkspaceOwnershipAnomalyZeroOwner WorkspaceOwnershipAnomalyKind = "zero_owner"
	// WorkspaceOwnershipAnomalyMultiOwner: more than one active owner.
	WorkspaceOwnershipAnomalyMultiOwner WorkspaceOwnershipAnomalyKind = "multi_owner"
)

// WorkspaceOwnershipAnomalyStatus tracks whether SuperAdmin has resolved
// the anomaly. Only "open" rows appear on the recovery list.
type WorkspaceOwnershipAnomalyStatus string

const (
	WorkspaceOwnershipAnomalyOpen     WorkspaceOwnershipAnomalyStatus = "open"
	WorkspaceOwnershipAnomalyResolved WorkspaceOwnershipAnomalyStatus = "resolved"
)

// WorkspaceOwnershipAnomaly records a zero-owner or multi-owner workspace
// discovered during the teaching role migration. Rows are upserted by
// tenant_id so re-runs stay idempotent.
type WorkspaceOwnershipAnomaly struct {
	ID              uint64                         `json:"id" gorm:"primaryKey"`
	TenantID        uint64                         `json:"tenant_id" gorm:"uniqueIndex;not null"`
	Kind            WorkspaceOwnershipAnomalyKind  `json:"kind" gorm:"type:varchar(32);not null"`
	OwnerCount      int                            `json:"owner_count" gorm:"not null;default:0"`
	OwnerUserIDs    JSON                           `json:"owner_user_ids" gorm:"type:jsonb;not null;default:'[]'"`
	Status          WorkspaceOwnershipAnomalyStatus `json:"status" gorm:"type:varchar(16);not null;default:'open';index"`
	ResolvedBy      *string                        `json:"resolved_by,omitempty" gorm:"type:varchar(36)"`
	ResolvedAt      *time.Time                     `json:"resolved_at,omitempty"`
	ResolutionNote  string                         `json:"resolution_note,omitempty" gorm:"type:text"`
	CreatedAt       time.Time                      `json:"created_at"`
	UpdatedAt       time.Time                      `json:"updated_at"`
}

// TableName pins the SQL migration table name.
func (WorkspaceOwnershipAnomaly) TableName() string {
	return "workspace_ownership_anomalies"
}

// TeachingRoleMigrationReport is the external result of one migrator run
// (#18). Counts reflect membership mutations and anomaly bookkeeping —
// not SQL statement ordering.
type TeachingRoleMigrationReport struct {
	Downgraded       int      `json:"downgraded"`
	Skipped          int      `json:"skipped"`
	Failed           int      `json:"failed"`
	AnomalyTenantIDs []uint64 `json:"anomaly_tenant_ids"`
}
