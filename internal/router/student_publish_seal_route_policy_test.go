package router

import (
	"os"
	"strings"
	"testing"
)

// sicau-v1 ADR-009-7 / issues #4–#5: students (viewers) must not reach
// publish-integration management reads or personal sandbox-secret APIs.
// Tripwires parse registration source the same way as
// session_artifact_route_policy_test.go.

func TestMyEnvVarRoutes_RequireContributor(t *testing.T) {
	src, err := os.ReadFile("routes_auth_tenant.go")
	if err != nil {
		t.Fatalf("read routes_auth_tenant.go: %v", err)
	}
	text := string(src)
	if !strings.Contains(text, "RegisterMyEnvVarRoutes") {
		t.Fatal("RegisterMyEnvVarRoutes missing; update this tripwire if renamed")
	}
	// Group or every handler line must require Contributor+.
	if !strings.Contains(text, `/me/env-vars`) {
		t.Fatal("/me/env-vars registration not found")
	}
	blockStart := strings.Index(text, "func RegisterMyEnvVarRoutes")
	if blockStart < 0 {
		t.Fatal("RegisterMyEnvVarRoutes func not found")
	}
	block := text[blockStart:]
	if end := strings.Index(block[1:], "\nfunc "); end >= 0 {
		block = block[:end+1]
	}
	if !strings.Contains(block, "g.Contributor()") {
		t.Fatalf("RegisterMyEnvVarRoutes must gate with g.Contributor(), got:\n%s", block)
	}
	if strings.Contains(block, "g.Viewer()") {
		t.Fatalf("RegisterMyEnvVarRoutes must not use g.Viewer():\n%s", block)
	}
}

func TestIMChannelListRoutes_RequireContributor(t *testing.T) {
	src, err := os.ReadFile("routes_agent.go")
	if err != nil {
		t.Fatalf("read routes_agent.go: %v", err)
	}
	cases := []struct {
		label  string
		needle string
	}{
		{"agent IM channel list", `imHandler.ListIMChannels`},
		{"all IM channel list", `imHandler.ListAllIMChannels`},
	}
	lines := strings.Split(string(src), "\n")
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			found := false
			for _, line := range lines {
				if !strings.Contains(line, tc.needle) {
					continue
				}
				found = true
				if !strings.Contains(line, "g.Contributor()") {
					t.Fatalf("%s must use g.Contributor(), got:\n%s", tc.label, strings.TrimSpace(line))
				}
				if strings.Contains(line, "g.Viewer()") {
					t.Fatalf("%s must not use g.Viewer():\n%s", tc.label, strings.TrimSpace(line))
				}
			}
			if !found {
				t.Fatalf("%s registration not found; update tripwire if handler renamed", tc.label)
			}
		})
	}
}

func TestEmbedChannelManagementReads_RequireContributor(t *testing.T) {
	src, err := os.ReadFile("routes_agent.go")
	if err != nil {
		t.Fatalf("read routes_agent.go: %v", err)
	}
	cases := []struct {
		label  string
		needle string
	}{
		{"agent embed list", `embedHandler.ListEmbedChannels`},
		{"all embed list", `embedHandler.ListAllEmbedChannels`},
		{"embed get", `embedHandler.GetEmbedChannel`},
		{"embed preview session", `embedHandler.IssuePreviewSession`},
		{"embed stats", `embedHandler.GetEmbedChannelStats`},
	}
	lines := strings.Split(string(src), "\n")
	for _, tc := range cases {
		t.Run(tc.label, func(t *testing.T) {
			found := false
			for _, line := range lines {
				if !strings.Contains(line, tc.needle) {
					continue
				}
				found = true
				if !strings.Contains(line, "g.Contributor()") {
					t.Fatalf("%s must use g.Contributor(), got:\n%s", tc.label, strings.TrimSpace(line))
				}
				if strings.Contains(line, "g.Viewer()") {
					t.Fatalf("%s must not use g.Viewer():\n%s", tc.label, strings.TrimSpace(line))
				}
			}
			if !found {
				t.Fatalf("%s registration not found; update tripwire if handler renamed", tc.label)
			}
		})
	}
}
