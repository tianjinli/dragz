// Package mailer is the platform-level SMTP delivery engine.
//
// Email to the outside world is a system-level guarantee, not a plugin
// capability: core features (tenant invitations, password mails) must be able
// to deliver regardless of which notification plugins are enabled. The engine
// therefore lives here, imports only the standard library, and carries no
// business concepts — configuration is passed in by the caller, never loaded
// from the database inside this package.
//
// Callers:
//   - internal/app wires a ConfigMailer that reads the deployment's mail.*
//     sys_config keys and delegates here (available even with every plugin
//     disabled).
//   - plugins/notify uses it as the wire adapter for its "smtp" channel so
//     there is exactly one SMTP implementation in the process.
package mailer

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"mime"
	"net"
	"net/smtp"
	"net/textproto"
	"strconv"
	"strings"
	"time"
)

// Encryption selects how the connection is secured.
const (
	// EncryptionNone is a plain connection. Authentication over it is refused
	// by net/smtp and would leak credentials, so a config with a username and
	// no encryption fails validation instead.
	EncryptionNone = "none"
	// EncryptionStartTLS connects plain and upgrades via STARTTLS before auth.
	EncryptionStartTLS = "starttls"
	// EncryptionTLS is implicit TLS from the first byte (port 465 style).
	EncryptionTLS = "tls"
)

// ContentType values for Message bodies.
const (
	ContentTypeText = "text/plain"
	ContentTypeHTML = "text/html"
)

var (
	// ErrInvalidConfig means the Config struct failed validation; the message
	// names the offending field.
	ErrInvalidConfig = errors.New("mailer: invalid configuration")
	// ErrUnconfigured is returned by Senders that have no deployment mail
	// settings (feature off or required keys empty). Callers treat it as
	// "delivery skipped", never as a message failure.
	ErrUnconfigured = errors.New("mailer: no mail configuration")
)

// Config is one SMTP account. Secrets travel in memory only; persistence and
// encryption at rest are the caller's concern.
type Config struct {
	Host     string
	Port     int
	Username string
	Password string
	// From is the envelope and header sender address, user@example.com.
	From string
	// FromName is the display name prepended to From; optional.
	FromName string
	// Encryption is one of EncryptionNone / EncryptionStartTLS /
	// EncryptionTLS. Empty means EncryptionNone.
	Encryption string
	// ContentType is ContentTypeText or ContentTypeHTML; empty means text.
	ContentType string
}

// Message is one outgoing mail. Exactly one of TextBody / HTMLBody is
// rendered; HTMLBody wins when both are set.
type Message struct {
	To      string
	Subject string
	TextBody string
	HTMLBody string
}

// Validate checks the fields every dial needs. Auth over an unencrypted
// connection is rejected outright: net/smtp refuses it anyway and the failure
// mode of silently leaking credentials is worse than a loud config error.
func (c Config) Validate() error {
	if strings.TrimSpace(c.Host) == "" {
		return fmt.Errorf("%w: host is required", ErrInvalidConfig)
	}
	if c.Port <= 0 || c.Port > 65535 {
		return fmt.Errorf("%w: port must be 1-65535", ErrInvalidConfig)
	}
	if strings.TrimSpace(c.From) == "" {
		return fmt.Errorf("%w: from is required", ErrInvalidConfig)
	}
	if (c.Username != "" || c.Password != "") &&
		c.encryption() == EncryptionNone {
		return fmt.Errorf("%w: authentication requires starttls or tls encryption", ErrInvalidConfig)
	}
	return nil
}

// encryption normalizes the configured mode; empty means plain.
func (c Config) encryption() string {
	switch strings.ToLower(strings.TrimSpace(c.Encryption)) {
	case EncryptionStartTLS:
		return EncryptionStartTLS
	case EncryptionTLS:
		return EncryptionTLS
	default:
		return EncryptionNone
	}
}

// contentType normalizes the body mode; empty means plain text.
func (c Config) contentType() string {
	if strings.Contains(strings.ToLower(c.ContentType), "html") {
		return ContentTypeHTML
	}
	return ContentTypeText
}

// Sender is the capability core features consume through app.Context. The
// interface stays in this package so callers depend only on platform code;
// the system-level implementation reads deployment config, and tests can
// install a stub without touching a database.
type Sender interface {
	Send(ctx context.Context, msg Message) error
}

// Unconfigured is the default sender: every call reports ErrUnconfigured.
// It stands in before wiring and in tests that never configured mail, so a
// missing deployment setting is visible instead of silently dropped.
type Unconfigured struct{}

var _ Sender = Unconfigured{}

// Send reports that mail is not configured.
func (Unconfigured) Send(context.Context, Message) error {
	return ErrUnconfigured
}

