package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Tencent/WeKnora/internal/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestRequirePasswordChangedAllowsChangePassword(t *testing.T) {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		ctx := context.WithValue(c.Request.Context(), types.UserContextKey, &types.User{
			ID:                 "u1",
			MustChangePassword: true,
		})
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	})
	r.Use(RequirePasswordChanged())
	r.POST("/api/v1/auth/change-password", func(c *gin.Context) { c.Status(http.StatusOK) })
	r.GET("/api/v1/models", func(c *gin.Context) { c.Status(http.StatusOK) })

	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/auth/change-password", nil))
	assert.Equal(t, http.StatusOK, w.Code)

	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, httptest.NewRequest(http.MethodGet, "/api/v1/models", nil))
	assert.Equal(t, http.StatusForbidden, w2.Code)
	assert.Contains(t, w2.Body.String(), "must_change_password")
}
