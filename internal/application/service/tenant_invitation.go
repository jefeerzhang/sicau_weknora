package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"time"

	apprepo "github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/gorm"
)

// Sentinel errors returned by tenantInvitationService. Callers compare
// with errors.Is to render the appropriate HTTP responses.
var (
	// ErrInvitationNotFound is returned when no invitation row matches
	// the supplied id.
	ErrInvitationNotFound = errors.New("invitation not found")

	// ErrPendingInvitationExists is returned by Create when a pending
	// invitation for (tenant, invitee) is already in flight. The
	// handler maps this to 409.
	ErrPendingInvitationExists = errors.New("a pending invitation for this user already exists")

	// ErrAlreadyMember is returned by Create when the invitee is
	// already an active member of the tenant; sending an invite would
	// be a no-op at best and a confusing UX at worst.
	ErrAlreadyMember = errors.New("user is already an active member of the tenant")

	// ErrInvitationNotPending is returned by Accept / Decline / Revoke
	// when the row exists but has already been finalised. Maps to 409.
	ErrInvitationNotPending = errors.New("invitation is no longer pending")

	// ErrInvitationExpired is returned by Accept / Decline when the
	// pending row has aged past its expires_at. Maps to 410.
	ErrInvitationExpired = errors.New("invitation has expired")

	// ErrInvitationForbidden is returned by Accept / Decline when the
	// caller is not the invitee. Owner-driven Revoke is gated at the
	// route layer so this error only surfaces on the /me/ paths.
	ErrInvitationForbidden = errors.New("only the invitee can accept or decline this invitation")

	// ErrInvitationRoleRestrictedToViewer is returned by Create /
	// CreateShareLink when a caller asks for any role other than viewer.
	// The teaching two-state model (#21) materialises invitations as the
	// student relationship only; this is the creation-time enforcement
	// that backs the HTTP guards, so no code path can persist an elevated
	// invitation role. Legacy elevated rows from before the rule are
	// handled at acceptance time (clamped) and by the normalization
	// migration, never by new writes.
	ErrInvitationRoleRestrictedToViewer = errors.New("invitations can only grant the student (viewer) role")

	// ErrInvitationTokenInvalid is returned by LookupByToken /
	// AcceptByToken when the supplied plaintext token does not match
	// any active share-link row. The handler maps this to 410 Gone.
	// We deliberately collapse "unknown" / "expired" / "revoked" into
	// a single sentinel so an attacker can't probe which slots used
	// to exist.
	ErrInvitationTokenInvalid = errors.New("invitation token is invalid or has been revoked")
)

// defaultInvitationTTL is the lifetime of a pending invitation before
// the lazy sweep transitions it to expired. Operator override is via
// the WEKNORA_INVITATION_TTL env var (Go duration: "168h", "7d-ish");
// keeping this out of TenantConfig avoids a yaml migration for a knob
// that almost nobody is going to tweak.
const defaultInvitationTTL = 7 * 24 * time.Hour

// invitationTTL resolves the effective TTL at call time so an operator
// can hot-reload the override without restarting. The env var is parsed
// once per call; cost is negligible and beats a goroutine watching the
// environment.
func invitationTTL() time.Duration {
	raw := os.Getenv("WEKNORA_INVITATION_TTL")
	if raw == "" {
		return defaultInvitationTTL
	}
	if d, err := time.ParseDuration(raw); err == nil && d > 0 {
		return d
	}
	// Allow "604800" seconds for callers that don't like Go duration
	// syntax — same shape as the rest of our int env knobs.
	if secs, err := strconv.ParseInt(raw, 10, 64); err == nil && secs > 0 {
		return time.Duration(secs) * time.Second
	}
	return defaultInvitationTTL
}

// tenantInvitationService implements interfaces.TenantInvitationService.
type tenantInvitationService struct {
	// db is the application database handle, used to open the acceptance
	// transaction. Nil is tolerated in unit tests whose doubles carry no
	// gorm handle (see runInTx).
	db *gorm.DB
	repo      interfaces.TenantInvitationRepository
	memberSvc interfaces.TenantMemberService
	audit     interfaces.AuditLogService // optional; nil ⇒ no audit, business ops still succeed
	now       func() time.Time           // injection seam for tests
}

