package repository

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// newListTeachersTestDB opens a shared-cache in-memory SQLite and migrates the
// tenant + user tables so the repository's ListTeachers query runs against real
// schema rather than a stub.
func newListTeachersTestDB(t *testing.T) (*gorm.DB, interfaces.UserRepository) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:list_teachers?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Tenant{}, &types.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db, NewUserRepository(db)
}

// TestListTeachersIncludesCompositeSuperAdmin guards fix#3: the teacher list
// must reflect the full 教师端 membership defined in CONTEXT.md — the explicitly
// appointed teacher identity (is_teacher=true) AND the composite SuperAdmin who
// inherits the teacher capability (is_system_admin=true, is_teacher=false).
// Ordinary users (neither flag) must stay out of the list.
func TestListTeachersIncludesCompositeSuperAdmin(t *testing.T) {
	_, repo := newListTeachersTestDB(t)
	ctx := context.Background()

	seed := []*types.User{
		{ID: "u-teacher", Username: "teacher", Email: "teacher@example.com", PasswordHash: "hashed", IsTeacher: true, IsSystemAdmin: false, IsActive: true},
		{ID: "u-super", Username: "super", Email: "super@example.com", PasswordHash: "hashed", IsTeacher: false, IsSystemAdmin: true, IsActive: true},
		{ID: "u-normal", Username: "normal", Email: "normal@example.com", PasswordHash: "hashed", IsTeacher: false, IsSystemAdmin: false, IsActive: true},
	}
	for _, u := range seed {
		if err := repo.CreateUser(ctx, u); err != nil {
			t.Fatalf("CreateUser(%s): %v", u.ID, err)
		}
	}

	users, total, err := repo.ListTeachers(ctx, 0, 100)
	if err != nil {
		t.Fatalf("ListTeachers: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2 (teacher + composite superadmin), got %d", total)
	}

	got := map[string]bool{}
	for _, u := range users {
		got[u.Email] = true
	}
	if !got["teacher@example.com"] {
		t.Errorf("explicit teacher missing from list: %v", got)
	}
	if !got["super@example.com"] {
		t.Errorf("composite SuperAdmin missing from list (fix#3): %v", got)
	}
	if got["normal@example.com"] {
		t.Errorf("ordinary user should NOT be in teacher list: %v", got)
	}
}
