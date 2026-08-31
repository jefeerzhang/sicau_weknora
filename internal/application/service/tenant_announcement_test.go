package service

// sicau-v1 announcements service tests: post gating, attachment
// whitelist/limits via the real local FileService (temp dir), delete
// semantics, and comment permissions.

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/application/service/file"
	apperrors "github.com/Tencent/WeKnora/internal/errors"
	"github.com/Tencent/WeKnora/internal/types"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type announcementTestCtx struct {
	svc    interfaces.TenantAnnouncementService
	repo   interfaces.TenantAnnouncementRepository
	caller string
	role   types.TenantRole
}

func (a *announcementTestCtx) withCaller(ctx context.Context, role types.TenantRole, userID string) context.Context {
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, userID)
	ctx = context.WithValue(ctx, types.TenantRoleContextKey, role)
	return ctx
}

func (a *announcementTestCtx) ctx() context.Context {
	return a.withCaller(context.Background(), a.role, a.caller)
}

func newAnnouncementTestCtx(t *testing.T) *announcementTestCtx {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "ann.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.Announcement{}, &types.AnnouncementComment{}, &types.User{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	// Seed the authors so username hydration resolves.
	users := []types.User{
		{ID: "u-teacher", Username: "teacher", Email: "teacher@test.local"},
		{ID: "u-student", Username: "张三1234", Email: "s1@test.local"},
		{ID: "u-other", Username: "李四5678", Email: "s2@test.local"},
	}
	for i := range users {
		if err := db.Create(&users[i]).Error; err != nil {
			t.Fatalf("seed user: %v", err)
		}
	}

	repo := repository.NewTenantAnnouncementRepository(db)
	fileSvc := file.NewLocalFileService(t.TempDir(), "")
	userRepo := repository.NewUserRepository(db)
	svc := NewTenantAnnouncementService(repo, fileSvc, userRepo)
	return &announcementTestCtx{svc: svc, repo: repo, caller: "u-teacher", role: types.TenantRoleOwner}
}

