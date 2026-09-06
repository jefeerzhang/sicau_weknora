package deploydefaults_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ticket G / I14 I15: teaching deploy examples must keep Docker sandbox and
// complex-password off by default. Auth classroom defaults (invite-only,
// tenantless, no self-service tenant create) are pinned here so a v0.8.0
// compose refresh cannot silently reopen public registration.

func repoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	// internal/deploydefaults -> repo root
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readRepoFile(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(repoRoot(t), rel))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(b)
}

func TestEnvExample_TeachingSandboxAndPasswordDefaults(t *testing.T) {
	text := readRepoFile(t, ".env.example")
	if !strings.Contains(text, "WEKNORA_SANDBOX_DOCKER_ENABLED=false") {
		t.Fatal(".env.example must set WEKNORA_SANDBOX_DOCKER_ENABLED=false")
	}
	if !strings.Contains(text, "WEKNORA_AUTH_COMPLEX_PASSWORD_ENABLED=false") {
		t.Fatal(".env.example must document WEKNORA_AUTH_COMPLEX_PASSWORD_ENABLED=false")
	}
}

func TestDockerCompose_TeachingDeployDefaults(t *testing.T) {
	text := readRepoFile(t, "docker-compose.yml")
	needles := []string{
		"WEKNORA_SANDBOX_DOCKER_ENABLED=${WEKNORA_SANDBOX_DOCKER_ENABLED:-false}",
		"WEKNORA_AUTH_COMPLEX_PASSWORD_ENABLED=${WEKNORA_AUTH_COMPLEX_PASSWORD_ENABLED:-false}",
		"DISABLE_REGISTRATION=${DISABLE_REGISTRATION:-true}",
		"WEKNORA_AUTH_REGISTRATION_MODE=${WEKNORA_AUTH_REGISTRATION_MODE:-invite_only}",
		"WEKNORA_AUTH_DEFAULT_TENANT_MODE=${WEKNORA_AUTH_DEFAULT_TENANT_MODE:-tenantless}",
		"WEKNORA_TENANT_SELF_SERVICE_CREATION_ENABLED=${WEKNORA_TENANT_SELF_SERVICE_CREATION_ENABLED:-false}",
	}
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Fatalf("docker-compose.yml missing teaching default %q", n)
		}
	}
}