// NewTenantInvitationService wires the dependencies. memberSvc is
// required (Accept must create the tenant_members row); audit is
// optional and matches the same nil-safe pattern tenantMemberService
// uses. db opens the acceptance transaction; nil keeps the unit-test
// doubles working unchanged.
func NewTenantInvitationService(
	db *gorm.DB,
	repo interfaces.TenantInvitationRepository,
	memberSvc interfaces.TenantMemberService,
	audit interfaces.AuditLogService,
) interfaces.TenantInvitationService {
	return &tenantInvitationService{
		db:        db,
		repo:      repo,
		memberSvc: memberSvc,
		audit:     audit,
		now:       time.Now,
	}
}

// runInTx executes fn on a database transaction when the service carries
// a db handle. Unit tests wiring in-memory doubles pass db=nil; their
// WithTx implementations return themselves for a nil tx, so the flow runs
// unbatched exactly as before. Production always has a handle.
func (s *tenantInvitationService) runInTx(ctx context.Context, fn func(tx *gorm.DB) error) error {
	if s.db == nil {
		return fn(nil)
	}
	return s.db.WithContext(ctx).Transaction(fn)
}

// emitAuditTx writes an audit entry on tx (the acceptance transaction) so
// the permission change and its trail commit or roll back together. When
// audit is unwired the flow stays nil-safe.
func (s *tenantInvitationService) emitAuditTx(ctx context.Context, tx *gorm.DB, entry *types.AuditLog) error {
	if s.audit == nil {
		return nil
	}
	return s.audit.LogTx(ctx, tx, entry)
}

// emitAudit is the best-effort audit hook; mirrors tenantMemberService.
func (s *tenantInvitationService) emitAudit(ctx context.Context, entry *types.AuditLog) {
	if s.audit == nil {
		return
	}
	_ = s.audit.Log(ctx, entry)
}

// detailsFor packs the role + invitation id into the audit Details so a
// reader can reconstruct "Alice invited Bob as Admin (inv #42)" without
// joining back to the invitations table.
func detailsFor(invID uint64, role types.TenantRole) types.JSON {
	b, _ := json.Marshal(map[string]any{
		"invitation_id": invID,
		"role":          string(role),
	})
	return types.JSON(b)
}

// sweep transitions overdue pending rows to expired before any List/
// Accept/Decline/Count read. Failures are logged and swallowed — a
// transient sweep error must not block a user from reading their
// inbox; the next call will sweep again.
func (s *tenantInvitationService) sweep(ctx context.Context) {
	if _, err := s.repo.SweepExpired(ctx, s.now()); err != nil {
		logger.Warnf(ctx, "tenant_invitation lazy sweep failed: %v", err)
	}
}

// Create issues a new pending invitation. Returns the standard
// conflict sentinels for the duplicate-pending and already-member
// cases.
func (s *tenantInvitationService) Create(
	ctx context.Context,
	tenantID uint64,
	inviteeUserID string,
	role types.TenantRole,
	invitedBy *string,
	message string,
) (*types.TenantInvitation, error) {
	if !role.IsValid() {
		return nil, ErrInvalidTenantRole
	}
	if err := rejectAPIKeyOwnerAssignment(ctx, role); err != nil {
		return nil, err
	}
	if role != types.TenantRoleViewer {
		return nil, ErrInvitationRoleRestrictedToViewer
	}
	// Reject early if the invitee is already an active member; the
	// handler renders this as "they're already in" rather than the
	// generic conflict.
	existing, err := s.memberSvc.GetMembership(ctx, inviteeUserID, tenantID)
	if err != nil {
		return nil, err
	}
	if existing != nil && existing.Status == types.TenantMemberStatusActive {
		return nil, ErrAlreadyMember
	}

	now := s.now()
	inv := &types.TenantInvitation{
		TenantID:      tenantID,
		InviteeUserID: inviteeUserID,
		InvitedBy:     invitedBy,
		Role:          role,
		Status:        types.TenantInvitationStatusPending,
		Message:       message,
		ExpiresAt:     now.Add(invitationTTL()),
	}
	if err := s.repo.Create(ctx, inv); err != nil {
		if errors.Is(err, apprepo.ErrPendingInvitationExists) {
			return nil, ErrPendingInvitationExists
		}
		return nil, err
	}

	s.emitAudit(ctx, &types.AuditLog{
		TenantID:     tenantID,
		ActorUserID:  auditActor(ctx),
		ActorRole:    auditActorRole(ctx),
		Action:       types.AuditActionInvitationSent,
		TargetType:   "tenant_invitation",
		TargetID:     strconv.FormatUint(inv.ID, 10),
		TargetUserID: inviteeUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      detailsFor(inv.ID, role),
	})
	return inv, nil
}

