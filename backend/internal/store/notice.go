package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Notice is one announcement as the admin list shows it. Content is omitted from list responses.
type Notice struct {
	ID          uint64          `json:"id"`
	Title       string          `json:"title"`
	NoticeType  string          `json:"noticeType"`
	Priority    string          `json:"priority"`
	Content     string          `json:"content,omitempty"`
	TargetType  string          `json:"targetType"`
	TargetIDs   json.RawMessage `json:"targetIds"`
	Status      string          `json:"status"`
	IsPinned    bool            `json:"isPinned"`
	PublishAt   *time.Time      `json:"publishAt"`
	PublishedBy *uint64         `json:"publishedBy"`
	CreatedBy   uint64          `json:"createdBy"`
	CreatorName string          `json:"creatorName"`
	Read        bool            `json:"read"`
	ReadCount   int64           `json:"readCount"`
	CreatedAt   time.Time       `json:"createdAt"`
	UpdatedAt   time.Time       `json:"updatedAt"`
}

// SaveNoticeInput is the payload shared by create and update.
type SaveNoticeInput struct {
	Title      string
	NoticeType string
	Priority   string
	Content    string
	TargetType string
	TargetIDs  json.RawMessage
	IsPinned   bool
	PublishAt  *time.Time
}

// ListNotices returns one page of announcements plus the total count.
func (r *AuthRepository) ListNotices(ctx context.Context, keyword, status string, page, pageSize int) ([]Notice, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	pattern := optionalPattern(keyword)
	st := optionalString(status)
	total, err := r.queries.CountNoticesFiltered(ctx, sqlcstore.CountNoticesFilteredParams{
		KeywordPattern: pattern, Status: st,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListNoticesFiltered(ctx, sqlcstore.ListNoticesFilteredParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		KeywordPattern: pattern, Status: st,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Notice, 0, len(rows))
	for _, row := range rows {
		items = append(items, Notice{
			ID: uint64(row.ID), Title: row.Title, NoticeType: row.NoticeType, Priority: row.Priority,
			TargetType: row.TargetType, TargetIDs: nullRawArray(row.TargetIds), Status: row.Status,
			IsPinned: smallIntBool(row.IsPinned), PublishAt: nullTimePtr(row.PublishAt),
			PublishedBy: nullInt64Ptr(row.PublishedBy), CreatedBy: row.CreatedBy, CreatorName: row.CreatorName,
			ReadCount: row.ReadCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, total, nil
}

// NoticeDetail loads one announcement and whether the given reader has already read it.
func (r *AuthRepository) NoticeDetail(ctx context.Context, noticeID, readerID uint64) (Notice, error) {
	row, err := r.queries.GetNoticeDetail(ctx, sqlcstore.GetNoticeDetailParams{
		UserID: readerID, ID: int64(noticeID),
	})
	if err != nil {
		return Notice{}, err
	}
	return Notice{
		ID: uint64(row.ID), Title: row.Title, NoticeType: row.NoticeType, Priority: row.Priority,
		Content: row.Content, TargetType: row.TargetType, TargetIDs: nullRawArray(row.TargetIds),
		Status: row.Status, IsPinned: smallIntBool(row.IsPinned), PublishAt: nullTimePtr(row.PublishAt),
		PublishedBy: nullInt64Ptr(row.PublishedBy), CreatedBy: row.CreatedBy, CreatorName: row.CreatorName,
		Read: row.IsRead, ReadCount: row.ReadCount, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
	}, nil
}

// CreateNotice inserts a draft; publishing is a separate step.
func (r *AuthRepository) CreateNotice(ctx context.Context, input SaveNoticeInput, creatorID uint64) (uint64, error) {
	id, err := r.queries.CreateNotice(ctx, sqlcstore.CreateNoticeParams{
		Title: input.Title, NoticeType: input.NoticeType, Priority: input.Priority, Content: input.Content,
		TargetType: input.TargetType, TargetIds: jsonNullRaw(input.TargetIDs), IsPinned: boolSmallInt16(input.IsPinned),
		PublishAt: nullTimeFromPtr(input.PublishAt), CreatedBy: creatorID,
	})
	return uint64(id), err
}

// UpdateNotice edits a draft. A published notice is refused with ErrNotEditable, so the author has to revoke it first.
func (r *AuthRepository) UpdateNotice(ctx context.Context, noticeID uint64, input SaveNoticeInput) error {
	affected, err := r.queries.UpdateNoticeDraft(ctx, sqlcstore.UpdateNoticeDraftParams{
		Title: input.Title, NoticeType: input.NoticeType, Priority: input.Priority, Content: input.Content,
		TargetType: input.TargetType, TargetIds: jsonNullRaw(input.TargetIDs), IsPinned: boolSmallInt16(input.IsPinned),
		PublishAt: nullTimeFromPtr(input.PublishAt), ID: int64(noticeID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotEditable
	}
	return nil
}

// PublishNotice moves a draft to published and records who did it.
func (r *AuthRepository) PublishNotice(ctx context.Context, noticeID, publisherID uint64) error {
	affected, err := r.queries.PublishNotice(ctx, sqlcstore.PublishNoticeParams{
		PublishedBy: sql.NullInt64{Int64: int64(publisherID), Valid: true}, ID: int64(noticeID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCannotPublish
	}
	return nil
}

// RevokeNotice moves a published notice back to draft.
func (r *AuthRepository) RevokeNotice(ctx context.Context, noticeID uint64) error {
	affected, err := r.queries.RevokeNotice(ctx, int64(noticeID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCannotRevoke
	}
	return nil
}

// DeleteNotice soft deletes a notice. A notice that is still published is refused with ErrCannotRevoke.
func (r *AuthRepository) DeleteNotice(ctx context.Context, noticeID uint64) error {
	affected, err := r.queries.SoftDeleteNotice(ctx, int64(noticeID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrCannotRevoke
	}
	return nil
}

// ListMyNotices returns the announcements targeted at one user, optionally only the unread ones.
func (r *AuthRepository) ListMyNotices(ctx context.Context, userID uint64, unreadOnly bool, page, pageSize int) ([]Notice, int64, error) {
	page, pageSize = normalizePage(page, pageSize)
	total, err := r.queries.CountMyNotices(ctx, sqlcstore.CountMyNoticesParams{
		UserID: userID, UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, 0, err
	}
	rows, err := r.queries.ListMyNotices(ctx, sqlcstore.ListMyNoticesParams{
		Limit: int32(pageSize), Offset: int32((page - 1) * pageSize),
		UserID: userID, UnreadOnly: unreadOnly,
	})
	if err != nil {
		return nil, 0, err
	}
	items := make([]Notice, 0, len(rows))
	for _, row := range rows {
		items = append(items, Notice{
			ID: uint64(row.ID), Title: row.Title, NoticeType: row.NoticeType, Priority: row.Priority,
			TargetType: row.TargetType, TargetIDs: nullRawArray(row.TargetIds), Status: row.Status,
			IsPinned: smallIntBool(row.IsPinned), PublishAt: nullTimePtr(row.PublishAt),
			PublishedBy: nullInt64Ptr(row.PublishedBy), CreatedBy: row.CreatedBy, CreatorName: row.CreatorName,
			Read: row.IsRead, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		})
	}
	return items, total, nil
}

// MarkNoticeRead records that a user has read a notice. Marking twice is a no-op.
func (r *AuthRepository) MarkNoticeRead(ctx context.Context, noticeID, userID uint64) error {
	return r.queries.MarkNoticeRead(ctx, sqlcstore.MarkNoticeReadParams{NoticeID: noticeID, UserID: userID})
}

// MyNoticeDetail returns a notice only when it is targeted at the user; otherwise it behaves like a missing row.
func (r *AuthRepository) MyNoticeDetail(ctx context.Context, noticeID, userID uint64) (Notice, error) {
	allowed, err := r.queries.NoticeAllowedForUser(ctx, sqlcstore.NoticeAllowedForUserParams{
		NoticeID: int64(noticeID), UserID: userID,
	})
	if err != nil {
		return Notice{}, err
	}
	if !allowed {
		return Notice{}, sql.ErrNoRows
	}
	return r.NoticeDetail(ctx, noticeID, userID)
}
