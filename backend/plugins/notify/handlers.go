package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"dragz-admin/backend/internal/app"
	apierrors "dragz-admin/backend/internal/pkg/errors"
	"dragz-admin/backend/internal/plugin"
	"dragz-admin/backend/platform/notifier"
)

// Sensitive config keys are encrypted at rest and masked on read. A masked
// value sent back on update means "keep the stored secret".
var secretKeys = []string{"password", "access_key_secret", "secret_key"}

const masked = "********"

type channelInput struct {
	ChannelType string         `json:"channelType"`
	Name        string         `json:"name"`
	Enabled     int16          `json:"enabled"`
	Config      map[string]any `json:"config"`
}

type templateInput struct {
	ChannelType string `json:"channelType"`
	Code        string `json:"code"`
	Name        string `json:"name"`
	Subject     string `json:"subject"`
	Content     string `json:"content"`
	Status      int16  `json:"status"`
}

// sendInput is the wire type shared with platform/notifier. Aliasing it keeps
// the HTTP endpoint and the cross-plugin contract from drifting apart.
type sendInput = notifier.SendInput

// Routes is the plugin's declarative route table. Permission codes hang
// under the notify_page declared in Meta, so the startup button sync resolves
// them without a MenuCode override.
func Routes() []plugin.Route {
	const p = "notify"
	route := func(method, path, permission string, handler func(*app.Context) http.HandlerFunc) plugin.Route {
		return plugin.Route{Method: method, Path: path, Permission: permission, MenuCode: "notify_page", Handler: handler}
	}
	return []plugin.Route{
		route(http.MethodGet, "/api/notify/channels", p+":channel:query", listChannelsHandler),
		route(http.MethodGet, "/api/notify/channels/:id", p+":channel:query", getChannelHandler),
		route(http.MethodPost, "/api/notify/channels", p+":channel:create", createChannelHandler),
		route(http.MethodPut, "/api/notify/channels/:id", p+":channel:update", updateChannelHandler),
		route(http.MethodDelete, "/api/notify/channels/:id", p+":channel:delete", deleteChannelHandler),
		route(http.MethodPost, "/api/notify/channels/:id/test", p+":channel:update", testChannelHandler),

		route(http.MethodGet, "/api/notify/templates", p+":template:query", listTemplatesHandler),
		route(http.MethodGet, "/api/notify/templates/:id", p+":template:query", getTemplateHandler),
		route(http.MethodPost, "/api/notify/templates", p+":template:create", createTemplateHandler),
		route(http.MethodPut, "/api/notify/templates/:id", p+":template:update", updateTemplateHandler),
		route(http.MethodDelete, "/api/notify/templates/:id", p+":template:delete", deleteTemplateHandler),

		route(http.MethodGet, "/api/notify/logs", p+":log:query", listLogsHandler),
		route(http.MethodPost, "/api/notify/send", p+":send", sendHandler),
	}
}

func r(ctx *app.Context) *repo { return newRepo(ctx.DB) }

func requireDB(w http.ResponseWriter, ctx *app.Context, r0 *http.Request) bool {
	if ctx == nil || ctx.DB == nil {
		fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, r0)
		return false
	}
	return true
}

func notFoundOrFail(w http.ResponseWriter, ctx *app.Context, req *http.Request, err error) bool {
	if errors.Is(err, errNotFound) {
		fail(w, http.StatusNotFound, apierrors.CodeNotFound, "errors.not_found", ctx, req)
		return true
	}
	if err != nil {
		fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
		return true
	}
	return false
}

// ---------------------------------------------------------------------------
// channels
// ---------------------------------------------------------------------------

func listChannelsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		items, total, err := r(ctx).listChannels(req.Context(),
			req.URL.Query().Get("channelType"), req.URL.Query().Get("q"),
			queryInt(req.URL.Query().Get("page"), 1), queryInt(req.URL.Query().Get("pageSize"), 20))
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		for i := range items {
			if cfg, err := decodeConfig(items[i].ConfigJSON); err == nil {
				maskSecrets(cfg)
				if raw, err := json.Marshal(cfg); err == nil {
					items[i].ConfigJSON = string(raw)
				}
			}
		}
		ok(w, map[string]any{"items": items, "total": total})
	}
}

func getChannelHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		item, err := r(ctx).getChannel(req.Context(), id)
		if notFoundOrFail(w, ctx, req, err) {
			return
		}
		if cfg, err := decodeConfig(item.ConfigJSON); err == nil {
			maskSecrets(cfg)
			if raw, err := json.Marshal(cfg); err == nil {
				item.ConfigJSON = string(raw)
			}
		}
		ok(w, item)
	}
}

func createChannelHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		var in channelInput
		if !decodeJSON(w, req, &in) {
			return
		}
		ch, err := channelOf(in.ChannelType)
		if err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		if err := ch.Validate(in.Config); err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		raw, err := encryptConfig(ctx, in.Config)
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		id, err := r(ctx).createChannel(req.Context(), in.ChannelType, in.Name, raw, normalizeEnabled(in.Enabled))
		if err != nil {
			fail(w, http.StatusConflict, apierrors.CodeDuplicate, "errors.duplicate", ctx, req)
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

func updateChannelHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		var in channelInput
		if !decodeJSON(w, req, &in) {
			return
		}
		ch, err := channelOf(in.ChannelType)
		if err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		// Merge masked secrets from the stored row so an update that echoes
		// "********" does not overwrite the real encrypted value.
		current, err := r(ctx).getChannel(req.Context(), id)
		if notFoundOrFail(w, ctx, req, err) {
			return
		}
		merged, err := mergeSecrets(ctx, current.ConfigJSON, in.Config)
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		if err := ch.Validate(merged); err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		raw, err := encryptConfig(ctx, merged)
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		err = r(ctx).updateChannel(req.Context(), id, in.ChannelType, in.Name, raw, normalizeEnabled(in.Enabled))
		if notFoundOrFail(w, ctx, req, err) {
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

func deleteChannelHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		if notFoundOrFail(w, ctx, req, r(ctx).deleteChannel(req.Context(), id)) {
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

// testChannelHandler sends a probe message through one channel and records
// the result, mirroring internal/httpapi/webhook.go's test button.
func testChannelHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		row, err := r(ctx).getChannel(req.Context(), id)
		if notFoundOrFail(w, ctx, req, err) {
			return
		}
		cfg, err := decryptConfig(ctx, row.ConfigJSON)
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		ch, err := channelOf(row.ChannelType)
		if err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		msg := Message{Recipient: str(cfg, "test_recipient"), Subject: "通知通道测试", Content: "这是一条测试消息", Phone: str(cfg, "test_recipient"), Email: str(cfg, "test_recipient")}
		if msg.Recipient == "" {
			msg.Recipient = str(cfg, "test_to")
			msg.Phone = msg.Recipient
			msg.Email = msg.Recipient
		}
		if msg.Recipient == "" {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		sendErr := ch.Send(req.Context(), cfg, msg)
		status, errText := int16(1), ""
		if sendErr != nil {
			status, errText = 0, sendErr.Error()
		}
		_ = r(ctx).createLog(req.Context(), LogRow{
			ChannelType: row.ChannelType, ChannelName: row.Name, TemplateCode: "test",
			Recipient: msg.Recipient, Subject: msg.Subject, Content: msg.Content,
			Status: status, ErrorMsg: errText,
		})
		if sendErr != nil {
			writeJSON(w, http.StatusBadGateway, envelope{Code: apierrors.CodeInternal, Message: sendErr.Error()})
			return
		}
		ok(w, map[string]any{"sent": true})
	}
}

// ---------------------------------------------------------------------------
// templates
// ---------------------------------------------------------------------------

func listTemplatesHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		items, total, err := r(ctx).listTemplates(req.Context(),
			req.URL.Query().Get("channelType"), req.URL.Query().Get("q"),
			queryInt(req.URL.Query().Get("page"), 1), queryInt(req.URL.Query().Get("pageSize"), 20))
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		ok(w, map[string]any{"items": items, "total": total})
	}
}

func getTemplateHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		item, err := r(ctx).getTemplate(req.Context(), id)
		if notFoundOrFail(w, ctx, req, err) {
			return
		}
		ok(w, item)
	}
}

func createTemplateHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		var in templateInput
		if !decodeJSON(w, req, &in) {
			return
		}
		if _, err := channelOf(in.ChannelType); err != nil || strings.TrimSpace(in.Code) == "" {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		id, err := r(ctx).createTemplate(req.Context(), in.ChannelType, in.Code, in.Name, in.Subject, in.Content, normalizeEnabled(in.Status))
		if err != nil {
			fail(w, http.StatusConflict, apierrors.CodeDuplicate, "errors.duplicate", ctx, req)
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

func updateTemplateHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		var in templateInput
		if !decodeJSON(w, req, &in) {
			return
		}
		if _, err := channelOf(in.ChannelType); err != nil || strings.TrimSpace(in.Code) == "" {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		if notFoundOrFail(w, ctx, req, r(ctx).updateTemplate(req.Context(), id, in.ChannelType, in.Code, in.Name, in.Subject, in.Content, normalizeEnabled(in.Status))) {
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

func deleteTemplateHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		id, valid := pathID(req)
		if !valid {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		if notFoundOrFail(w, ctx, req, r(ctx).deleteTemplate(req.Context(), id)) {
			return
		}
		ok(w, map[string]any{"id": id})
	}
}

// ---------------------------------------------------------------------------
// logs + send
// ---------------------------------------------------------------------------

func listLogsHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		var status *int16
		if raw := req.URL.Query().Get("status"); raw == "0" || raw == "1" {
			v := int16(raw[0] - '0')
			status = &v
		}
		items, total, err := r(ctx).listLogs(req.Context(),
			req.URL.Query().Get("channelType"), status, req.URL.Query().Get("q"),
			queryInt(req.URL.Query().Get("page"), 1), queryInt(req.URL.Query().Get("pageSize"), 20))
		if err != nil {
			fail(w, http.StatusInternalServerError, apierrors.CodeInternal, "errors.internal", ctx, req)
			return
		}
		ok(w, map[string]any{"items": items, "total": total})
	}
}

// sendHandler is the HTTP entry point; the same logic is reachable through
// the "notify.send" event handled by Service.HandleEvent (see notifier.go).
func sendHandler(ctx *app.Context) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		if !requireDB(w, ctx, req) {
			return
		}
		var in sendInput
		if !decodeJSON(w, req, &in) {
			return
		}
		sent, err := dispatch(req.Context(), ctx, in)
		if err != nil {
			fail(w, http.StatusBadRequest, apierrors.CodeValidation, "errors.validation", ctx, req)
			return
		}
		ok(w, sent)
	}
}

