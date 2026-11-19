package noticenotify

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/store"
)

// StartDispatcher polls for due scheduled notices and pushes them over WS.
// Returns a stop function suitable for app.Context.Defer.
func StartDispatcher(repo *store.AuthRepository, push Pusher, every time.Duration) func() {
	if every <= 0 {
		every = 30 * time.Second
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		if err := repo.BackfillLegacyNoticeNotified(ctx); err != nil {
			logx.Errorf("notice notified_at backfill: %v", err)
		}
		// First pass soon after boot so near-due notices are not waiting a full interval.
		runOnce(ctx, repo, push)
		ticker := time.NewTicker(every)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				runOnce(ctx, repo, push)
			}
		}
	}()
	return cancel
}

func runOnce(ctx context.Context, repo *store.AuthRepository, push Pusher) {
	delivered, err := DeliverDue(ctx, repo, push, 50)
	if err != nil {
		logx.Errorf("notice due dispatcher: %v", err)
		return
	}
	if delivered > 0 {
		logx.Infof("notice due dispatcher delivered %d notice(s)", delivered)
	}
}
