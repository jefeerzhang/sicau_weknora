package types_test

import (
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
)

func TestEffectiveStudentPersonalModels_DefaultOff(t *testing.T) {
	got := types.EffectiveStudentPersonalModels(nil)
	if got.Enabled {
		t.Fatal("nil config must default to enabled=false")
	}
}

func TestStudentPersonalModelHostAllowed_Preset(t *testing.T) {
	if !types.StudentPersonalModelHostAllowed("https://api.siliconflow.cn/v1", nil) {
		t.Fatal("siliconflow preset should allow")
	}
	if types.StudentPersonalModelHostAllowed("https://evil.example/v1", nil) {
		t.Fatal("unknown host must reject without teacher append")
	}
}

func TestStudentPersonalModelHostAllowed_TeacherAppend(t *testing.T) {
	if !types.StudentPersonalModelHostAllowed("https://llm.sicau.edu.cn/v1", []string{"sicau.edu.cn"}) {
		t.Fatal("teacher-appended suffix should allow")
	}
}
