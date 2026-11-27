package notify

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"dragz-admin/backend/platform/mailer"
)

// Message is the channel-agnostic input to a delivery. Templates already
// rendered; a channel adapter only has to put it on the wire.
type Message struct {
	Recipient string
	Subject   string
	Content   string
	// Phone is E.164-ish digits for SMS channels; Email for mail channels.
	Phone string
	Email string
}

// Channel is the delivery contract. Config is the decrypted JSON object stored
// on notify_channel.config_json; each adapter validates the keys it needs.
type Channel interface {
	Type() string
	Validate(config map[string]any) error
	Send(ctx context.Context, config map[string]any, msg Message) error
}

// Registry holds the adapters by channel_type. Unknown types fail closed.
var registry = map[string]Channel{
	"smtp":        smtpChannel{},
	"aliyun_sms":  aliyunSMSChannel{},
	"tencent_sms": tencentSMSChannel{},
}

func channelOf(channelType string) (Channel, error) {
	ch, ok := registry[channelType]
	if !ok {
		return nil, fmt.Errorf("unsupported channel type %q", channelType)
	}
	return ch, nil
}

// decodeConfig turns the stored JSON into a map. Empty config is valid JSON
// `{}` so a freshly created channel does not panic on read.
func decodeConfig(raw string) (map[string]any, error) {
	if strings.TrimSpace(raw) == "" {
		return map[string]any{}, nil
	}
	var cfg map[string]any
	if err := json.Unmarshal([]byte(raw), &cfg); err != nil {
		return nil, fmt.Errorf("invalid channel config: %w", err)
	}
	return cfg, nil
}

func str(cfg map[string]any, key string) string {
	v, _ := cfg[key].(string)
	return strings.TrimSpace(v)
}

