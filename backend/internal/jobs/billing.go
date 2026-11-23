package jobs

import (
	"context"
	"errors"
	"fmt"
)

func reconcileSubscriptions(ctx context.Context, env HandlerEnv) (string, error) {
	if env.Tenants == nil {
		return "", errors.New("billing reconcile is not configured")
	}
	stats, err := env.Tenants.ReconcileSubscriptions(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("moved %d subscription(s) to overdue, cancelled %d", stats.Overdue, stats.Cancelled), nil
}

func purgeDueTenants(ctx context.Context, env HandlerEnv) (string, error) {
	if env.Tenants == nil {
		return "", errors.New("tenant purge is not configured")
	}
	n, err := env.Tenants.PurgeDueTenants(ctx)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("purged %d tenant(s)", n), nil
}
