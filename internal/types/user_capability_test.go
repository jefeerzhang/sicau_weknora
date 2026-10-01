package types

import "testing"

// #13: SuperAdmin is a composite identity — platform governance plus
// inherited teacher capability. HasTeacherCapability must hold for it
// regardless of the stored IsTeacher flag, so revoking a teacher
// appointment can never strip a SuperAdmin of its teacher capability.
func TestUserHasTeacherCapabilityCompositeSuperAdmin(t *testing.T) {
	cases := []struct {
		name string
		user User
		want bool
	}{
		{"appointed teacher", User{IsTeacher: true}, true},
		{"superadmin only (teacher flag false)", User{IsSystemAdmin: true}, true},
		{"superadmin whose teacher flag was revoked", User{IsSystemAdmin: true, IsTeacher: false}, true},
		{"student", User{}, false},
		{"unappointed account", User{IsActive: true}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.user.HasTeacherCapability(); got != tc.want {
				t.Fatalf("HasTeacherCapability()=%v, want %v (user=%+v)", got, tc.want, tc.user)
			}
		})
	}
}
