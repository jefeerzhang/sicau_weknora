package bootstrap

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Tencent/WeKnora/internal/types"
)

type memStore struct {
	byEmail map[string]*types.User
	admins  []*types.User
}

func newMemStore(admins ...*types.User) *memStore {
	m := &memStore{byEmail: map[string]*types.User{}}
	for _, u := range admins {
		cp := *u
		m.byEmail[u.Email] = &cp
		if u.IsSystemAdmin {
			m.admins = append(m.admins, &cp)
		}
	}
	return m
}

func (m *memStore) GetUserByEmail(_ context.Context, email string) (*types.User, error) {
	u, ok := m.byEmail[email]
	if !ok {
		return nil, errors.New("not found")
	}
	cp := *u
	return &cp, nil
}

func (m *memStore) ListSystemAdmins(_ context.Context, _, _ int) ([]*types.User, int64, error) {
	out := make([]*types.User, 0, len(m.admins))
	for _, u := range m.admins {
		cp := *u
		out = append(out, &cp)
	}
	return out, int64(len(m.admins)), nil
}

func (m *memStore) CreateUser(_ context.Context, user *types.User) error {
	if _, ok := m.byEmail[user.Email]; ok {
		return errors.New("duplicate email")
	}
	cp := *user
	m.byEmail[user.Email] = &cp
	if user.IsSystemAdmin {
		m.admins = append(m.admins, &cp)
	}
	return nil
}

func TestEnsureSuperAdmin_CreatesWhenEmpty(t *testing.T) {
	store := newMemStore()
	user, err := EnsureSuperAdmin(context.Background(), store, Config{
		Email:    "admin@example.com",
		Password: "SecurePass9",
	})
	require.NoError(t, err)
	require.NotNil(t, user)
	require.True(t, user.IsSystemAdmin)
	require.True(t, user.MustChangePassword)
	require.Equal(t, uint64(0), user.TenantID)
	require.Equal(t, int64(1), mustCount(store))
}

func TestEnsureSuperAdmin_IdempotentWhenOneExists(t *testing.T) {
	existing := &types.User{ID: "a1", Email: "a@x.com", IsSystemAdmin: true}
	store := newMemStore(existing)
	user, err := EnsureSuperAdmin(context.Background(), store, Config{
		Email:    "other@example.com",
		Password: "SecurePass9",
	})
	require.NoError(t, err)
	require.Nil(t, user)
	require.Equal(t, int64(1), mustCount(store))
}

func TestEnsureSuperAdmin_RejectsMultiple(t *testing.T) {
	store := newMemStore(
		&types.User{ID: "1", Email: "a@x.com", IsSystemAdmin: true},
		&types.User{ID: "2", Email: "b@x.com", IsSystemAdmin: true},
	)
	_, err := EnsureSuperAdmin(context.Background(), store, Config{})
	require.ErrorIs(t, err, ErrAmbiguousSuperAdmins)
}

func TestEnsureSuperAdmin_MissingCreds(t *testing.T) {
	_, err := EnsureSuperAdmin(context.Background(), newMemStore(), Config{})
	require.ErrorIs(t, err, ErrMissingBootstrapCreds)
}

func TestEnsureSuperAdmin_LegacyOnly(t *testing.T) {
	_, err := EnsureSuperAdmin(context.Background(), newMemStore(), Config{LegacyEmailSet: true})
	require.ErrorIs(t, err, ErrLegacyBootstrapOnly)
}

func TestEnsureSuperAdmin_RefusesPromoteExistingEmail(t *testing.T) {
	store := newMemStore(&types.User{ID: "u1", Email: "admin@example.com", IsSystemAdmin: false})
	_, err := EnsureSuperAdmin(context.Background(), store, Config{
		Email:    "admin@example.com",
		Password: "SecurePass9",
	})
	require.ErrorIs(t, err, ErrMissingBootstrapCreds)
}

func TestAssertUniqueSuperAdminPromote(t *testing.T) {
	store := newMemStore(&types.User{ID: "1", Email: "a@x.com", IsSystemAdmin: true})
	err := AssertUniqueSuperAdminPromote(context.Background(), store, &types.User{ID: "2", Email: "b@x.com"})
	require.ErrorIs(t, err, ErrSecondSuperAdmin)

	err = AssertUniqueSuperAdminPromote(context.Background(), store, &types.User{ID: "1", Email: "a@x.com", IsSystemAdmin: true})
	require.NoError(t, err)
}

func mustCount(store *memStore) int64 {
	_, n, err := store.ListSystemAdmins(context.Background(), 0, 10)
	if err != nil {
		panic(err)
	}
	return n
}