// Accept transitions a pending invitation into accepted AND creates the
// active tenant_members row inside a single database transaction, together
// with the rbac.member_added and rbac.invitation_accepted audit rows: any
// failure rolls the flip, the membership and both audits back, so a retry
// starts from a clean pending state (#21).
//
// The persisted invitation role is never trusted at this boundary — a
// pre-upgrade pending row may still carry admin/contributor — so the
// membership is always materialised as viewer (the teaching "student"
// relationship). An invitee who is already an active member keeps their
// existing membership untouched (no downgrade, no duplicate audit).
func (s *tenantInvitationService) Accept(
	ctx context.Context,
	invID uint64,
	callerUserID string,
) (*types.TenantMember, error) {
	s.sweep(ctx)

	inv, err := s.repo.GetByID(ctx, invID)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvitationNotFound
	}
	if inv.InviteeUserID != callerUserID {
		return nil, ErrInvitationForbidden
	}
	if inv.Status != types.TenantInvitationStatusPending {
		return nil, ErrInvitationNotPending
	}
	if inv.IsExpired(s.now()) {
		// The sweep above should have flipped it already, but a row
		// can age past expires_at between the sweep and this read.
		// Treat it as expired regardless.
		return nil, ErrInvitationExpired
	}

	// #21: invitation acceptance materialises viewer (student) only.
	memberRole := types.TenantRoleViewer

	now := s.now()
	var member *types.TenantMember
	err = s.runInTx(ctx, func(tx *gorm.DB) error {
		if err := s.repo.WithTx(tx).MarkStatusIfPending(ctx, invID, types.TenantInvitationStatusAccepted, now); err != nil {
			// Another goroutine (concurrent click) won the race. Honour
			// the state machine.
			return ErrInvitationNotPending
		}

		m, err := s.memberSvc.AddMemberTx(ctx, tx, inv.InviteeUserID, inv.TenantID, memberRole, inv.InvitedBy)
		if err != nil {
			// "Already a member" is the idempotent outcome: keep the
			// flip and the accepted audit, return the existing
			// membership instead of bubbling the error up. The
			// membership itself is never rewritten, so an elevated
			// pre-existing role cannot be clobbered here.
			if errors.Is(err, ErrMembershipAlreadyExists) {
				existing, getErr := s.memberSvc.GetMembership(ctx, inv.InviteeUserID, inv.TenantID)
				if getErr == nil && existing != nil {
					member = existing
					return s.emitInvitationAcceptedTx(ctx, tx, inv, memberRole)
				}
			}
			return err
		}
		member = m
		return s.emitInvitationAcceptedTx(ctx, tx, inv, memberRole)
	})
	if err != nil {
		// The transaction rolled back: invitation still pending, no
		// membership, no audits — a retry is safe.
		if !errors.Is(err, ErrInvitationNotPending) {
			logger.Errorf(ctx,
				"invitation %d accept failed (rolled back): %v",
				invID, err)
		}
		return nil, err
	}
	return member, nil
}

// MarkPendingAcceptedIfExists reconciles a stale per-user pending row when
// tenant.auto_accept_invitation joins the invitee directly. member_added
// audit from AddMember is the authoritative trail; we only flip status here.
func (s *tenantInvitationService) MarkPendingAcceptedIfExists(
	ctx context.Context,
	tenantID uint64,
	inviteeUserID string,
) error {
	s.sweep(ctx)
	inv, err := s.repo.GetPendingByPair(ctx, tenantID, inviteeUserID)
	if err != nil {
		return err
	}
	if inv == nil {
		return nil
	}
	return s.repo.MarkStatusIfPending(ctx, inv.ID, types.TenantInvitationStatusAccepted, s.now())
}

