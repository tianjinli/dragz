package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	sqlcstore "dragz-admin/backend/internal/store/sqlc"
)

// Webhook is one sys_webhook row. Secret and Headers hold ciphertext, and the list query masks them.
type Webhook struct {
	ID                  uint64          `json:"id"`
	Name                string          `json:"name"`
	URL                 string          `json:"url"`
	Secret              string          `json:"secret"`
	Events              json.RawMessage `json:"events"`
	Headers             string          `json:"headers"`
	AllowPrivateNetwork bool            `json:"allowPrivateNetwork"`
	TimeoutSeconds      int             `json:"timeoutSeconds"`
	MaxRetries          int             `json:"maxRetries"`
	Status              int8            `json:"status"`
	LastSuccessAt       *time.Time      `json:"lastSuccessAt"`
	LastFailureAt       *time.Time      `json:"lastFailureAt"`
	CreatedAt           time.Time       `json:"createdAt"`
	UpdatedAt           time.Time       `json:"updatedAt"`
}

// SaveWebhookInput is the payload shared by create and update. The secret and headers arrive already encrypted.
type SaveWebhookInput struct {
	Name                string
	URL                 string
	SecretEncrypted     string
	Events              []string
	HeadersEncrypted    string
	AllowPrivateNetwork bool
	TimeoutSeconds      int
	MaxRetries          int
	Status              int8
}