func TestAnnouncements_CreateAndList_OwnerWithAttachment(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	ctx := tc.ctx()

	ann, err := tc.svc.Create(ctx, "第一次作业", "请完成第一章习题", []interfaces.AnnouncementUploadFile{
		{Name: "作业一.pdf", Data: []byte("%PDF-1.4 fake pdf bytes")},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if ann.ID == "" || len(ann.Attachments) != 1 {
		t.Fatalf("unexpected announcement: %+v", ann)
	}
	if !strings.HasPrefix(ann.Attachments[0].Path, "local://") {
		t.Fatalf("attachment path should be a FileService reference, got %q", ann.Attachments[0].Path)
	}

	got, err := tc.svc.Get(ctx, ann.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Title != "第一次作业" || len(got.Attachments) != 1 {
		t.Fatalf("roundtrip mismatch: %+v", got)
	}
}

func TestAnnouncements_ViewerCannotPost(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	tc.role = types.TenantRoleViewer
	tc.caller = "u-student"

	_, err := tc.svc.Create(tc.ctx(), "t", "c", nil)
	if err == nil {
		t.Fatal("viewer must not post")
	}
	appErr, ok := err.(*apperrors.AppError)
	if !ok || appErr.HTTPCode != 403 {
		t.Fatalf("want 403 app error, got %v", err)
	}
}

func TestAnnouncements_RejectsBadAttachments(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	ctx := tc.ctx()

	if _, err := tc.svc.Create(ctx, "t", "c", []interfaces.AnnouncementUploadFile{{Name: "virus.exe", Data: []byte("MZ")}}); err == nil {
		t.Fatal(".exe must be rejected")
	}

	big := strings.Repeat("a", 51<<20)
	if _, err := tc.svc.Create(ctx, "t", "c", []interfaces.AnnouncementUploadFile{{Name: "big.pdf", Data: []byte(big)}}); err == nil {
		t.Fatal(">50MB must be rejected")
	}

	many := make([]interfaces.AnnouncementUploadFile, 6)
	for i := range many {
		many[i] = interfaces.AnnouncementUploadFile{Name: "a.txt", Data: []byte("x")}
	}
	if _, err := tc.svc.Create(ctx, "t", "c", many); err == nil {
		t.Fatal("6 attachments must be rejected")
	}
}

func TestAnnouncements_DeleteSemantics(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	ann, err := tc.svc.Create(tc.ctx(), "t", "c", []interfaces.AnnouncementUploadFile{{Name: "a.txt", Data: []byte("x")}})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	studentCtx := tc.withCaller(context.Background(), types.TenantRoleViewer, "u-student")
	if err := tc.svc.Delete(studentCtx, ann.ID); err == nil {
		t.Fatal("student must not delete another's announcement")
	}

	if err := tc.svc.Delete(tc.ctx(), ann.ID); err != nil {
		t.Fatalf("author delete: %v", err)
	}
	if _, err := tc.svc.Get(tc.ctx(), ann.ID); err == nil {
		t.Fatal("deleted announcement should 404")
	}
}

func TestAnnouncements_Comments_Permissions(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	ann, err := tc.svc.Create(tc.ctx(), "t", "c", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	studentCtx := tc.withCaller(context.Background(), types.TenantRoleViewer, "u-student")
	item, err := tc.svc.CreateComment(studentCtx, ann.ID, "老师，第二章资料在哪里下载？")
	if err != nil {
		t.Fatalf("student comment: %v", err)
	}
	if item.AuthorName != "张三1234" {
		t.Fatalf("author hydrate mismatch: %q", item.AuthorName)
	}

	if _, err := tc.svc.CreateComment(studentCtx, ann.ID, "   "); err == nil {
		t.Fatal("empty comment must be rejected")
	}

	otherCtx := tc.withCaller(context.Background(), types.TenantRoleViewer, "u-other")
	if err := tc.svc.DeleteComment(otherCtx, ann.ID, item.ID); err == nil {
		t.Fatal("other student must not delete the comment")
	}
	if err := tc.svc.DeleteComment(studentCtx, ann.ID, item.ID); err != nil {
		t.Fatalf("author delete own comment: %v", err)
	}

	second, err := tc.svc.CreateComment(studentCtx, ann.ID, "再来一条")
	if err != nil {
		t.Fatalf("student comment: %v", err)
	}
	adminCtx := tc.withCaller(context.Background(), types.TenantRoleOwner, "u-teacher")
	if err := tc.svc.DeleteComment(adminCtx, ann.ID, second.ID); err != nil {
		t.Fatalf("admin delete comment: %v", err)
	}

	if _, err := tc.svc.CreateComment(studentCtx, "missing-id", "hi"); err == nil {
		t.Fatal("missing announcement must 404")
	}
}

// Ticket 12: deleting someone else's comment must look exactly like
// deleting a missing one (404), never a 403 — existence is not leaked.
func TestAnnouncements_DeleteComment_ForeignLooksLikeMissing(t *testing.T) {
	tc := newAnnouncementTestCtx(t)
	ann, err := tc.svc.Create(tc.ctx(), "t", "c", nil)
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	studentCtx := tc.withCaller(context.Background(), types.TenantRoleViewer, "u-student")
	item, err := tc.svc.CreateComment(studentCtx, ann.ID, "只有我能删的留言")
	if err != nil {
		t.Fatalf("student comment: %v", err)
	}

	otherCtx := tc.withCaller(context.Background(), types.TenantRoleViewer, "u-other")
	err = tc.svc.DeleteComment(otherCtx, ann.ID, item.ID)
	if err == nil {
		t.Fatal("other student must not delete the comment")
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("foreign comment delete must surface as not-found (404), got %v", err)
	}
	// Same shape as a genuinely missing comment.
	if err := tc.svc.DeleteComment(otherCtx, ann.ID, "no-such-comment"); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("missing comment must be not-found too, got %v", err)
	}

	comments, err := tc.svc.ListComments(tc.ctx(), ann.ID)
	if err != nil || len(comments) != 1 {
		t.Fatalf("comment must survive the foreign delete: n=%d err=%v", len(comments), err)
	}
}
