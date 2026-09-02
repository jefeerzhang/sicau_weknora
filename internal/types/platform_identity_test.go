package types

import "testing"

// PlatformIdentity is the unified platform identity classification exposed as
// the "用户身份标签" (see CONTEXT.md). These cases pin the derivation and the
// defensive "身份未设置" (unset) branch: superadmin (composite) wins over the
// teacher flag; an appointed teacher is a teacher; everything else is a
// student; missing data (nil) is unset and never inferred as a privilege.
func TestUserPlatformIdentity(t *testing.T) {
	cases := []struct {
		name string
		user *User
		want PlatformIdentity
	}{
		{"composite superadmin wins over teacher flag", &User{IsSystemAdmin: true, IsTeacher: false}, PlatformIdentitySuperAdmin},
		{"composite superadmin with teacher flag", &User{IsSystemAdmin: true, IsTeacher: true}, PlatformIdentitySuperAdmin},
		{"appointed teacher", &User{IsTeacher: true}, PlatformIdentityTeacher},
		{"default student", &User{}, PlatformIdentityStudent},
		{"active unappointed account is student", &User{IsActive: true}, PlatformIdentityStudent},
		{"missing identity data is unset", nil, PlatformIdentityUnset},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.user.PlatformIdentity(); got != tc.want {
				t.Fatalf("PlatformIdentity()=%q, want %q (user=%+v)", got, tc.want, tc.user)
			}
		})
	}
}

// ToUserInfo must carry the unified platform identity so every surface that
// returns UserInfo (auth/me, teacher management) can display the label.
func TestToUserInfoCarriesPlatformIdentity(t *testing.T) {
	u := &User{IsSystemAdmin: true, IsTeacher: false}
	info := u.ToUserInfo()
	if info.PlatformIdentity != PlatformIdentitySuperAdmin {
		t.Fatalf("ToUserInfo().PlatformIdentity=%q, want %q", info.PlatformIdentity, PlatformIdentitySuperAdmin)
	}
}
