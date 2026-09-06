// Bootstrap-time hooks that run after the DI container is built but
// before the HTTP server starts listening.
package main

import (
	"context"
	"os"

	"go.uber.org/dig"

	"github.com/Tencent/WeKnora/internal/bootstrap"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
)

// runStartupBootstrap consults the env and applies one-shot bootstrap
// actions. SuperAdmin ensure (#8) is fail-closed: a non-nil error aborts
// process startup so the deployment never runs without a clear authority root.
func runStartupBootstrap(c *dig.Container) error {
	ctx := context.Background()

	// Legacy hash repair for migration 000065 placeholder rows.
	if err := c.Invoke(func(apiKeySvc interfaces.TenantAPIKeyService) {
		if n, err := apiKeySvc.BackfillMissingKeyHashes(ctx); err != nil {
			logger.Warnf(ctx, "[bootstrap] tenant api key hash backfill failed: %v", err)
		} else if n > 0 {
			logger.Infof(ctx, "[bootstrap] backfilled %d legacy tenant api key hash(es)", n)
		}
	}); err != nil {
		logger.Warnf(ctx, "[bootstrap] failed to resolve TenantAPIKeyService: %v", err)
	}

	var ensureErr error
	if err := c.Invoke(func(userRepo interfaces.UserRepository) {
		cfg := bootstrap.ConfigFromEnv(os.Getenv)
		user, err := bootstrap.EnsureSuperAdmin(ctx, userRepo, cfg)
		if err != nil {
			ensureErr = err
			logger.Errorf(ctx, "[bootstrap] SuperAdmin ensure failed: %v", err)
			return
		}
		if user != nil {
			logger.Infof(ctx, "[bootstrap] created SuperAdmin %s (%s); must_change_password=true",
				user.ID, user.Email)
		} else {
			logger.Infof(ctx, "[bootstrap] SuperAdmin invariant OK")
		}
	}); err != nil {
		return err
	}
	return ensureErr
}
