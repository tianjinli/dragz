package httpapi

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/ctxkey"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/store"
)

type saveNoticeRequest struct {
	Title      string          `json:"title"`
	NoticeType string          `json:"noticeType"`
	Priority   string          `json:"priority"`
	Content    string          `json:"content"`
	TargetType string          `json:"targetType"`
	TargetIDs  json.RawMessage `json:"targetIds"`
	IsPinned   bool            `json:"isPinned"`
	PublishAt  *time.Time      `json:"publishAt"`
}

func listNoticesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListNotices(r.Context(), query.Get("keyword"), query.Get("status"), page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_list_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func getNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		readerID, _ := currentUserID(r.Context())
		item, err := ctx.Auth.NoticeDetail(r.Context(), id, readerID)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.notice.not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_detail_failed")
			return
		}
		ok(w, r, item)
	}
}

func createNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveNoticeRequest
		if !decode(w, r, &req) || !validateNotice(w, r, req) {
			return
		}
		creatorID, _ := currentUserID(r.Context())
		id, err := ctx.Auth.CreateNotice(r.Context(), noticeInput(req), creatorID)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		var req saveNoticeRequest
		if !decode(w, r, &req) || !validateNotice(w, r, req) {
			return
		}
		if err := ctx.Auth.UpdateNotice(r.Context(), id, noticeInput(req)); err != nil {
			writeNoticeError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func publishNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		publisherID, _ := currentUserID(r.Context())
		tenantID, _ := ctxkey.TenantIDFrom(r.Context())
		body, _ := json.Marshal(map[string]any{"noticeId": id, "publishedBy": publisherID})
		if err := ctx.Auth.PublishNoticeWithOutbox(r.Context(), id, publisherID, tenantID, "notice.published", body); err != nil {
			writeNoticeError(w, r, err)
			return
		}
		notifyNoticePublished(ctx, r, id, publisherID)
		ok(w, r, map[string]bool{"success": true})
	}
}

func revokeNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		window, err := ctx.Auth.NoticeRevokeWindowMinutes(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice.failed")
			return
		}
		if window > 0 {
			publisherID, _ := currentUserID(r.Context())
			detail, err := ctx.Auth.NoticeDetail(r.Context(), id, publisherID)
			if err != nil {
				writeNoticeError(w, r, err)
				return
			}
			if detail.PublishAt != nil && time.Since(*detail.PublishAt) > time.Duration(window)*time.Minute {
				writeNoticeError(w, r, store.ErrNoticeRevokeExpired)
				return
			}
		}
		// Resolve audience while the notice is still published, then revoke and sync-delete over WS.
		audienceIDs, audienceErr := ctx.Auth.ListNoticeAudienceUserIDs(r.Context(), id)
		if err := ctx.Auth.RevokeNotice(r.Context(), id); err != nil {
			writeNoticeError(w, r, err)
			return
		}
		if audienceErr != nil {
			logx.Errorf("notice %d audience before revoke: %v", id, audienceErr)
		} else {
			notifyNoticeRevoked(ctx, r, id, audienceIDs)
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteNotice(r.Context(), id); err != nil {
			writeNoticeError(w, r, err)
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func listMyNoticesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, _ := currentUserID(r.Context())
		query := r.URL.Query()
		page, pageSize := queryInt(query.Get("page"), 1), queryInt(query.Get("pageSize"), 20)
		items, total, err := ctx.Auth.ListMyNotices(r.Context(), userID, query.Get("unreadOnly") == "true", page, pageSize)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_inbox_failed")
			return
		}
		ok(w, r, map[string]any{"items": items, "total": total, "page": page, "pageSize": pageSize})
	}
}

func markNoticeReadHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		userID, _ := currentUserID(r.Context())
		if _, err := ctx.Auth.MyNoticeDetail(r.Context(), id, userID); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.notice_message_not_found")
			return
		}
		if err := ctx.Auth.MarkNoticeRead(r.Context(), id, userID); err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_mark_read_failed")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func getMyNoticeHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_id_invalid")
			return
		}
		userID, _ := currentUserID(r.Context())
		item, err := ctx.Auth.MyNoticeDetail(r.Context(), id, userID)
		if errors.Is(err, sql.ErrNoRows) {
			fail(w, r, http.StatusNotFound, 40401, "errors.notice_message_not_found")
			return
		}
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.notice_message_detail_failed")
			return
		}
		ok(w, r, item)
	}
}

func normalizeNoticeType(raw string) string {
	switch strings.TrimSpace(raw) {
	case "notification":
		return "notification"
	case "announcement", "notice", "":
		return "announcement"
	default:
		return ""
	}
}

func validateNotice(w http.ResponseWriter, r *http.Request, req saveNoticeRequest) bool {
	if strings.TrimSpace(req.Title) == "" || len(req.Title) > 255 || strings.TrimSpace(req.Content) == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.notice_fields_invalid")
		return false
	}
	if normalizeNoticeType(req.NoticeType) == "" {
		fail(w, r, http.StatusBadRequest, 40001, "errors.notice_type_invalid")
		return false
	}
	switch req.Priority {
	case "normal", "important", "urgent":
	default:
		fail(w, r, http.StatusBadRequest, 40001, "errors.notice_priority_invalid")
		return false
	}
	noticeType := normalizeNoticeType(req.NoticeType)
	if noticeType == "announcement" {
		return true
	}
	switch req.TargetType {
	case "user", "department", "role":
		if len(req.TargetIDs) == 0 || !json.Valid(req.TargetIDs) || string(req.TargetIDs) == "[]" || string(req.TargetIDs) == "null" {
			fail(w, r, http.StatusBadRequest, 40001, "errors.notice_targets_required")
			return false
		}
		return true
	default:
		fail(w, r, http.StatusBadRequest, 40001, "errors.notice_scope_invalid")
		return false
	}
}

func noticeInput(req saveNoticeRequest) store.SaveNoticeInput {
	noticeType := normalizeNoticeType(req.NoticeType)
	targetType := "all"
	targetIDs := json.RawMessage("[]")
	if noticeType == "notification" {
		targetType = req.TargetType
		if len(req.TargetIDs) > 0 && json.Valid(req.TargetIDs) {
			targetIDs = req.TargetIDs
		}
	}
	return store.SaveNoticeInput{
		Title:      strings.TrimSpace(req.Title),
		NoticeType: noticeType,
		Priority:   req.Priority,
		Content:    req.Content,
		TargetType: targetType,
		TargetIDs:  targetIDs,
		IsPinned:   req.IsPinned,
		PublishAt:  req.PublishAt,
	}
}

func writeNoticeError(w http.ResponseWriter, r *http.Request, err error) {
	failHTTP(w, r, apierrors.MapNotice(err))
}
