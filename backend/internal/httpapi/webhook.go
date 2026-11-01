package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/store"
	webhookdelivery "dragz-admin/backend/internal/webhook"
)

type saveWebhookRequest struct {
	Name                string             `json:"name"`
	URL                 string             `json:"url"`
	Secret              string             `json:"secret"`
	Events              []string           `json:"events"`
	Headers             *map[string]string `json:"headers"`
	AllowPrivateNetwork bool               `json:"allowPrivateNetwork"`
	TimeoutSeconds      int                `json:"timeoutSeconds"`
	MaxRetries          int                `json:"maxRetries"`
	Status              int8               `json:"status"`
}

func listWebhooksHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		items, err := ctx.Auth.ListWebhooks(r.Context())
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_list_failed")
			return
		}
		ok(w, r, items)
	}
}

func createWebhookHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req saveWebhookRequest
		if !decode(w, r, &req) || !validateWebhook(w, r, req, true) {
			return
		}
		secret, headers, err := encryptWebhookSecrets(ctx, req.Secret, req.Headers)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_encrypt_failed")
			return
		}
		id, err := ctx.Auth.CreateWebhook(r.Context(), webhookInput(req, secret, headers))
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_save_failed")
			return
		}
		ok(w, r, map[string]uint64{"id": id})
	}
}

func updateWebhookHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_id_invalid")
			return
		}
		var req saveWebhookRequest
		if !decode(w, r, &req) || !validateWebhook(w, r, req, false) {
			return
		}
		keepSecret := req.Secret == "" || req.Secret == "********"
		keepHeaders := req.Headers == nil
		secret, headers, err := encryptWebhookSecrets(ctx, optionalSecret(req.Secret, keepSecret), req.Headers)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_encrypt_failed")
			return
		}
		if err := ctx.Auth.UpdateWebhook(r.Context(), id, webhookInput(req, secret, headers), keepSecret, keepHeaders); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.webhook_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func deleteWebhookHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_id_invalid")
			return
		}
		if err := ctx.Auth.DeleteWebhook(r.Context(), id); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.webhook_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func updateWebhookStatusHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_id_invalid")
			return
		}
		var req updateStatusRequest
		if !decode(w, r, &req) || !validStatus(req.Status) {
			fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_status_invalid")
			return
		}
		if err := ctx.Auth.UpdateWebhookStatus(r.Context(), id, req.Status); err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.webhook_not_found")
			return
		}
		ok(w, r, map[string]bool{"success": true})
	}
}

func testWebhookHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, valid := pathID(r)
		if !valid {
			fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_id_invalid")
			return
		}
		item, err := ctx.Auth.WebhookByID(r.Context(), id)
		if err != nil {
			fail(w, r, http.StatusNotFound, 40401, "errors.webhook_not_found")
			return
		}
		secret, err := ctx.Secrets.Decrypt(item.Secret)
		if err != nil {
			fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_decrypt_failed")
			return
		}
		headers := map[string]string{}
		if item.Headers != "" {
			value, decryptErr := ctx.Secrets.Decrypt(item.Headers)
			if decryptErr != nil || json.Unmarshal([]byte(value), &headers) != nil {
				fail(w, r, http.StatusInternalServerError, 50000, "errors.webhook_headers_decrypt_failed")
				return
			}
		}
		result, err := webhookdelivery.Deliver(r.Context(), webhookdelivery.Target{
			URL: item.URL, Secret: secret, Headers: headers,
			AllowPrivateNetwork: item.AllowPrivateNetwork,
			Timeout:             time.Duration(item.TimeoutSeconds) * time.Second, MaxRetries: item.MaxRetries,
		}, "system.webhook.test", map[string]any{"message": "Dragz Admin webhook test"})
		ctx.Auth.MarkWebhookDelivery(r.Context(), id, err == nil)
		if err != nil {
			fail(w, r, http.StatusBadGateway, 50201, "errors.webhook_deliver_failed")
			return
		}
		ok(w, r, result)
	}
}

func validateWebhook(w http.ResponseWriter, r *http.Request, req saveWebhookRequest, create bool) bool {
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 128 || webhookdelivery.ValidateTarget(req.URL) != nil {
		fail(w, r, http.StatusBadRequest, 40001, "errors.webhook_fields_invalid")
		return false
	}
	if create && strings.TrimSpace(req.Secret) == "" {
		fail(w, r, http.StatusBadRequest, 40002, "errors.webhook_secret_required")
		return false
	}
	if len(req.Events) == 0 {
		fail(w, r, http.StatusBadRequest, 40003, "errors.webhook_events_required")
		return false
	}
	if req.TimeoutSeconds < 1 || req.TimeoutSeconds > 60 || req.MaxRetries < 0 || req.MaxRetries > 10 {
		fail(w, r, http.StatusBadRequest, 40004, "errors.webhook_options_invalid")
		return false
	}
	return req.Status == 0 || req.Status == 1
}

func encryptWebhookSecrets(ctx *app.Context, secret string, headers *map[string]string) (string, string, error) {
	encryptedSecret, err := ctx.Secrets.Encrypt(secret)
	if err != nil {
		return "", "", err
	}
	if headers == nil {
		return encryptedSecret, "", nil
	}
	value, err := json.Marshal(headers)
	if err != nil {
		return "", "", err
	}
	encryptedHeaders, err := ctx.Secrets.Encrypt(string(value))
	return encryptedSecret, encryptedHeaders, err
}

func webhookInput(req saveWebhookRequest, secret, headers string) store.SaveWebhookInput {
	return store.SaveWebhookInput{
		Name: strings.TrimSpace(req.Name), URL: strings.TrimSpace(req.URL),
		SecretEncrypted: secret, Events: req.Events, HeadersEncrypted: headers,
		AllowPrivateNetwork: req.AllowPrivateNetwork, TimeoutSeconds: req.TimeoutSeconds,
		MaxRetries: req.MaxRetries, Status: req.Status,
	}
}