// emitInvitationAcceptedTx writes the rbac.invitation_accepted audit row
// on tx. Actor is the invitee (acting on their own inbox); target is the
// same user since the action is self-directed. role is the role the
// acceptance actually materialised (viewer in the teaching model), which
// is what the audit must record.
func (s *tenantInvitationService) emitInvitationAcceptedTx(
	ctx context.Context,
	tx *gorm.DB,
	inv *types.TenantInvitation,
	role types.TenantRole,
) error {
	return s.emitAuditTx(ctx, tx, &types.AuditLog{
		TenantID:     inv.TenantID,
		ActorUserID:  auditActor(ctx),
		ActorRole:    auditActorRole(ctx),
		Action:       types.AuditActionInvitationAccepted,
		TargetType:   "tenant_invitation",
		TargetID:     strconv.FormatUint(inv.ID, 10),
		TargetUserID: inv.InviteeUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      detailsFor(inv.ID, role),
	})
}

// Decline transitions the pending row into declined. Only the invitee
// themselves can call this.
func (s *tenantInvitationService) Decline(
	ctx context.Context,
	invID uint64,
	callerUserID string,
) error {
	s.sweep(ctx)

	inv, err := s.repo.GetByID(ctx, invID)
	if err != nil {
		return err
	}
	if inv == nil {
		return ErrInvitationNotFound
	}
	if inv.InviteeUserID != callerUserID {
		return ErrInvitationForbidden
	}
	if inv.Status != types.TenantInvitationStatusPending {
		return ErrInvitationNotPending
	}
	if inv.IsExpired(s.now()) {
		return ErrInvitationExpired
	}

	if err := s.repo.MarkStatusIfPending(ctx, invID, types.TenantInvitationStatusDeclined, s.now()); err != nil {
		return ErrInvitationNotPending
	}

	s.emitAudit(ctx, &types.AuditLog{
		TenantID:     inv.TenantID,
		ActorUserID:  auditActor(ctx),
		ActorRole:    auditActorRole(ctx),
		Action:       types.AuditActionInvitationDeclined,
		TargetType:   "tenant_invitation",
		TargetID:     strconv.FormatUint(inv.ID, 10),
		TargetUserID: inv.InviteeUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      detailsFor(inv.ID, inv.Role),
	})
	return nil
}

// Revoke transitions the pending row into revoked. Route-layer Owner
// gate guarantees the caller is allowed to act on this tenant; this
// method does not re-check role.
func (s *tenantInvitationService) Revoke(ctx context.Context, invID uint64) error {
	s.sweep(ctx)

	inv, err := s.repo.GetByID(ctx, invID)
	if err != nil {
		return err
	}
	if inv == nil {
		return ErrInvitationNotFound
	}
	if inv.Status != types.TenantInvitationStatusPending {
		return ErrInvitationNotPending
	}

	if err := s.repo.MarkStatusIfPending(ctx, invID, types.TenantInvitationStatusRevoked, s.now()); err != nil {
		return ErrInvitationNotPending
	}

	s.emitAudit(ctx, &types.AuditLog{
		TenantID:     inv.TenantID,
		ActorUserID:  auditActor(ctx),
		ActorRole:    auditActorRole(ctx),
		Action:       types.AuditActionInvitationRevoked,
		TargetType:   "tenant_invitation",
		TargetID:     strconv.FormatUint(inv.ID, 10),
		TargetUserID: inv.InviteeUserID,
		Outcome:      types.AuditOutcomeSuccess,
		Details:      detailsFor(inv.ID, inv.Role),
	})
	return nil
}

// GetByID returns the row or (nil, nil) without sweeping. The handler
// uses this for narrow per-row checks where running a full sweep just
// to read one row would be wasted work; List* paths still sweep.
func (s *tenantInvitationService) GetByID(
	ctx context.Context,
	invID uint64,
) (*types.TenantInvitation, error) {
	return s.repo.GetByID(ctx, invID)
}

// ListByTenant sweeps then returns. The tenant-side management UI
// expects expired rows to surface correctly; running the sweep here
// guarantees the page reflects reality even if no other code path has
// touched the table recently.
func (s *tenantInvitationService) ListByTenant(
	ctx context.Context,
	tenantID uint64,
	includeTerminal bool,
) ([]*types.TenantInvitation, error) {
	s.sweep(ctx)
	return s.repo.ListByTenant(ctx, tenantID, includeTerminal)
}