func mapWebhook(id int64, name, url, secret string, events []byte, headers string, allowPrivate int16, timeout, retries int32, status int16, lastSuccess, lastFailure sql.NullTime, createdAt, updatedAt time.Time) Webhook {
	item := Webhook{
		ID: uint64(id), Name: name, URL: url, Secret: secret, Events: json.RawMessage(events),
		Headers: headers, AllowPrivateNetwork: smallIntBool(allowPrivate),
		TimeoutSeconds: int(timeout), MaxRetries: int(retries), Status: int8(status),
		LastSuccessAt: nullTimePtr(lastSuccess), LastFailureAt: nullTimePtr(lastFailure),
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	return item
}

// ListWebhooks returns every webhook with its secret and headers masked, so the list can be shown without decrypting anything.
func (r *AuthRepository) ListWebhooks(ctx context.Context) ([]Webhook, error) {
	rows, err := r.queries.ListWebhooks(ctx)
	if err != nil {
		return nil, err
	}
	items := make([]Webhook, 0, len(rows))
	for _, row := range rows {
		item := mapWebhook(row.ID, row.Name, row.Url, row.SecretEncrypted, row.Events, row.HeadersEncrypted,
			row.AllowPrivateNetwork, row.TimeoutSeconds, row.MaxRetries, row.Status,
			row.LastSuccessAt, row.LastFailureAt, row.CreatedAt, row.UpdatedAt)
		item.Secret = "********"
		if item.Headers != "" {
			item.Headers = "********"
		}
		items = append(items, item)
	}
	return items, nil
}

// WebhookByID loads one webhook including its encrypted secret, which only the delivery path needs.
func (r *AuthRepository) WebhookByID(ctx context.Context, webhookID uint64) (Webhook, error) {
	row, err := r.queries.GetWebhookByID(ctx, int64(webhookID))
	if err != nil {
		return Webhook{}, err
	}
	return mapWebhook(row.ID, row.Name, row.Url, row.SecretEncrypted, row.Events, row.HeadersEncrypted,
		row.AllowPrivateNetwork, row.TimeoutSeconds, row.MaxRetries, row.Status,
		row.LastSuccessAt, row.LastFailureAt, row.CreatedAt, row.UpdatedAt), nil
}

// CreateWebhook inserts a webhook.
func (r *AuthRepository) CreateWebhook(ctx context.Context, input SaveWebhookInput) (uint64, error) {
	events, _ := json.Marshal(input.Events)
	id, err := r.queries.CreateWebhook(ctx, sqlcstore.CreateWebhookParams{
		Name: input.Name, Url: input.URL, SecretEncrypted: input.SecretEncrypted, Events: events,
		HeadersEncrypted:    optionalString(input.HeadersEncrypted),
		AllowPrivateNetwork: boolSmallInt16(input.AllowPrivateNetwork),
		TimeoutSeconds:      int32(input.TimeoutSeconds), MaxRetries: int32(input.MaxRetries), Status: int16(input.Status),
	})
	return uint64(id), err
}

// UpdateWebhook rewrites a webhook. keepSecret and keepHeaders let the caller leave the stored ciphertext alone when the form submitted the mask instead of a new value.
func (r *AuthRepository) UpdateWebhook(ctx context.Context, webhookID uint64, input SaveWebhookInput, keepSecret, keepHeaders bool) error {
	events, _ := json.Marshal(input.Events)
	var affected int64
	var err error
	base := sqlcstore.UpdateWebhookParams{
		Name: input.Name, Url: input.URL, Events: events,
		AllowPrivateNetwork: boolSmallInt16(input.AllowPrivateNetwork),
		TimeoutSeconds:      int32(input.TimeoutSeconds), MaxRetries: int32(input.MaxRetries),
		Status: int16(input.Status), ID: int64(webhookID),
	}
	switch {
	case !keepSecret && !keepHeaders:
		affected, err = r.queries.UpdateWebhookWithSecretAndHeaders(ctx, sqlcstore.UpdateWebhookWithSecretAndHeadersParams{
			Name: base.Name, Url: base.Url, Events: base.Events, AllowPrivateNetwork: base.AllowPrivateNetwork,
			TimeoutSeconds: base.TimeoutSeconds, MaxRetries: base.MaxRetries, Status: base.Status,
			SecretEncrypted: input.SecretEncrypted, HeadersEncrypted: optionalString(input.HeadersEncrypted), ID: base.ID,
		})
	case !keepSecret:
		affected, err = r.queries.UpdateWebhookWithSecret(ctx, sqlcstore.UpdateWebhookWithSecretParams{
			Name: base.Name, Url: base.Url, Events: base.Events, AllowPrivateNetwork: base.AllowPrivateNetwork,
			TimeoutSeconds: base.TimeoutSeconds, MaxRetries: base.MaxRetries, Status: base.Status,
			SecretEncrypted: input.SecretEncrypted, ID: base.ID,
		})
	case !keepHeaders:
		affected, err = r.queries.UpdateWebhookWithHeaders(ctx, sqlcstore.UpdateWebhookWithHeadersParams{
			Name: base.Name, Url: base.Url, Events: base.Events, AllowPrivateNetwork: base.AllowPrivateNetwork,
			TimeoutSeconds: base.TimeoutSeconds, MaxRetries: base.MaxRetries, Status: base.Status,
			HeadersEncrypted: optionalString(input.HeadersEncrypted), ID: base.ID,
		})
	default:
		affected, err = r.queries.UpdateWebhook(ctx, base)
	}
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// UpdateWebhookStatus enables or disables a webhook.
func (r *AuthRepository) UpdateWebhookStatus(ctx context.Context, webhookID uint64, status int8) error {
	affected, err := r.queries.UpdateWebhookStatus(ctx, sqlcstore.UpdateWebhookStatusParams{
		Status: int16(status), ID: int64(webhookID),
	})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// DeleteWebhook soft deletes a webhook.
func (r *AuthRepository) DeleteWebhook(ctx context.Context, webhookID uint64) error {
	affected, err := r.queries.SoftDeleteWebhook(ctx, int64(webhookID))
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrNotFound
	}
	return nil
}

// MarkWebhookDelivery records the outcome of a delivery attempt. Failures are swallowed: a webhook status update must never break the business request that triggered it.
func (r *AuthRepository) MarkWebhookDelivery(ctx context.Context, webhookID uint64, success bool) {
	if success {
		_ = r.queries.MarkWebhookSuccess(ctx, int64(webhookID))
		return
	}
	_ = r.queries.MarkWebhookFailure(ctx, int64(webhookID))
}
