package jobs

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/store"
)

func TestHandlerAllowlist(t *testing.T) {
	if !HandlerAllowed("maintenance.cleanup_audit_logs") {
		t.Fatal("expected built-in handler to be allowed")
	}
	if HandlerAllowed("shell.exec") {
		t.Fatal("arbitrary handler must not be allowed")
	}
}

func TestRunHandlerRejectsUnknownHandler(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}

	_, err = RunHandler(context.Background(), HandlerEnv{DB: wrapped}, store.ScheduledJob{
		Handler: "unknown.handler",
		Payload: json.RawMessage(`{"retentionDays":30}`),
	})
	if err == nil {
		t.Fatal("expected error for unknown handler")
	}
}

func TestRunHandlerRejectsLargeRetention(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}

	_, err = RunHandler(context.Background(), HandlerEnv{DB: wrapped}, store.ScheduledJob{
		Handler: "maintenance.cleanup_refresh_tokens",
		Payload: json.RawMessage(`{"retentionDays":5000}`),
	})
	if err == nil {
		t.Fatal("expected error for excessive retentionDays")
	}
}
