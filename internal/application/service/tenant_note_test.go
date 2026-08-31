package service

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Tencent/WeKnora/internal/application/repository"
	"github.com/Tencent/WeKnora/internal/types"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// sicau-v1 notes design N-6: 列表标题由服务端派生。
func TestDeriveNoteTitle(t *testing.T) {
	nl := "\n"
	cases := []struct {
		name   string
		head   string
		expect string
	}{
		{"heading wins", "# 会议纪要" + nl + "正文", "会议纪要"},
		{"deep heading", "###标签整理", "标签整理"},
		// Ticket 06 N-6: the first `#` heading wins even when a plain
		// line comes first.
		{"heading after plain line wins", "乡村建设行动方案" + nl + "# 真正的标题", "真正的标题"},
		{"first line fallback", "乡村建设行动方案" + nl + "后面没有井号", "乡村建设行动方案"},
		{"bare hashes skipped", "###" + nl + "正文开始", "正文开始"},
		{"truncated to 50 runes", strings.Repeat("长", 80), strings.Repeat("长", 50)},
		{"empty", "", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := deriveNoteTitle(tc.head); got != tc.expect {
				t.Fatalf("got %q want %q", got, tc.expect)
			}
		})
	}
}

// Update that drops an image reference must GC the image when no other
// note still points at it — otherwise the 100MB quota leaks.
func TestNotes_Update_GCsUnreferencedImages(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "notes.db")), &gorm.Config{})
	if err != nil {
		t.Fatalf("sqlite: %v", err)
	}
	if err := db.AutoMigrate(&types.TenantNote{}, &types.TenantNoteImage{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	repo := repository.NewTenantNoteRepository(db)
	svc := NewTenantNoteService(repo)

	ctx := context.Background()
	ctx = context.WithValue(ctx, types.TenantIDContextKey, uint64(7))
	ctx = context.WithValue(ctx, types.UserIDContextKey, "u-student")

	png := []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	img, err := svc.CreateImage(ctx, png)
	if err != nil {
		t.Fatalf("CreateImage: %v", err)
	}
	md := "see ![](" + types.NoteImageURLPrefix + img.ID + ")\n"
	note, err := svc.Create(ctx, md)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := svc.Update(ctx, note.ID, "no images left"); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if _, err := svc.GetImage(ctx, img.ID); !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("unreferenced image must be GC'd after Update, got err=%v", err)
	}
}
