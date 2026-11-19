package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Notification is one row in a user's in-app inbox.
type Notification struct {
	ID        uint64     `json:"id"`
	UserID    uint64     `json:"userId"`
	Channel   string     `json:"channel"`
	Title     string     `json:"title"`
	Content   string     `json:"content"`
	BizType   string     `json:"bizType"`
	BizID     string     `json:"bizId"`
	Read      bool       `json:"read"`
	ReadAt    *time.Time `json:"readAt"`
	CreatedAt time.Time  `json:"createdAt"`
}

// SaveNotificationInput is the payload for one user-targeted notification.
type SaveNotificationInput struct {
	UserID  uint64
	Channel string
	Title   string
	Content string
	BizType string
	BizID   string
}

// NotifyPref is one channel switch. A channel with no row is treated as enabled.
type NotifyPref struct {
	Channel   string    `json:"channel"`
	Enabled   bool      `json:"enabled"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// UnreadMessageCounts is the badge split between announcements and the inbox.
type UnreadMessageCounts struct {
	Notices       int64 `json:"notices"`
	Notifications int64 `json:"notifications"`
	Total         int64 `json:"total"`
}

// CreateUserNotification inserts a notification for the user. Preferences were
// removed from the product surface, so delivery is no longer gated by channel.
func (r *AuthRepository) CreateUserNotification(ctx context.Context, input SaveNotificationInput) (Notification, bool, error) {
	row, err := r.queries.CreateNotification(ctx, sqlcstore.CreateNotificationParams{
		UserID: input.UserID, Channel: input.Channel, Title: input.Title,
		Content: input.Content, BizType: input.BizType, BizID: input.BizID,
	})
	if err != nil {
		return Notification{}, false, err
	}
	return notificationFrom(row), true, nil
}

// ListMyNotifications returns one page of a user's inbox.
func (r *AuthRepository) ListMyNotifications(ctx context.Context, userID uint64, unreadOnly bool, channel string, page, pageSize int) ([]Notification, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	filter := sqlcstore.CountMyNotificationsParams{
		UserID: userID, UnreadOnly: unreadOnly, Channel: optionalString(channel),
	}
	total, err := r.queries.CountMyNotifications(ctx, filter)
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListMyNotifications(ctx, sqlcstore.ListMyNotificationsParams{
		UserID: userID, UnreadOnly: unreadOnly, Channel: filter.Channel,
		RowLimit: int32(pageSize), RowOffset: int32((page - 1) * pageSize),
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Notification, 0, len(rows))
	for _, row := range rows {
		items = append(items, notificationFrom(row))
	}
	return items, total, nil
}

// GetNotificationForUser loads one inbox row owned by the user.
func (r *AuthRepository) GetNotificationForUser(ctx context.Context, id, userID uint64) (Notification, error) {
	row, err := r.queries.GetNotificationForUser(ctx, sqlcstore.GetNotificationForUserParams{
		ID: int64(id), UserID: userID,
	})
	if err != nil {
		return Notification{}, err
	}
	return notificationFrom(row), nil
}

// MarkNotificationRead stamps read_at. Marking an already-read row is a no-op;
// a row the user does not own is sql.ErrNoRows.
func (r *AuthRepository) MarkNotificationRead(ctx context.Context, id, userID uint64) error {
	affected, err := r.queries.MarkNotificationRead(ctx, sqlcstore.MarkNotificationReadParams{
		ID: int64(id), UserID: userID,
	})
	if err != nil {
		return err
	}
	if affected > 0 {
		return nil
	}
	_, err = r.queries.GetNotificationForUser(ctx, sqlcstore.GetNotificationForUserParams{
		ID: int64(id), UserID: userID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return err
	}
	return err
}

// MarkAllNotificationsRead stamps every unread row for the user.
// An empty channel marks every channel.
func (r *AuthRepository) MarkAllNotificationsRead(ctx context.Context, userID uint64, channel string) error {
	_, err := r.queries.MarkAllNotificationsRead(ctx, sqlcstore.MarkAllNotificationsReadParams{
		UserID: userID, Channel: optionalString(channel),
	})
	return err
}

// UpsertNotifyPref stores whether a channel may deliver to the user.
func (r *AuthRepository) UpsertNotifyPref(ctx context.Context, userID uint64, channel string, enabled bool) error {
	return r.queries.UpsertNotifyPref(ctx, sqlcstore.UpsertNotifyPrefParams{
		UserID: userID, Channel: channel, Enabled: boolSmallInt16(enabled),
	})
}

// ListNotifyPrefsForUser returns the stored switches. Missing channels stay enabled.
func (r *AuthRepository) ListNotifyPrefsForUser(ctx context.Context, userID uint64) ([]NotifyPref, error) {
	rows, err := r.queries.ListNotifyPrefsForUser(ctx, userID)
	if err != nil {
		return nil, err
	}
	items := make([]NotifyPref, 0, len(rows))
	for _, row := range rows {
		items = append(items, NotifyPref{
			Channel: row.Channel, Enabled: smallIntBool(row.Enabled), UpdatedAt: row.UpdatedAt,
		})
	}
	return items, nil
}

// UnreadMessageCounts adds the user's unread announcements to the unread inbox.
func (r *AuthRepository) UnreadMessageCounts(ctx context.Context, userID uint64) (UnreadMessageCounts, error) {
	notices, err := r.queries.CountMyNotices(ctx, sqlcstore.CountMyNoticesParams{
		UserID: userID, UnreadOnly: true,
	})
	if err != nil {
		return UnreadMessageCounts{}, err
	}
	notifications, err := r.queries.CountUnreadNotifications(ctx, userID)
	if err != nil {
		return UnreadMessageCounts{}, err
	}
	return UnreadMessageCounts{
		Notices: notices, Notifications: notifications, Total: notices + notifications,
	}, nil
}

// ListNoticeAudienceUserIDs returns the active users a published notice targets.
func (r *AuthRepository) ListNoticeAudienceUserIDs(ctx context.Context, noticeID uint64) ([]uint64, error) {
	rows, err := r.queries.ListNoticeAudienceUserIDs(ctx, int64(noticeID))
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, 0, len(rows))
	for _, id := range rows {
		if id > 0 {
			ids = append(ids, uint64(id))
		}
	}
	return ids, nil
}

func notificationFrom(row sqlcstore.SysNotification) Notification {
	return Notification{
		ID: uint64(row.ID), UserID: row.UserID, Channel: row.Channel, Title: row.Title,
		Content: row.Content, BizType: row.BizType, BizID: row.BizID,
		Read: row.ReadAt.Valid, ReadAt: nullTimePtr(row.ReadAt), CreatedAt: row.CreatedAt,
	}
}
