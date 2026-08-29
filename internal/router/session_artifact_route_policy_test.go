package router

import (
	"os"
	"strings"
	"testing"
)

// TestSessionArtifactRoutePolicies_RequireContributor pins the sicau-v1
// ticket-03 policy: Skill-generated session artifacts (list + streamed
// download) are Contributor+ only. Students (viewers) run a pure-Q&A
// course deployment and must never fetch generated files, so the three
// artifact registrations must carry an explicit g.Contributor() guard on
// top of the Viewer+ sessions group.
//
// Like the tenant-member tripwire, this parses the registration source
// because gin does not expose per-route middleware for reflection. A
// revert (e.g. via upstream merge) turns this red and forces a conscious
// decision.
func TestSessionArtifactRoutePolicies_RequireContributor(t *testing.T) {
	src, err := os.ReadFile("routes_chat.go")
	if err != nil {
		t.Fatalf("read routes_chat.go: %v", err)
	}

	routes := []struct {
		label  string
		needle string
	}{
		{label: "session artifact list", needle: `"/:id/artifacts"`},
		{label: "message artifact list", needle: `"/:id/messages/:message_id/artifacts"`},
		{label: "artifact download stream", needle: `"/:id/messages/:message_id/artifacts/:index/download"`},
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
				if !strings.Contains(line, "g.Contributor()") {
					t.Fatalf("artifact route %s must be registered with g.Contributor(), got:\n%s",
						route.needle, strings.TrimSpace(line))
				}
			}
			if !found {
				t.Fatalf("registration for %s not found in routes_chat.go; "+
					"if the route moved, update this tripwire", route.needle)
			}
		})
	}
}
