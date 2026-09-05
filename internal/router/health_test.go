package router

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/database"
	"github.com/gin-gonic/gin"
)

// /health is the probe surface for the Docker healthcheck and the Helm
// liveness/readiness probes (#24): it must go non-200 while the teaching
// data migration is blocked so orchestrators stop routing and restart the
// instance, and return to 200 once a retry clears the state.
func TestHealthHandler_ReflectsTeachingMigrationBlockedState(t *testing.T) {
	t.Cleanup(func() { database.CacheTeachingMigrationError("") })
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/health", NewHealthHandler())
	do := func() *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/health", nil))
		return w
	}

	// Healthy: no blocked state.
	database.CacheTeachingMigrationError("")
	w := do()
	if w.Code != http.StatusOK {
		t.Fatalf("healthy readiness must be 200, got %d body=%s", w.Code, w.Body.String())
	}
	var okBody struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &okBody); err != nil || okBody.Status != "ok" {
		t.Fatalf("healthy body must report ok, got %s", w.Body.String())
	}

	// Blocked: non-200 with the actionable reason.
	database.CacheTeachingMigrationError("teaching data migration requires table \"tenant_members\"")
	w = do()
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("blocked readiness must be 503, got %d body=%s", w.Code, w.Body.String())
	}
	var blockedBody struct {
		Status                 string `json:"status"`
		TeachingMigrationError string `json:"teaching_migration_error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &blockedBody); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if blockedBody.Status != "blocked" || blockedBody.TeachingMigrationError == "" {
		t.Fatalf("blocked body must carry the state and reason, got %s", w.Body.String())
	}

	// Retry cleared the state: readiness returns.
	database.CacheTeachingMigrationError("")
	w = do()
	if w.Code != http.StatusOK {
		t.Fatalf("cleared readiness must be 200, got %d", w.Code)
	}
}
