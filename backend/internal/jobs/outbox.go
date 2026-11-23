package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"dragz-admin/backend/internal/pkg/ctxkey"
	"dragz-admin/backend/internal/store"
	webhookdelivery "dragz-admin/backend/internal/webhook"
	"dragz-admin/backend/platform/eventbus"
)

// outboxDeferred means the row was recorded as failed (retry or dead-letter)
// and the relay should keep going through the rest of the batch.
type outboxDeferred struct{ err error }

func (e *outboxDeferred) Error() string {
	if e == nil || e.err == nil {
		return "outbox delivery failed"
	}
	return e.err.Error()
}

func (e *outboxDeferred) Unwrap() error { return e.err }

// publishOutbox delivers pending sys_outbox rows. The list runs with row-level
// security bypassed so every tenant is visible. Each row is published on the
// process bus when one is wired, then delivered to that tenant's enabled
// webhooks. Deliver's own retries are the webhook attempt budget: a returned
// error fails the outbox row (backoff, then dead-letter) and the relay
// continues. MarkOutboxSent only updates status 0, so a second worker that
// loses the race treats the row as already sent.
func publishOutbox(ctx context.Context, env HandlerEnv) (string, error) {
	if env.DB == nil || env.Tenants == nil {
		return "", errors.New("outbox relay is not configured")
	}
	// A worker that died mid-delivery left its rows in "processing". Return
	// them before claiming, otherwise they stay invisible for good.
	if _, err := env.Tenants.ReclaimStaleOutbox(ctx, outboxLeaseSeconds); err != nil {
		return "", fmt.Errorf("reclaim stale outbox claims: %w", err)
	}
	events, err := listPendingOutbox(ctx, env)
	if err != nil {
		return "", err
	}
	sent, failed := 0, 0
	for _, event := range events {
		err := deliverOutbox(ctx, env, event)
		if err != nil {
			var deferred *outboxDeferred
			if errors.As(err, &deferred) {
				failed++
				continue
			}
			if failed > 0 {
				return "", fmt.Errorf("published %d outbox event(s), %d failed, then id %d: %w", sent, failed, event.ID, err)
			}
			return "", fmt.Errorf("published %d outbox event(s), then id %d: %w", sent, event.ID, err)
		}
		sent++
	}
	summary := fmt.Sprintf("published %d outbox event(s)", sent)
	if failed > 0 {
		summary = fmt.Sprintf("%s, %d failed", summary, failed)
	}
	return summary, nil
}

func listPendingOutbox(ctx context.Context, env HandlerEnv) ([]store.OutboxEvent, error) {
	// Claims, not lists. The claim moves each row to "processing" with
	// FOR UPDATE SKIP LOCKED, so a second relay running at the same time skips
	// these events instead of delivering them again.
	return env.Tenants.ClaimPendingOutbox(ctx, outboxWorkerID(), 100)
}

// outboxLeaseSeconds is how long a claim stays valid. It has to cover a full
// webhook delivery including its own retries, or a slow but healthy delivery
// would be reclaimed and repeated.
const outboxLeaseSeconds = 900

// outboxWorkerID identifies this relay process in sys_outbox.claimed_by, so a
// stuck claim can be traced back to the worker that took it.
func outboxWorkerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "worker"
	}
	return host + "/" + strconv.Itoa(os.Getpid())
}

func deliverOutbox(ctx context.Context, env HandlerEnv, event store.OutboxEvent) error {
	// The jobs-runner process has its own bus. Publish when one is wired so
	// in-process subscribers (tests, a future shared worker) can react. An
	// unwired bus is not an error: webhooks are the durable delivery path.
	if env.Bus != nil {
		if err := env.Bus.Publish(tenantCtx(ctx, event.TenantID), eventbus.Event{
			Name:   event.EventType,
			Source: "outbox",
			Payload: map[string]any{
				"id":       event.ID,
				"tenantId": event.TenantID,
				"payload":  json.RawMessage(event.Payload),
			},
		}); err != nil {
			return err
		}
	}
	if env.Repo == nil {
		return errors.New("webhook delivery is not configured")
	}
	hooks, err := tenantWebhooks(ctx, env, event.TenantID)
	if err != nil {
		return deferOutbox(ctx, env, event, fmt.Errorf("list webhooks: %w", err))
	}
	data, err := outboxData(event.Payload)
	if err != nil {
		return deferOutbox(ctx, env, event, err)
	}
	for _, hook := range hooks {
		if !webhookSubscribed(hook.Events, event.EventType) {
			continue
		}
		if err := deliverWebhook(ctx, env, event.TenantID, hook, event.EventType, data); err != nil {
			return deferOutbox(ctx, env, event, fmt.Errorf("webhook %d: %w", hook.ID, err))
		}
	}
	return markOutboxSent(tenantCtx(ctx, event.TenantID), env, event.ID)
}

