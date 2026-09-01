package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/Tencent/WeKnora/internal/application/service"
	"github.com/Tencent/WeKnora/internal/types"
)

// Env vars for teaching SuperAdmin bootstrap (#8).
const (
	EnvSuperAdminEmail    = "WEKNORA_BOOTSTRAP_SUPERADMIN_EMAIL"
	EnvSuperAdminPassword = "WEKNORA_BOOTSTRAP_SUPERADMIN_PASSWORD"
	// LegacyEmailEnv is the pre-#8 promote-only hook. Presence without the
	// new pair must fail closed so operators do not silently keep the old
	// "register then promote" path after invite_only is enabled.
	LegacyEmailEnv = "WEKNORA_BOOTSTRAP_SYSTEM_ADMIN_EMAIL"
)

var (
	// ErrAmbiguousSuperAdmins is returned when more than one SystemAdmin exists.
	ErrAmbiguousSuperAdmins = errors.New("bootstrap: multiple system admins exist; converge to exactly one SuperAdmin before starting")
	// ErrMissingBootstrapCreds is returned when zero SuperAdmins exist and
	// EMAIL+PASSWORD are not both set / valid.
	ErrMissingBootstrapCreds = errors.New("bootstrap: no SuperAdmin and WEKNORA_BOOTSTRAP_SUPERADMIN_EMAIL/PASSWORD not set; refuse to start without an authority root")
	// ErrLegacyBootstrapOnly is returned when only the old EMAIL env is set.
	ErrLegacyBootstrapOnly = errors.New("bootstrap: WEKNORA_BOOTSTRAP_SYSTEM_ADMIN_EMAIL is no longer sufficient; set WEKNORA_BOOTSTRAP_SUPERADMIN_EMAIL and WEKNORA_BOOTSTRAP_SUPERADMIN_PASSWORD to create the SuperAdmin")
	// ErrSecondSuperAdmin is returned by Promote when one SuperAdmin already exists.
	ErrSecondSuperAdmin = errors.New("cannot promote a second SuperAdmin; appoint Teachers instead")
)

// UserStore is the narrow surface EnsureSuperAdmin needs.
type UserStore interface {
	GetUserByEmail(ctx context.Context, email string) (*types.User, error)
	ListSystemAdmins(ctx context.Context, offset, limit int) ([]*types.User, int64, error)
	CreateUser(ctx context.Context, user *types.User) error
}

// Config is the resolved bootstrap input (usually from env).
type Config struct {
	Email          string
	Password       string
	LegacyEmailSet bool
}

// ConfigFromEnv reads the bootstrap environment.
func ConfigFromEnv(getenv func(string) string) Config {
	return Config{
		Email:          strings.TrimSpace(getenv(EnvSuperAdminEmail)),
		Password:       getenv(EnvSuperAdminPassword), // preserve intentional spaces only at ends via TrimSpace below
		LegacyEmailSet: strings.TrimSpace(getenv(LegacyEmailEnv)) != "",
	}
}

// EnsureSuperAdmin enforces the unique SuperAdmin invariant at startup.
//
//   - count > 1 → ErrAmbiguousSuperAdmins
//   - count = 1 → nil (idempotent)
//   - count = 0 + valid EMAIL/PASSWORD → create tenantless SuperAdmin with MustChangePassword
//   - count = 0 + legacy-only env → ErrLegacyBootstrapOnly
//   - count = 0 + missing/invalid creds → ErrMissingBootstrapCreds
func EnsureSuperAdmin(ctx context.Context, store UserStore, cfg Config) (*types.User, error) {
	cfg.Email = strings.TrimSpace(cfg.Email)
	cfg.Password = strings.TrimSpace(cfg.Password)

	_, total, err := store.ListSystemAdmins(ctx, 0, 2)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: list system admins: %w", err)
	}
	if total > 1 {
		return nil, ErrAmbiguousSuperAdmins
	}
	if total == 1 {
		return nil, nil
	}

	// total == 0
	if cfg.Email == "" || cfg.Password == "" {
		if cfg.LegacyEmailSet {
			return nil, ErrLegacyBootstrapOnly
		}
		return nil, ErrMissingBootstrapCreds
	}
	if err := service.ValidatePasswordPolicy(cfg.Password); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMissingBootstrapCreds, err)
	}

	existing, _ := store.GetUserByEmail(ctx, cfg.Email)
	if existing != nil {
		// Do not silently promote an arbitrary pre-registered account into
		// SuperAdmin — that was the old path and races with invite_only.
		return nil, fmt.Errorf("%w: email %s already registered; refuse silent promotion", ErrMissingBootstrapCreds, cfg.Email)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(cfg.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("bootstrap: hash password: %w", err)
	}

	username := deriveUsername(cfg.Email)
	now := time.Now()
	id := uuid.New().String()
	user := &types.User{
		ID:                 id,
		Username:           username,
		Email:              cfg.Email,
		PasswordHash:       string(hashed),
		TenantID:           0, // tenantless
		IsActive:           true,
		IsSystemAdmin:      true,
		MustChangePassword: true,
		CreatedAt:          now,
		UpdatedAt:          now,
	}
	if err := store.CreateUser(ctx, user); err != nil {
		// Username collision: fall back to a unique derived name.
		user.Username = "sa_" + strings.ReplaceAll(id[:8], "-", "")
		if err2 := store.CreateUser(ctx, user); err2 != nil {
			return nil, fmt.Errorf("bootstrap: create SuperAdmin: %w", err2)
		}
	}
	return user, nil
}

func deriveUsername(email string) string {
	local := email
	if i := strings.IndexByte(email, '@'); i > 0 {
		local = email[:i]
	}
	local = strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
			return r
		default:
			return '_'
		}
	}, local)
	if local == "" {
		return "superadmin"
	}
	if len(local) > 64 {
		local = local[:64]
	}
	return local
}

// AssertUniqueSuperAdminPromote rejects creating a second SystemAdmin.
func AssertUniqueSuperAdminPromote(ctx context.Context, store UserStore, target *types.User) error {
	if target == nil {
		return errors.New("user is nil")
	}
	if target.IsSystemAdmin {
		return nil // idempotent
	}
	_, total, err := store.ListSystemAdmins(ctx, 0, 1)
	if err != nil {
		return err
	}
	if total >= 1 {
		return ErrSecondSuperAdmin
	}
	return nil
}