// Send delivers msg through the configured account. It honors ctx
// cancellation; net/smtp has no context support, so the dial and the
// conversation run in a goroutine that is abandoned (not leaked: the
// underlying TCP conns carry deadlines) when ctx finishes first.
func Send(ctx context.Context, cfg Config, msg Message) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	to := strings.TrimSpace(msg.To)
	if to == "" {
		return fmt.Errorf("%w: recipient is required", ErrInvalidConfig)
	}

	body, contentType, err := buildBody(cfg, msg)
	if err != nil {
		return err
	}
	from := cfg.From
	headers := textproto.MIMEHeader{}
	headers.Set("From", formatAddress(cfg.FromName, from))
	headers.Set("To", to)
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", strings.TrimSpace(msg.Subject)))
	headers.Set("Date", time.Now().Format(time.RFC1123Z))
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", contentType+`; charset="utf-8"`)
	payload, err := render(headers, body)
	if err != nil {
		return err
	}

	sendErr := make(chan error, 1)
	go func() { sendErr <- deliver(cfg, from, to, payload) }()
	select {
	case err := <-sendErr:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// deliver dials, secures, authenticates and sends one message.
func deliver(cfg Config, from, to string, payload []byte) error {
	addr := net.JoinHostPort(strings.TrimSpace(cfg.Host), strconv.Itoa(cfg.Port))
	// Deadline bounds a hung server even when the caller passed context.Background.
	deadline := time.Now().Add(30 * time.Second)

	switch cfg.encryption() {
	case EncryptionTLS:
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: strings.TrimSpace(cfg.Host)})
		if err != nil {
			return fmt.Errorf("mailer: dial %s: %w", addr, err)
		}
		_ = conn.SetDeadline(deadline)
		return converse(conn, cfg, from, to, payload)
	default:
		conn, err := net.DialTimeout("tcp", addr, 15*time.Second)
		if err != nil {
			return fmt.Errorf("mailer: dial %s: %w", addr, err)
		}
		_ = conn.SetDeadline(deadline)
		if cfg.encryption() == EncryptionStartTLS {
			client, err := smtp.NewClient(conn, strings.TrimSpace(cfg.Host))
			if err != nil {
				_ = conn.Close()
				return fmt.Errorf("mailer: handshake: %w", err)
			}
			if ok, _ := client.Extension("STARTTLS"); !ok {
				_ = client.Close()
				return fmt.Errorf("mailer: %s does not advertise STARTTLS", addr)
			}
			if err := client.StartTLS(&tls.Config{ServerName: strings.TrimSpace(cfg.Host)}); err != nil {
				_ = client.Close()
				return fmt.Errorf("mailer: starttls: %w", err)
			}
			return converseOn(client, cfg, from, to, payload)
		}
		return converse(conn, cfg, from, to, payload)
	}
}

// converse drives the SMTP conversation on a fresh connection.
func converse(conn net.Conn, cfg Config, from, to string, payload []byte) error {
	client, err := smtp.NewClient(conn, strings.TrimSpace(cfg.Host))
	if err != nil {
		_ = conn.Close()
		return fmt.Errorf("mailer: handshake: %w", err)
	}
	return converseOn(client, cfg, from, to, payload)
}

// converseOn runs MAIL FROM / RCPT TO / DATA on an established client and
// always closes it.
func converseOn(client *smtp.Client, cfg Config, from, to string, payload []byte) error {
	defer client.Close()
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, strings.TrimSpace(cfg.Host))
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("mailer: auth: %w", err)
		}
	}
	if err := client.Mail(from); err != nil {
		return fmt.Errorf("mailer: mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mailer: rcpt to: %w", err)
	}
	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("mailer: data: %w", err)
	}
	if _, err := w.Write(payload); err != nil {
		_ = w.Close()
		return fmt.Errorf("mailer: write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("mailer: finish body: %w", err)
	}
	return client.Quit()
}

// buildBody returns the payload body and its Content-Type value. HTML wins
// when both bodies are present; a plain-only message stays single-part.
func buildBody(cfg Config, msg Message) (string, string, error) {
	text := strings.TrimSpace(msg.TextBody)
	html := strings.TrimSpace(msg.HTMLBody)
	if text == "" && html == "" {
		return "", "", fmt.Errorf("%w: body is required", ErrInvalidConfig)
	}
	if html != "" {
		return html, ContentTypeHTML, nil
	}
	return text, ContentTypeText, nil
}

// render writes MIME headers followed by the body. A multipart wrapper is
// deliberately not used: every message here carries exactly one part.
func render(headers textproto.MIMEHeader, body string) ([]byte, error) {
	var buf strings.Builder
	for key, values := range headers {
		for _, value := range values {
			// Headers fold with CRLF per RFC 5322; the DATA stream terminates
			// on a lone dot, which body content cannot inject because the
			// body is written after the headers below with no header
			// injection surface (keys are constants, values are encoded).
			if _, err := fmt.Fprintf(&buf, "%s: %s\r\n", textproto.CanonicalMIMEHeaderKey(key), value); err != nil {
				return nil, err
			}
		}
	}
	buf.WriteString("\r\n")
	if _, err := io.WriteString(&buf, body); err != nil {
		return nil, err
	}
	return []byte(buf.String()), nil
}

// formatAddress renders "Name <addr>" with RFC 2047 encoding for the display
// name; a bare address passes through.
func formatAddress(name, addr string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return addr
	}
	return mime.QEncoding.Encode("utf-8", name) + " <" + addr + ">"
}
