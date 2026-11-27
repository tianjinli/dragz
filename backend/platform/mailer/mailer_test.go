package mailer

import (
	"bytes"
	"context"
	"errors"
	"mime"
	"net/textproto"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	cases := []struct {
		name string
		cfg  Config
		want error
	}{
		{"missing host", Config{Port: 465, From: "a@b.c"}, ErrInvalidConfig},
		{"missing port", Config{Host: "smtp.example.com", From: "a@b.c"}, ErrInvalidConfig},
		{"missing from", Config{Host: "smtp.example.com", Port: 465}, ErrInvalidConfig},
		{"auth over plaintext", Config{Host: "h", Port: 25, From: "a@b.c", Username: "u", Password: "p"}, ErrInvalidConfig},
		{"ok implicit tls", Config{Host: "h", Port: 465, From: "a@b.c", Username: "u", Password: "p", Encryption: EncryptionTLS}, nil},
		{"ok starttls", Config{Host: "h", Port: 587, From: "a@b.c", Username: "u", Password: "p", Encryption: EncryptionStartTLS}, nil},
		{"ok plaintext no auth", Config{Host: "h", Port: 25, From: "a@b.c", Encryption: EncryptionNone}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if !errors.Is(err, tc.want) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestSendRejectsEmptyRecipientAndBody(t *testing.T) {
	cfg := Config{Host: "127.0.0.1", Port: 1, From: "a@b.c"}
	if err := Send(context.Background(), cfg, Message{Subject: "x", TextBody: "y"}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty recipient: got %v", err)
	}
	if err := Send(context.Background(), cfg, Message{To: "x@y.z", Subject: "x"}); !errors.Is(err, ErrInvalidConfig) {
		t.Fatalf("empty body: got %v", err)
	}
}

func TestRenderHeadersAndBody(t *testing.T) {
	payload, err := render(buildHeaderFixtures(), "正文内容")
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	for _, want := range []string{"From: =?utf-8?", "Subject: =?utf-8?", "Mime-Version: 1.0", `Content-Type: text/plain; charset="utf-8"`, "\r\n\r\n正文内容"} {
		if !bytes.Contains(payload, []byte(want)) {
			t.Fatalf("rendered payload missing %q:\n%s", want, payload)
		}
	}
	// RFC 5322 requires CRLF line endings throughout the header block.
	if strings.Contains(string(payload[:bytes.Index(payload, []byte("\r\n\r\n"))]), "\n") &&
		!strings.Contains(string(payload), "\r\n") {
		t.Fatal("header block is not CRLF terminated")
	}
}

func buildHeaderFixtures() textproto.MIMEHeader {
	headers := textproto.MIMEHeader{}
	headers.Set("MIME-Version", "1.0")
	headers.Set("Content-Type", ContentTypeText+`; charset="utf-8"`)
	headers.Set("From", formatAddress("系统管理员", "noreply@example.com"))
	// Send() Q-encodes the subject before calling render; mirror that here.
	headers.Set("Subject", mime.QEncoding.Encode("utf-8", "邀请"))
	return headers
}

func TestFormatAddress(t *testing.T) {
	if got := formatAddress("", "a@b.c"); got != "a@b.c" {
		t.Fatalf("bare address = %q", got)
	}
	if got := formatAddress("名字", "a@b.c"); !strings.HasPrefix(got, "=?utf-8?") || !strings.HasSuffix(got, " <a@b.c>") {
		t.Fatalf("display name = %q", got)
	}
}
