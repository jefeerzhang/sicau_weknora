package router

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Tencent/WeKnora/internal/database"
)

// NewHealthHandler serves GET /health — the probe both the Docker compose
// healthcheck and the Helm liveness/readiness probes point at. The
// teaching data migration blocked state (#24) turns it non-200: while
// legacy elevated memberships may be un-normalized, orchestrators must
// stop routing to this instance and restart it so the next startup
// retries. The error message names schema objects or counts only — it
// never embeds invitation tokens or credentials.
func NewHealthHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		if blocked := database.CachedTeachingMigrationError(); blocked != "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{
				"status":                   "blocked",
				"teaching_migration_error": blocked,
			})
			return
		}
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	}
}
