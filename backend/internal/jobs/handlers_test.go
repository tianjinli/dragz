package jobs

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/eventbus"
)

func TestHandlerAllowlist(t *testing.T) {
	if !HandlerAllowed("maintenance.cleanup_audit_logs") {
		t.Fatal("expected built-in handler to be allowed")
	}
	if !HandlerAllowed("outbox.publish") {
		t.Fatal("expected outbox relay handler to be allowed")
	}
	if !HandlerAllowed("billing.reconcile_subscriptions") {
		t.Fatal("expected billing reconcile handler to be allowed")
	}
	if !HandlerAllowed("tenant.purge_due") {
		t.Fatal("expected tenant purge handler to be allowed")
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

func TestPublishOutboxSetsTenantAndMarksSent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}
	bus := eventbus.New()
	var published string
	bus.Subscribe("notice.published", func(_ context.Context, ev eventbus.Event) error {
		published = ev.Name
		body, _ := ev.Payload.(map[string]any)
		if body["tenantId"] != uint64(7) || body["id"] != uint64(8) {
			t.Fatalf("unexpected event payload: %#v", ev.Payload)
		}
		return nil
	})

	expectPendingOutbox(mock)
	expectEnabledWebhooks(mock, nil)
	expectMarkOutboxSent(mock)

	output, err := RunHandler(context.Background(), HandlerEnv{
		DB: wrapped, Repo: store.NewAuthRepository(wrapped),
		Tenants: store.NewTenantRepository(wrapped), Bus: bus,
	}, store.ScheduledJob{Handler: "outbox.publish"})
	if err != nil {
		t.Fatalf("publish outbox: %v", err)
	}
	if output != "published 1 outbox event(s)" || published != "notice.published" {
		t.Fatalf("output %q published %q", output, published)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPublishOutboxNilBusMarksSentWhenNoWebhooks(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}

	expectPendingOutbox(mock)
	expectEnabledWebhooks(mock, nil)
	expectMarkOutboxSent(mock)

	output, err := RunHandler(context.Background(), HandlerEnv{
		DB: wrapped, Repo: store.NewAuthRepository(wrapped),
		Tenants: store.NewTenantRepository(wrapped),
	}, store.ScheduledJob{Handler: "outbox.publish"})
	if err != nil {
		t.Fatalf("publish outbox: %v", err)
	}
	if output != "published 1 outbox event(s)" {
		t.Fatalf("output %q", output)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPublishOutboxDeliversMatchingWebhook(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}

	box, err := security.NewSecretBox(base64.StdEncoding.EncodeToString(make([]byte, 32)))
	if err != nil {
		t.Fatalf("secret box: %v", err)
	}
	secret, err := box.Encrypt("topsecret")
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	gotEvent := ""
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotEvent = r.Header.Get("X-Webhook-Event")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	now := time.Now()
	hooks := sqlmock.NewRows([]string{
		"id", "name", "url", "secret_encrypted", "events", "headers_encrypted",
		"allow_private_network", "timeout_seconds", "max_retries",
		"status", "last_success_at", "last_failure_at", "created_at", "updated_at",
	}).AddRow(
		int64(3), "hook", server.URL, secret, []byte(`["*"]`), "",
		int16(1), int32(5), int32(0),
		int16(1), nil, nil, now, now,
	)

	expectPendingOutbox(mock)
	expectEnabledWebhooks(mock, hooks)
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("7").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("false").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`last_success_at`).WithArgs(int64(3)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	expectMarkOutboxSent(mock)

	output, err := RunHandler(context.Background(), HandlerEnv{
		DB: wrapped, Repo: store.NewAuthRepository(wrapped),
		Tenants: store.NewTenantRepository(wrapped), Secrets: box,
	}, store.ScheduledJob{Handler: "outbox.publish"})
	if err != nil {
		t.Fatalf("publish outbox: %v", err)
	}
	if output != "published 1 outbox event(s)" || gotEvent != "notice.published" {
		t.Fatalf("output %q event %q", output, gotEvent)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestPublishOutboxDefersWhenSecretsMissing(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	defer db.Close()
	wrapped := &store.DB{DB: db}
	now := time.Now()
	hooks := sqlmock.NewRows([]string{
		"id", "name", "url", "secret_encrypted", "events", "headers_encrypted",
		"allow_private_network", "timeout_seconds", "max_retries",
		"status", "last_success_at", "last_failure_at", "created_at", "updated_at",
	}).AddRow(
		int64(3), "hook", "https://example.com/hook", "enc:v1:secret", []byte(`["notice.published"]`), "",
		int16(0), int32(5), int32(0),
		int16(1), nil, nil, now, now,
	)

	expectPendingOutbox(mock)
	expectEnabledWebhooks(mock, hooks)
	expectMarkOutboxFailed(mock, int64(8), "webhook 3: webhook secrets are not configured")

	output, err := RunHandler(context.Background(), HandlerEnv{
		DB: wrapped, Repo: store.NewAuthRepository(wrapped),
		Tenants: store.NewTenantRepository(wrapped),
	}, store.ScheduledJob{Handler: "outbox.publish"})
	if err != nil {
		t.Fatalf("publish outbox: %v", err)
	}
	if output != "published 0 outbox event(s), 1 failed" {
		t.Fatalf("output %q", output)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func expectPendingOutbox(mock sqlmock.Sqlmock) {
	// The relay first returns rows stranded by a dead worker, then claims a
	// batch. Both run on the platform identity, so both carry the bypass.
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("true").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`ReclaimStaleOutbox`).WithArgs(int32(900)).WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectCommit()

	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("0").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("true").WillReturnResult(sqlmock.NewResult(0, 0))
	// The relay now claims rather than lists: worker id plus batch size.
	mock.ExpectQuery(`FROM sys_outbox`).WithArgs(sqlmock.AnyArg(), int32(100)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "tenant_id", "event_type", "payload", "created_at"}).
			AddRow(int64(8), uint64(7), "notice.published", []byte(`{"noticeId":1}`), time.Now()))
	mock.ExpectCommit()
}

func expectEnabledWebhooks(mock sqlmock.Sqlmock, rows *sqlmock.Rows) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("7").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("false").WillReturnResult(sqlmock.NewResult(0, 0))
	if rows == nil {
		rows = sqlmock.NewRows([]string{
			"id", "name", "url", "secret_encrypted", "events", "headers_encrypted",
			"allow_private_network", "timeout_seconds", "max_retries",
			"status", "last_success_at", "last_failure_at", "created_at", "updated_at",
		})
	}
	mock.ExpectQuery(`FROM sys_webhook`).WillReturnRows(rows)
	mock.ExpectCommit()
}

func expectMarkOutboxSent(mock sqlmock.Sqlmock) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("7").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("false").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`UPDATE sys_outbox`).WithArgs(int64(8)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}

func expectMarkOutboxFailed(mock sqlmock.Sqlmock, id int64, errMsg string) {
	mock.ExpectBegin()
	mock.ExpectExec(`SELECT set_config\('app\.tenant_id'`).WithArgs("7").WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectExec(`SELECT set_config\('app\.rls_bypass'`).WithArgs("false").WillReturnResult(sqlmock.NewResult(0, 0))
	// sqlc orders these by statement position: last_error is $1, id is $2.
	mock.ExpectExec(`UPDATE sys_outbox`).WithArgs(errMsg, id).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
}
