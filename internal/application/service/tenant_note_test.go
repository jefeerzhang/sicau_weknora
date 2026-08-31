package service

import (
	"strings"
	"testing"
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
