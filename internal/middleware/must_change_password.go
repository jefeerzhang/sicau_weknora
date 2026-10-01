package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/types"
)

// passwordChangeAllowedPrefixes are the only authenticated routes usable
// while users.must_change_password is true (sicau #8 bootstrap).
var passwordChangeAllowedExact = map[string]map[string]struct{}{
	"/api/v1/auth/change-password": {http.MethodPost: {}},
	"/api/v1/auth/me":              {http.MethodGet: {}},
	"/api/v1/auth/logout":          {http.MethodPost: {}},
	"/api/v1/auth/refresh":         {http.MethodPost: {}},
}

// RequirePasswordChanged aborts authenticated JWT sessions that still carry
// MustChangePassword, except for the allowlisted auth endpoints.
func RequirePasswordChanged() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodOptions {
			c.Next()
			return
		}
		user, ok := c.Request.Context().Value(types.UserContextKey).(*types.User)
		if !ok || user == nil || !user.MustChangePassword {
			c.Next()
			return
		}
		path := c.Request.URL.Path
		if methods, ok := passwordChangeAllowedExact[path]; ok {
			if _, ok := methods[c.Request.Method]; ok {
				c.Next()
				return
			}
		}
		// Also allow GET /auth/config style public paths that somehow pass Auth.
		if strings.HasPrefix(path, "/api/v1/auth/") &&
			(c.Request.Method == http.MethodGet || c.Request.Method == http.MethodPost) {
			switch path {
			case "/api/v1/auth/change-password", "/api/v1/auth/me", "/api/v1/auth/logout", "/api/v1/auth/refresh":
				c.Next()
				return
			}
		}
		c.JSON(http.StatusForbidden, gin.H{
			"error": "Password change required before continuing",
			"code":  "must_change_password",
		})
		c.Abort()
	}
}
