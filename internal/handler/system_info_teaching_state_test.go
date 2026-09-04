package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/database"
	"github.com/gin-gonic/gin"
)

// /system/info must surface the teaching data migration blocked state so
// operators can see the deployment is not upgraded (#22).
func TestGetSystemInfo_ExposesTeachingMigrationState(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	h := &SystemHandler{}
	r.GET("/system/info", h.GetSystemInfo)

	t.Cleanup(func() { database.CacheTeachingMigrationError("") })

	database.CacheTeachingMigrationError("teaching data migration left 1 item(s) unresolved")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/system/info", nil))
	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			TeachingMigrationError string `json:"teaching_migration_error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v body=%s", err, w.Body.String())
	}
	if resp.Data.TeachingMigrationError == "" {
		t.Fatalf("blocked teaching migration state must be surfaced, body=%s", w.Body.String())
	}

	database.CacheTeachingMigrationError("")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/system/info", nil))
	if w2.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", w2.Code, w2.Body.String())
	}
	var resp2 struct {
		Data struct {
			TeachingMigrationError string `json:"teaching_migration_error"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp2.Data.TeachingMigrationError != "" {
		t.Fatalf("cleared state must disappear from system info, got %q", resp2.Data.TeachingMigrationError)
	}
}
