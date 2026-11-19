// Package noticenotify pushes published notices to their audience when due.
package noticenotify

import (
	"context"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/store"
)

// Pusher fans an event out to the given users (local sockets and/or Redis).
type Pusher interface {
	Push(userIDs []uint64, payload any) error
}

// DeliverOne pushes one published notice when its publish_at is due, then marks
// it notified. Future-dated notices are left for a later pass.
func DeliverOne(ctx context.Context, repo *store.AuthRepository, push Pusher, notice store.Notice) (bool, error) {
	if push == nil || repo == nil {
		return false, nil
	}
	if notice.PublishAt != nil && notice.PublishAt.After(time.Now()) {
		return false, nil
	}
	ids, err := repo.ListNoticeAudienceUserIDs(ctx, notice.ID)
	if err != nil {
		return false, err
	}
	if len(ids) == 0 {
		// Still mark so empty audiences (e.g. deleted targets) do not retry forever.
		_, markErr := repo.MarkNoticeNotified(ctx, notice.ID)
		return false, markErr
	}
	data := map[string]any{
		"id":         notice.ID,
		"title":      notice.Title,
		"content":    notice.Content,
		"noticeType": notice.NoticeType,
		"priority":   notice.Priority,
		"publishAt":  notice.PublishAt,
	}
	if err := push.Push(ids, map[string]any{"type": "notice", "data": data}); err != nil {
		return false, err
	}
	for _, userID := range ids {
		counts, countErr := repo.UnreadMessageCounts(ctx, userID)
		if countErr != nil {
			logx.Errorf("unread counts for %d: %v", userID, countErr)
			continue
		}
		if err := push.Push([]uint64{userID}, map[string]any{"type": "unread", "data": counts}); err != nil {
			logx.Errorf("unread push for %d: %v", userID, err)
		}
	}
	marked, err := repo.MarkNoticeNotified(ctx, notice.ID)
	if err != nil {
		return false, err
	}
	return marked, nil
}

// DeliverDue finds published notices whose publish_at has arrived and pushes them.
func DeliverDue(ctx context.Context, repo *store.AuthRepository, push Pusher, limit int) (int, error) {
	if limit <= 0 {
		limit = 50
	}
	items, err := repo.ListDueNoticesForNotify(ctx, limit)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, item := range items {
		ok, err := DeliverOne(ctx, repo, push, item)
		if err != nil {
			logx.Errorf("deliver notice %d: %v", item.ID, err)
			continue
		}
		if ok {
			delivered++
		}
	}
	return delivered, nil
}