// ListTenantInvitationsPage sweeps then returns a page plus total rows
// matching the same filter as ListByTenant.
func (s *tenantInvitationService) ListTenantInvitationsPage(
	ctx context.Context,
	tenantID uint64,
	includeTerminal bool,
	page, pageSize int,
) ([]*types.TenantInvitation, int64, error) {
	const (
		defSize = 20
		maxSize = 100
	)
	s.sweep(ctx)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defSize
	}
	if pageSize > maxSize {
		pageSize = maxSize
	}
	total, err := s.repo.CountByTenantList(ctx, tenantID, includeTerminal)
	if err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	rows, err := s.repo.ListByTenantPage(ctx, tenantID, includeTerminal, offset, pageSize)
	if err != nil {
		return nil, 0, err
	}
	return rows, total, nil
}

// GetActiveShareLink returns the tenant's reusable link independently of
// invitation-list pagination.
func (s *tenantInvitationService) GetActiveShareLink(
	ctx context.Context,
	tenantID uint64,
) (*types.TenantInvitation, error) {
	s.sweep(ctx)
	return s.repo.GetActiveShareLinkByTenant(ctx, tenantID)
}

// ListByInvitee sweeps then returns. Same reasoning as ListByTenant.
func (s *tenantInvitationService) ListByInvitee(
	ctx context.Context,
	inviteeUserID string,
	includeTerminal bool,
) ([]*types.TenantInvitation, error) {
	s.sweep(ctx)
	return s.repo.ListByInvitee(ctx, inviteeUserID, includeTerminal)
}

// CountPendingByInvitee sweeps then counts. The avatar-row badge polls
// this endpoint so a stale sweep would manifest as a phantom "1" on
// the bell icon for the full polling interval; sweeping inline is
// worth the extra UPDATE.
func (s *tenantInvitationService) CountPendingByInvitee(
	ctx context.Context,
	inviteeUserID string,
) (int64, error) {
	s.sweep(ctx)
	return s.repo.CountPendingByInvitee(ctx, inviteeUserID)
}

// invitationTokenBytes is the raw entropy length for share-link
// tokens before base64url encoding. 32 bytes -> 256 bits, well above
// the 128-bit floor for unguessable opaque tokens.
const invitationTokenBytes = 32

