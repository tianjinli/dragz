package app

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"

	"dragz-admin/backend/internal/security"
	"dragz-admin/backend/internal/store"
	"dragz-admin/backend/platform/mailer"
)

// ConfigMailer is the system-level mail sender. It reads the deployment's
// mail.* sys_config keys on every send, so an operator edits the SMTP account
// in 系统设置 and the next message uses it — no restart, no cache to
// invalidate. Unlike the notify plugin it exists independently of the plugin
// runtime: core features such as tenant invitations depend on it.
type ConfigMailer struct {
	auth    *store.AuthRepository
	secrets *security.SecretBox
}

// NewConfigMailer builds the sender; both dependencies are always present in
// NewContext, so the result is never nil.
func NewConfigMailer(auth *store.AuthRepository, secrets *security.SecretBox) *ConfigMailer {
	return &ConfigMailer{auth: auth, secrets: secrets}
}

// mailConfigKeys maps every mail.* sys_config key. The password is stored
// SecretBox-encrypted (is_secret) and decrypted here, in memory only.
const (
	mailKeyEnabled    = "mail.smtp.enabled"
	mailKeyHost       = "mail.smtp.host"
	mailKeyPort       = "mail.smtp.port"
	mailKeyEncryption = "mail.smtp.encryption"
	mailKeyUsername   = "mail.smtp.username"
	mailKeyPassword   = "mail.smtp.password"
	mailKeyFrom       = "mail.smtp.from"
	mailKeyFromName   = "mail.smtp.fromName"
)

// MailInviteBaseURL reads the public origin invite links are built against.
// Empty means the operator has not configured one; callers skip the send.
const mailInviteBaseURLKey = "mail.invite.baseUrl"

// MailInviteBaseURL resolves the invite-link origin from sys_config.
func (c *Context) MailInviteBaseURL(ctx context.Context) string {
	value, err := c.Auth.ConfigValue(ctx, mailInviteBaseURLKey)
	if err != nil {
		return ""
	}
	return strings.TrimRight(strings.TrimSpace(value), "/")
}

// Send implements the platform mailer sender contract. ErrUnconfigured means
// "mail disabled or required keys missing" — a skip, not a delivery failure.
func (m *ConfigMailer) Send(ctx context.Context, msg mailer.Message) error {
	cfg, err := m.loadConfig(ctx)
	if err != nil {
		return err
	}
	return mailer.Send(ctx, cfg, msg)
}

// loadConfig assembles the engine config from sys_config. Every missing
// required key degrades to ErrUnconfigured so callers see one skip signal
// instead of distinguishing "off" from "half configured".
func (m *ConfigMailer) loadConfig(ctx context.Context) (mailer.Config, error) {
	var cfg mailer.Config
	enabled, err := m.auth.ConfigValue(ctx, mailKeyEnabled)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return cfg, mailer.ErrUnconfigured
		}
		return cfg, fmt.Errorf("mailer: read enabled flag: %w", err)
	}
	if enabled != "true" && enabled != "1" {
		return cfg, mailer.ErrUnconfigured
	}

	read := func(key string) string {
		value, err := m.auth.ConfigValue(ctx, key)
		if err != nil {
			return ""
		}
		return value
	}
	cfg.Host = read(mailKeyHost)
	cfg.From = read(mailKeyFrom)
	cfg.FromName = read(mailKeyFromName)
	cfg.Username = read(mailKeyUsername)
	cfg.Encryption = read(mailKeyEncryption)
	if cfg.Host == "" || cfg.From == "" {
		return cfg, mailer.ErrUnconfigured
	}

	if port := read(mailKeyPort); port != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(port))
		if err != nil {
			return cfg, fmt.Errorf("%w: mail.smtp.port is not numeric: %s", mailer.ErrInvalidConfig, port)
		}
		cfg.Port = parsed
	} else {
		cfg.Port = 465
	}

	// A masked value can only appear through a config write that kept the
	// stored secret; ConfigValue on the raw column never returns the mask.
	password := read(mailKeyPassword)
	if password != "" {
		plain, err := m.secrets.Decrypt(password)
		if err != nil {
			return cfg, fmt.Errorf("mailer: decrypt password: %w", err)
		}
		cfg.Password = plain
	}
	return cfg, nil
}

// SendInviteMail delivers one invite link and records the outcome on the
// invite row. It runs on its own context: the HTTP response must not wait on
// an SMTP server, and a process restart mid-send leaves the invite "pending",
// which the UI renders as still sending. A delivery failure never invalidates
// the invitation — the admin copies the link manually instead.
func (appCtx *Context) SendInviteMail(inviteID uint64, subject, body string) {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	err := appCtx.Mailer.Send(ctx, mailer.Message{
		Subject:  subject,
		TextBody: body,
	})
	switch {
	case err == nil:
		if err := appCtx.Tenants.MarkInviteDelivery(ctx, inviteID, store.InviteMailSent, ""); err != nil {
			logx.Errorf("invite %d delivered but status write failed: %v", inviteID, err)
		}
	case errors.Is(err, mailer.ErrUnconfigured):
		if err := appCtx.Tenants.MarkInviteDelivery(ctx, inviteID, store.InviteMailSkipped, ""); err != nil {
			logx.Errorf("invite %d mail skipped but status write failed: %v", inviteID, err)
		}
	default:
		if err := appCtx.Tenants.MarkInviteDelivery(ctx, inviteID, store.InviteMailFailed, err.Error()); err != nil {
			logx.Errorf("invite %d mail failed and status write also failed: %v", inviteID, err)
		}
		logx.Errorf("invite %d mail delivery failed: %v", inviteID, err)
	}
}
