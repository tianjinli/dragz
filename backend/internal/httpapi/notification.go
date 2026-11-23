package httpapi

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strings"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/noticenotify"
	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
)

var notifyChannelPattern = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_-]{0,31}$`)

type notifyPrefItem struct {
	Channel string `json:"channel"`
	Enabled bool   `json:"enabled"`
}

type putNotifyPrefsRequest struct {
	Items []notifyPrefItem `json:"items"`
}

type testNotificationRequest struct {
	UserID  uint64 `json:"userId"`
	Channel string `json:"channel"`
	Title   string `json:"title"`
	Content string `json:"content"`
	BizType string `json:"bizType"`
	BizID   string `json:"bizId"`
}

func listMyNotificationsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := currentUserID(r.Context())
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListMyNotifications(r.Context(), userID, query.Get("unreadOnly") == "true", query.Get("channel"), page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func markNotificationReadHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_id_invalid")
			return
		}
		userID, _ := currentUserID(r.Context())
		if err := ctx.Auth.MarkNotificationRead(r.Context(), id, userID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				fail(w, r, http.StatusNotFound, 40401, "errors.notification_not_found")
				return
			}
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_mark_read_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
		pushUnread(ctx, r, userID)
	}
}

func markAllNotificationsReadHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := currentUserID(r.Context())
		channel := strings.TrimSpace(r.URL.Query().Get("channel"))
		if channel != "" && !validNotifyChannel(channel) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_channel_invalid")
			return
		}
		if err := ctx.Auth.MarkAllNotificationsRead(r.Context(), userID, channel); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_read_all_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
		pushUnread(ctx, r, userID)
	}
}

func listNotifyPrefsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := currentUserID(r.Context())
		items, err := ctx.Auth.ListNotifyPrefsForUser(r.Context(), userID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_prefs_load_failed")
			return
		}
		ok(w, r, map[string]any{"items": items})
	}
}

func putNotifyPrefsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req putNotifyPrefsRequest
		if !decode(w, r, &req) {
			return
		}
		if len(req.Items) == 0 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_prefs_invalid")
			return
		}
		for i := range req.Items {
			req.Items[i].Channel = strings.TrimSpace(req.Items[i].Channel)
			if !validNotifyChannel(req.Items[i].Channel) {
				fail(w, r, http.StatusBadRequest, 40001, "errors.notification_channel_invalid")
				return
			}
		}
		userID, _ := currentUserID(r.Context())
		for _, item := range req.Items {
			if err := ctx.Auth.UpsertNotifyPref(r.Context(), userID, item.Channel, item.Enabled); err != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_prefs_save_failed")
				return
			}
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func testNotificationHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req testNotificationRequest
		if !decode(w, r, &req) {
			return
		}
		req.Channel = strings.TrimSpace(req.Channel)
		if !validNotifyChannel(req.Channel) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_channel_invalid")
			return
		}
		title := strings.TrimSpace(req.Title)
		if title == "" || len(title) > 255 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_fields_invalid")
			return
		}
		userID := req.UserID
		if userID == 0 {
			userID, _ = currentUserID(r.Context())
		}
		if userID == 0 {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notification_fields_invalid")
			return
		}
		item, created, err := ctx.Auth.CreateUserNotification(r.Context(), store.SaveNotificationInput{
			UserID: userID, Channel: req.Channel, Title: title, Content: req.Content,
			BizType: strings.TrimSpace(req.BizType), BizID: strings.TrimSpace(req.BizID),
		})
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notification_test_failed")
			return
		}
		if created {
			pushEvent(ctx, r, []uint64{userID}, map[string]any{"type": "notification", "data": item})
			pushUnread(ctx, r, userID)
		}
		ok(w, r, map[string]any{"id": item.ID, "created": created})
	}
}

func notifyNoticePublished(ctx *app.Context, r *http.Request, noticeID, publisherID uint64) {
	if ctx.WS == nil {
		return
	}
	item, err := ctx.Auth.NoticeDetail(r.Context(), noticeID, publisherID)
	if err != nil {
		logx.Errorf("notice %d detail: %v", noticeID, err)
		return
	}
	if _, err := noticenotify.DeliverOne(r.Context(), ctx.Auth, ctx.WS, item); err != nil {
		logx.Errorf("notice %d deliver: %v", noticeID, err)
	}
}

func notifyNoticeRevoked(ctx *app.Context, r *http.Request, noticeID uint64, audienceIDs []uint64) {
	if ctx.WS == nil || len(audienceIDs) == 0 {
		return
	}
	pushEvent(ctx, r, audienceIDs, map[string]any{
		"type": "notice_revoked",
		"data": map[string]any{"id": noticeID},
	})
	pushUnread(ctx, r, audienceIDs...)
}

func pushUnread(ctx *app.Context, r *http.Request, userIDs ...uint64) {
	if ctx.WS == nil {
		return
	}
	for _, userID := range userIDs {
		counts, err := ctx.Auth.UnreadMessageCounts(r.Context(), userID)
		if err != nil {
			logx.Errorf("unread counts for %d: %v", userID, err)
			continue
		}
		pushEvent(ctx, r, []uint64{userID}, map[string]any{"type": "unread", "data": counts})
	}
}

// pushEvent delivers a realtime event inside the request's tenant. The tenant
// comes from ctx rather than from the caller so no handler can accidentally
// address another tenant's sockets.
func pushEvent(ctx *app.Context, r *http.Request, userIDs []uint64, payload any) {
	if ctx == nil || ctx.WS == nil || len(userIDs) == 0 {
		return
	}
	tenantID, _ := ctxkey.TenantIDFrom(r.Context())
	if tenantID == 0 {
		logx.Errorf("notification push skipped: no tenant on request")
		return
	}
	if err := ctx.WS.Push(tenantID, userIDs, payload); err != nil {
		logx.Errorf("notification push: %v", err)
	}
}

func validNotifyChannel(channel string) bool {
	return notifyChannelPattern.MatchString(channel)
}
