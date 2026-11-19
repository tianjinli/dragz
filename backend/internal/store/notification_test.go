package store

import (
	"context"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestCreateUserNotificationInserts(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	createdAt := time.Date(2026, 11, 19, 8, 0, 0, 0, time.UTC)
	mock.ExpectQuery(`INSERT INTO sys_notification`).
		WithArgs(uint64(7), "in_app", "hi", "body", "biz", "1").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "user_id", "channel", "title", "content", "biz_type", "biz_id", "read_at", "created_at",
		}).AddRow(int64(9), uint64(7), "in_app", "hi", "body", "biz", "1", nil, createdAt))

	repo := NewAuthRepository(&DB{DB: db})
	item, created, err := repo.CreateUserNotification(context.Background(), SaveNotificationInput{
		UserID: 7, Channel: "in_app", Title: "hi", Content: "body", BizType: "biz", BizID: "1",
	})
	if err != nil {
		t.Fatalf("CreateUserNotification: %v", err)
	}
	if !created || item.ID != 9 || item.Read || !item.CreatedAt.Equal(createdAt) {
		t.Fatalf("item = %+v created = %v", item, created)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestUnreadMessageCountsSumsInboxAndNotices(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()

	mock.ExpectQuery(`FROM sys_notice n`).
		WithArgs(uint64(7), true).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(2)))
	mock.ExpectQuery(`FROM sys_notification`).
		WithArgs(uint64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(int64(3)))

	repo := NewAuthRepository(&DB{DB: db})
	counts, err := repo.UnreadMessageCounts(context.Background(), 7)
	if err != nil {
		t.Fatalf("UnreadMessageCounts: %v", err)
	}
	if counts.Notices != 2 || counts.Notifications != 3 || counts.Total != 5 {
		t.Fatalf("counts = %+v", counts)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}