// generateShareLinkToken returns a freshly-randomised plaintext token
// (base64url, no padding) for a new share-link invitation.
func generateShareLinkToken() (string, error) {
	buf := make([]byte, invitationTokenBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// CreateShareLink issues a multi-use share-link invitation. The token
// is generated server-side and persisted plaintext on the row so the
// management UI can re-display it on demand. A tenant has at most one
// pending share link: repeated calls return that row and token until it
// is revoked or expires. Consumption remains non-destructive (see
// AcceptByToken).
func (s *tenantInvitationService) CreateShareLink(
	ctx context.Context,
	tenantID uint64,
	role types.TenantRole,
	invitedBy *string,
	message string,
) (*types.TenantInvitation, string, error) {
	if !role.IsValid() {
		return nil, "", ErrInvalidTenantRole
	}
	if err := rejectAPIKeyOwnerAssignment(ctx, role); err != nil {
		return nil, "", err
	}
	if role != types.TenantRoleViewer {
		return nil, "", ErrInvitationRoleRestrictedToViewer
	}
	s.sweep(ctx)
	existing, err := s.repo.GetActiveShareLinkByTenant(ctx, tenantID)
	if err != nil {
		return nil, "", err
	}
	if existing != nil && !existing.IsExpired(s.now()) {
		return existing, existing.Token, nil
	}
	token, err := generateShareLinkToken()
	if err != nil {
		return nil, "", err
	}
	now := s.now()
	inv := &types.TenantInvitation{
		TenantID:      tenantID,
		InviteeUserID: "", // share-link rows have no specific invitee
		Token:         token,
		InvitedBy:     invitedBy,
		Role:          role,
		Status:        types.TenantInvitationStatusPending,
		Message:       message,
		ExpiresAt:     now.Add(invitationTTL()),
	}
	if err := s.repo.Create(ctx, inv); err != nil {
		// A concurrent request may have created the tenant's link after
		// our read. Return the winner instead of surfacing a conflict.
		if errors.Is(err, apprepo.ErrPendingInvitationExists) {
			existing, readErr := s.repo.GetActiveShareLinkByTenant(ctx, tenantID)
			if readErr == nil && existing != nil {
				return existing, existing.Token, nil
			}
		}
		return nil, "", err
	}
	s.emitAudit(ctx, &types.AuditLog{
		TenantID:    tenantID,
		ActorUserID: auditActor(ctx),
		ActorRole:   auditActorRole(ctx),
		Action:      types.AuditActionInvitationSent,
		TargetType:  "tenant_invitation",
		TargetID:    strconv.FormatUint(inv.ID, 10),
		// TargetUserID intentionally empty — share-link has no invitee yet.
		Outcome: types.AuditOutcomeSuccess,
		Details: detailsFor(inv.ID, role),
	})
	return inv, token, nil
}

// LookupByToken resolves a plaintext share-link token to its row.
// Sweeps overdue rows first so an expired link is reflected as
// expired rather than letting the registration page accept it for a
// few extra seconds. Multi-use semantics: a successful lookup does
// not consume or modify the row.
func (s *tenantInvitationService) LookupByToken(
	ctx context.Context,
	plainToken string,
) (*types.TenantInvitation, error) {
	plainToken = strings.TrimSpace(plainToken)
	if plainToken == "" {
		return nil, ErrInvitationTokenInvalid
	}
	s.sweep(ctx)
	inv, err := s.repo.GetActiveByToken(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	if inv == nil {
		return nil, ErrInvitationTokenInvalid
	}
	if inv.IsExpired(s.now()) {
		return nil, ErrInvitationTokenInvalid
	}
	return inv, nil
}

// AcceptByToken adds newUserID to the share-link's tenant. Unlike Accept,
// the invitation row itself is NOT mutated — share-link rows stay pending
// across uses. The membership insert and its audits run in one transaction
// (#21): the persisted link role is never trusted, so the membership is
// always materialised as viewer (student). Idempotent: an existing
// membership is returned untouched (no downgrade, no duplicate audit, no
// second usage bump).
func (s *tenantInvitationService) AcceptByToken(
	ctx context.Context,
	plainToken string,
	newUserID string,
) (*types.TenantMember, error) {
	if newUserID == "" {
		return nil, errors.New("newUserID is required")
	}
	inv, err := s.LookupByToken(ctx, plainToken)
	if err != nil {
		return nil, err
	}
	// #21: share-link acceptance materialises viewer (student) only.
	memberRole := types.TenantRoleViewer

	var member *types.TenantMember
	fresh := false
	err = s.runInTx(ctx, func(tx *gorm.DB) error {
		m, err := s.memberSvc.AddMemberTx(ctx, tx, newUserID, inv.TenantID, memberRole, inv.InvitedBy)
		if err != nil {
			if errors.Is(err, ErrMembershipAlreadyExists) {
				// Repeat click of the same link: return the existing
				// membership unchanged and skip the usage counter and
				// the accepted audit (already written on first use).
				existing, getErr := s.memberSvc.GetMembership(ctx, newUserID, inv.TenantID)
				if getErr == nil && existing != nil {
					member = existing
					return nil
				}
			}
			return err
		}
		member = m
		fresh = true
		return s.emitInvitationAcceptedTx(ctx, tx, inv, memberRole)
	})
	if err != nil {
		// Rolled back: no membership, no audit — a retry is safe.
		logger.Errorf(ctx,
			"share-link %d accept failed for user %s (rolled back): %v",
			inv.ID, newUserID, err)
		return nil, err
	}
	// Bump usage counter so the management UI can show "N 人已加入".
	// Best-effort and outside the acceptance transaction: the counter is
	// for display only; audit log + tenant_members rows are the
	// authoritative trail. Only fresh joins bump the count.
	if fresh {
		if incErr := s.repo.IncrementAcceptedCount(ctx, inv.ID); incErr != nil {
			logger.Warnf(ctx,
				"share-link %d accepted_count bump failed (membership still created): %v",
				inv.ID, incErr)
		}
	}
	return member, nil
}