func require(cfg map[string]any, keys ...string) error {
	for _, key := range keys {
		if str(cfg, key) == "" {
			return fmt.Errorf("missing required config: %s", key)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// SMTP (email)
// ---------------------------------------------------------------------------

type smtpChannel struct{}

func (smtpChannel) Type() string { return "smtp" }

func (smtpChannel) Validate(cfg map[string]any) error {
	return require(cfg, "host", "port", "from", "username", "password")
}

// Send delegates to the platform SMTP engine: one implementation, two config
// sources (this channel's stored config and the system mail.* settings).
func (c smtpChannel) Send(ctx context.Context, cfg map[string]any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	port, err := strconv.Atoi(str(cfg, "port"))
	if err != nil {
		return fmt.Errorf("smtp port must be numeric: %w", err)
	}
	to := msg.Recipient
	if to == "" {
		to = msg.Email
	}
	if to == "" {
		return fmt.Errorf("smtp recipient is empty")
	}
	body := msg.Content
	if strings.TrimSpace(body) == "" {
		return fmt.Errorf("smtp body is empty")
	}
	engineCfg := mailer.Config{
		Host:        str(cfg, "host"),
		Port:        port,
		Username:    str(cfg, "username"),
		Password:    str(cfg, "password"),
		From:        str(cfg, "from"),
		Encryption:  str(cfg, "encryption"),
		ContentType: str(cfg, "content_type"),
	}
	// Channels created before the engine existed never stored an encryption
	// mode; a username implies an authenticated relay, so starttls is the
	// only sane default that keeps credentials off the wire.
	if engineCfg.Encryption == "" && engineCfg.Username != "" {
		engineCfg.Encryption = mailer.EncryptionStartTLS
	}
	msg2 := mailer.Message{To: to, Subject: subjectOr(msg.Subject, "通知")}
	if strings.Contains(strings.ToLower(engineCfg.ContentType), "html") {
		msg2.HTMLBody = body
	} else {
		msg2.TextBody = body
	}
	return mailer.Send(ctx, engineCfg, msg2)
}

func subjectOr(subject, fallback string) string {
	if strings.TrimSpace(subject) == "" {
		return fallback
	}
	return subject
}

// ---------------------------------------------------------------------------
// Aliyun SMS (RPC signature, standard library)
// ---------------------------------------------------------------------------

type aliyunSMSChannel struct{}

func (aliyunSMSChannel) Type() string { return "aliyun_sms" }

func (aliyunSMSChannel) Validate(cfg map[string]any) error {
	return require(cfg, "access_key_id", "access_key_secret", "sign_name", "template_code")
}

func (c aliyunSMSChannel) Send(ctx context.Context, cfg map[string]any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	phone := msg.Recipient
	if phone == "" {
		phone = msg.Phone
	}
	if phone == "" {
		return fmt.Errorf("sms recipient is empty")
	}
	params := map[string]string{
		"Action":           "SendSms",
		"Version":          "2017-05-25",
		"Format":           "JSON",
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
		"AccessKeyId":      str(cfg, "access_key_id"),
		"PhoneNumbers":     phone,
		"SignName":         str(cfg, "sign_name"),
		"TemplateCode":     str(cfg, "template_code"),
	}
	// TemplateParam is JSON supplied per request (rendered variables).
	param := msg.Content
	if param == "" {
		param = "{}"
	}
	params["TemplateParam"] = param
	secret := str(cfg, "access_key_secret")
	params["Signature"] = aliyunSign(params, secret)

	form := url.Values{}
	for k, v := range params {
		form.Set(k, v)
	}
	endpoint := str(cfg, "endpoint")
	if endpoint == "" {
		endpoint = "https://dysmsapi.aliyuncs.com/"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return postJSON(req, "aliyun sms")
}

func aliyunSign(params map[string]string, secret string) string {
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var canonical strings.Builder
	for i, k := range keys {
		if i > 0 {
			canonical.WriteByte('&')
		}
		canonical.WriteString(aliyunPercentEncode(k))
		canonical.WriteByte('=')
		canonical.WriteString(aliyunPercentEncode(params[k]))
	}
	stringToSign := "POST" + "&" + aliyunPercentEncode("/") + "&" + aliyunPercentEncode(canonical.String())
	h := hmac.New(sha1.New, []byte(secret+"&"))
	_, _ = h.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// aliyunPercentEncode is RFC 3986: unreserved chars pass through, everything
// else becomes %XY uppercase. url.QueryEscape is wrong here (space -> "+",
// "~" escaped) and would produce a signature Aliyun rejects.
func aliyunPercentEncode(s string) string {
	const upper = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_' || c == '.' || c == '~' {
			b.WriteByte(c)
		} else {
			b.WriteByte('%')
			b.WriteByte(upper[c>>4])
			b.WriteByte(upper[c&0x0f])
		}
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// Tencent Cloud SMS (TC3-HMAC-SHA256 signature, standard library)
// ---------------------------------------------------------------------------

type tencentSMSChannel struct{}

func (tencentSMSChannel) Type() string { return "tencent_sms" }

func (tencentSMSChannel) Validate(cfg map[string]any) error {
	return require(cfg, "secret_id", "secret_key", "sdk_app_id", "sign_name", "template_id")
}

func (c tencentSMSChannel) Send(ctx context.Context, cfg map[string]any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	phone := msg.Recipient
	if phone == "" {
		phone = msg.Phone
	}
	if phone == "" {
		return fmt.Errorf("sms recipient is empty")
	}
	param := msg.Content
	if param == "" {
		param = "[]"
	}
	body := map[string]any{
		"PhoneNumberSet": []string{phone},
		"SmsSdkAppId":    str(cfg, "sdk_app_id"),
		"SignName":       str(cfg, "sign_name"),
		"TemplateID":     str(cfg, "template_id"),
		"TemplateParamSet": []string{param},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}
	endpoint := str(cfg, "endpoint")
	if endpoint == "" {
		endpoint = "https://sms.tencentcloudapi.com/"
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	timestamp := strconv.FormatInt(now.Unix(), 10)
	authorization := tc3Sign(cfg, payload, now)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("Host", "sms.tencentcloudapi.com")
	req.Header.Set("X-TC-Action", "SendSms")
	req.Header.Set("X-TC-Version", "2019-03-25")
	req.Header.Set("X-TC-Timestamp", timestamp)
	req.Header.Set("X-TC-Region", str(cfg, "region"))
	req.Header.Set("Authorization", authorization)
	return postJSON(req, "tencent sms")
}

// tc3Sign implements the TC3-HMAC-SHA256 signing process.
func tc3Sign(cfg map[string]any, payload []byte, now time.Time) string {
	secretID := str(cfg, "secret_id")
	secretKey := str(cfg, "secret_key")
	date := now.UTC().Format("2006-01-02")
	timestamp := strconv.FormatInt(now.Unix(), 10)
	service := "sms"

	hashedPayload := sha256Hex(payload)
	canonicalRequest := strings.Join([]string{
		"POST", "/", "",
		"content-type:application/json; charset=utf-8\nhost:sms.tencentcloudapi.com\n",
		"content-type;host", hashedPayload,
	}, "\n")
	credentialScope := date + "/" + service + "/tc3_request"
	stringToSign := strings.Join([]string{
		"TC3-HMAC-SHA256", timestamp, credentialScope, sha256Hex([]byte(canonicalRequest)),
	}, "\n")

	hDate := hmacSHA256([]byte("TC3"+secretKey), []byte(date))
	hService := hmacSHA256(hDate, []byte(service))
	hSign := hmacSHA256(hService, []byte("tc3_request"))
	signature := hex.EncodeToString(hmacSHA256(hSign, []byte(stringToSign)))

	return "TC3-HMAC-SHA256 Credential=" + secretID + "/" + credentialScope +
		", SignedHeaders=content-type;host, Signature=" + signature
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	_, _ = h.Write(data)
	return h.Sum(nil)
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// postJSON executes a provider request and surfaces provider-level errors.
func postJSON(req *http.Request, label string) error {
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return fmt.Errorf("%s read response: %w", label, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s: HTTP %d: %s", label, resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		Code    *string `json:"Code"`
		Message *string `json:"Message"`
		Response *struct {
			Error *struct {
				Code    string `json:"Code"`
				Message string `json:"Message"`
			} `json:"Error"`
		} `json:"Response"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if parsed.Response != nil && parsed.Response.Error != nil && parsed.Response.Error.Code != "" {
			return fmt.Errorf("%s: %s: %s", label, parsed.Response.Error.Code, parsed.Response.Error.Message)
		}
		if parsed.Code != nil && *parsed.Code != "OK" {
			return fmt.Errorf("%s: %s: %s", label, *parsed.Code, stringValue(parsed.Message))
		}
	}
	return nil
}

func stringValue(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
