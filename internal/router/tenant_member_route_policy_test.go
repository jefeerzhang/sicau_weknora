package router

import (
	"os"
	"strings"
	"testing"
)

// TestTenantMemberRoutePolicies_PinnedToAdmin pins the sicau-v1 ticket-01
// policy: reading the member roster (GET /:id/members) and pending
// invitations (GET /:id/invitations) is Admin+ only — students (viewers)
// must never see who else is in the course workspace. The invitations list
// carries invitee emails, so it is in scope alongside the roster.
//
// The gin engine does not expose per-route middleware chains for reflection,
// so the cheapest way to pin the wiring itself is to parse the registration
// source. If someone reverts either route to g.Viewer() (e.g. via an
// upstream merge), this test fails and forces a conscious decision.
//
// Out of scope by design: POST /:id/leave stays Viewer+ — quitting the
// tenant is the student's own action and leaks nothing about others.
func TestTenantMemberRoutePolicies_PinnedToAdmin(t *testing.T) {
	src, err := os.ReadFile("routes_auth_tenant.go")
	if err != nil {
		t.Fatalf("read routes_auth_tenant.go: %v", err)
	}

	routes := []struct {
		label string
		needle string
	}{
		{label: "GET /members roster", needle: `http.MethodGet, "/members"`},
		{label: "GET /invitations pending list", needle: `http.MethodGet, "/invitations"`},
	}

	lines := strings.Split(string(src), "\n")
	for _, route := range routes {
		t.Run(route.label, func(t *testing.T) {
			found := false
			for _, line := range lines {
				if !strings.Contains(line, route.needle) {
					continue
				}
				found = true
				if !strings.Contains(line, "g.Admin()") {
					t.Fatalf("route %s must be registered with g.Admin(), got:\n%s",
						route.needle, strings.TrimSpace(line))
				}
				if strings.Contains(line, "g.Viewer()") {
					t.Fatalf("route %s must not use g.Viewer():\n%s",
						route.needle, strings.TrimSpace(line))
				}
			}
			if !found {
				t.Fatalf("registration for %s not found in routes_auth_tenant.go; "+
					"if the route moved, update this tripwire", route.needle)
			}
		})
	}
}