// dispatch resolves the channel + template, renders, sends, and logs.
func dispatch(ctx context.Context, appCtx *app.Context, in sendInput) (notifier.SendResult, error) {
	if err := in.Validate(); err != nil {
		return notifier.SendResult{}, err
	}
	if appCtx == nil || appCtx.DB == nil {
		return notifier.SendResult{}, fmt.Errorf("notify: no database")
	}
	repo := r(appCtx)

	var row ChannelRow
	var err error
	if in.ChannelID > 0 {
		row, err = repo.getChannel(ctx, in.ChannelID)
	} else {
		// Pick the first enabled channel of the requested type.
		items, _, listErr := repo.listChannels(ctx, in.ChannelType, "", 1, 1)
		if listErr != nil {
			return notifier.SendResult{}, listErr
		}
		if len(items) == 0 {
			return notifier.SendResult{}, errNotFound
		}
		row = items[0]
	}
	if err != nil {
		return notifier.SendResult{}, err
	}
	if row.Enabled != 1 {
		return notifier.SendResult{}, fmt.Errorf("notify: channel disabled")
	}
	cfg, err := decryptConfig(appCtx, row.ConfigJSON)
	if err != nil {
		return notifier.SendResult{}, err
	}
	ch, err := channelOf(row.ChannelType)
	if err != nil {
		return notifier.SendResult{}, err
	}

	subject, content := in.Subject, in.Content
	if in.TemplateCode != "" {
		tpl, tplErr := repo.findTemplate(ctx, row.ChannelType, in.TemplateCode)
		if tplErr != nil {
			return notifier.SendResult{}, tplErr
		}
		subject = render(tpl.Subject, in.Vars)
		content = render(tpl.Content, in.Vars)
	}
	msg := Message{Recipient: in.Recipient, Subject: subject, Content: content, Phone: in.Recipient, Email: in.Recipient}
	sendErr := ch.Send(ctx, cfg, msg)
	status, errText := int16(1), ""
	if sendErr != nil {
		status, errText = 0, sendErr.Error()
	}
	_ = repo.createLog(ctx, LogRow{
		ChannelType: row.ChannelType, ChannelName: row.Name, TemplateCode: in.TemplateCode,
		Recipient: in.Recipient, Subject: subject, Content: content, Status: status, ErrorMsg: errText,
	})
	if sendErr != nil {
		return notifier.SendResult{}, sendErr
	}
	return notifier.SendResult{Sent: true, ChannelType: row.ChannelType}, nil
}

// ---------------------------------------------------------------------------
// config encryption helpers
// ---------------------------------------------------------------------------

func encryptConfig(ctx *app.Context, cfg map[string]any) (string, error) {
	if ctx == nil || ctx.Secrets == nil {
		return "", fmt.Errorf("notify: secret box unavailable")
	}
	out := make(map[string]any, len(cfg))
	for k, v := range cfg {
		if isSecret(k) && v != nil {
			s, _ := v.(string)
			if s != "" && s != masked {
				enc, err := ctx.Secrets.Encrypt(s)
				if err != nil {
					return "", err
				}
				out[k] = enc
				continue
			}
		}
		out[k] = v
	}
	raw, err := json.Marshal(out)
	return string(raw), err
}

func decryptConfig(ctx *app.Context, raw string) (map[string]any, error) {
	cfg, err := decodeConfig(raw)
	if err != nil {
		return nil, err
	}
	if ctx == nil || ctx.Secrets == nil {
		return cfg, nil
	}
	for _, k := range secretKeys {
		v, ok := cfg[k].(string)
		if !ok || v == "" || strings.HasPrefix(v, "enc:v1:") == false {
			continue
		}
		plain, err := ctx.Secrets.Decrypt(v)
		if err != nil {
			return nil, fmt.Errorf("notify: decrypt %s: %w", k, err)
		}
		cfg[k] = plain
	}
	return cfg, nil
}

func mergeSecrets(ctx *app.Context, storedRaw string, incoming map[string]any) (map[string]any, error) {
	stored, err := decodeConfig(storedRaw)
	if err != nil {
		return nil, err
	}
	out := make(map[string]any, len(incoming))
	for k, v := range incoming {
		out[k] = v
	}
	for _, k := range secretKeys {
		if v, ok := out[k].(string); ok && v == masked {
			if old, ok := stored[k]; ok {
				out[k] = old
			} else {
				delete(out, k)
			}
		}
	}
	return out, nil
}

func maskSecrets(cfg map[string]any) {
	for _, k := range secretKeys {
		if v, ok := cfg[k].(string); ok && v != "" {
			cfg[k] = masked
		}
	}
}

func isSecret(key string) bool {
	for _, k := range secretKeys {
		if k == key {
			return true
		}
	}
	return false
}

func normalizeEnabled(v int16) int16 {
	if v == 0 {
		return 1
	}
	return v
}

// render replaces {{key}} placeholders with the corresponding variable.
func render(tpl string, vars map[string]any) string {
	out := tpl
	for k, v := range vars {
		out = strings.ReplaceAll(out, "{{"+k+"}}", fmt.Sprint(v))
	}
	return out
}