// tenantCtx scopes a derived context to one tenant with row-level security
// enforced, so repository calls see only that tenant's rows.
func tenantCtx(ctx context.Context, tenantID uint64) context.Context {
	return ctxkey.WithRLSBypass(ctxkey.WithTenantID(ctx, tenantID), false)
}

// tenantTx runs fn inside a short transaction bound to one tenant. Outbox
// delivery is cross-tenant, so each event gets its own session rather than
// inheriting whatever tenant the relay job itself runs as.
func tenantTx(ctx context.Context, env HandlerEnv, tenantID uint64, fn func(context.Context) error) error {
	scoped := tenantCtx(ctx, tenantID)
	tx, err := env.DB.BeginTx(scoped, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	xctx := ctxkey.WithTx(scoped, tx.Tx)
	if err := fn(xctx); err != nil {
		return err
	}
	return tx.Commit()
}

func tenantWebhooks(ctx context.Context, env HandlerEnv, tenantID uint64) ([]store.Webhook, error) {
	var hooks []store.Webhook
	err := tenantTx(ctx, env, tenantID, func(txCtx context.Context) error {
		var err error
		hooks, err = env.Repo.ListEnabledWebhooks(txCtx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return hooks, nil
}

func deliverWebhook(ctx context.Context, env HandlerEnv, tenantID uint64, hook store.Webhook, eventType string, data any) error {
	secret, headers, err := webhookSecrets(env, hook)
	if err != nil {
		return err
	}
	// The HTTP call happens outside any transaction; only the delivery record
	// write holds one.
	_, err = webhookdelivery.Deliver(ctx, webhookdelivery.Target{
		URL: hook.URL, Secret: secret, Headers: headers,
		AllowPrivateNetwork: hook.AllowPrivateNetwork,
		Timeout:             time.Duration(hook.TimeoutSeconds) * time.Second,
		MaxRetries:          hook.MaxRetries,
	}, eventType, data)
	recordErr := tenantTx(ctx, env, tenantID, func(txCtx context.Context) error {
		env.Repo.MarkWebhookDelivery(txCtx, hook.ID, err == nil)
		return nil
	})
	if recordErr != nil && err == nil {
		return recordErr
	}
	return err
}

func webhookSecrets(env HandlerEnv, hook store.Webhook) (string, map[string]string, error) {
	if (hook.Secret != "" || hook.Headers != "") && env.Secrets == nil {
		return "", nil, errors.New("webhook secrets are not configured")
	}
	secret := ""
	if hook.Secret != "" {
		var err error
		secret, err = env.Secrets.Decrypt(hook.Secret)
		if err != nil {
			return "", nil, fmt.Errorf("secret: %w", err)
		}
	}
	headers := map[string]string{}
	if hook.Headers == "" {
		return secret, headers, nil
	}
	value, err := env.Secrets.Decrypt(hook.Headers)
	if err != nil {
		return "", nil, fmt.Errorf("headers: %w", err)
	}
	if err := json.Unmarshal([]byte(value), &headers); err != nil {
		return "", nil, fmt.Errorf("headers: %w", err)
	}
	return secret, headers, nil
}

func webhookSubscribed(raw json.RawMessage, eventType string) bool {
	var names []string
	if err := json.Unmarshal(raw, &names); err != nil {
		return false
	}
	for _, name := range names {
		name = strings.TrimSpace(name)
		if name == "*" || name == eventType {
			return true
		}
	}
	return false
}

func outboxData(payload []byte) (any, error) {
	if len(payload) == 0 {
		return map[string]any{}, nil
	}
	var data any
	if err := json.Unmarshal(payload, &data); err != nil {
		return nil, fmt.Errorf("outbox payload: %w", err)
	}
	return data, nil
}

func deferOutbox(ctx context.Context, env HandlerEnv, event store.OutboxEvent, cause error) error {
	if cause == nil {
		cause = errors.New("outbox delivery failed")
	}
	if err := markOutboxFailed(tenantCtx(ctx, event.TenantID), env, event.ID, cause); err != nil {
		return fmt.Errorf("%w; mark outbox failed: %v", cause, err)
	}
	return &outboxDeferred{err: cause}
}

func markOutboxSent(ctx context.Context, env HandlerEnv, id uint64) error {
	tx, err := env.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	err = env.Tenants.MarkOutboxSent(ctxkey.WithTx(ctx, tx.Tx), id)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	return tx.Commit()
}

func markOutboxFailed(ctx context.Context, env HandlerEnv, id uint64, cause error) error {
	tx, err := env.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	if err := env.Tenants.MarkOutboxFailed(ctxkey.WithTx(ctx, tx.Tx), id, cause.Error()); err != nil {
		return err
	}
	return tx.Commit()
}
