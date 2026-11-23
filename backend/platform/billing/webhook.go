package billing

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Event is one provider webhook, normalized.
type Event struct {
	// ID is the provider's event id. It is the idempotency key: a retried
	// delivery carries the same id and must not be applied twice.
	ID string
	// Type is the provider's event name, for example invoice.paid.
	Type string
	// TenantID is the tenant the event is about. A provider that does not know
	// our tenant ids can send a subscription or invoice id instead; the handler
	// resolves it.
	TenantID uint64
	// InvoiceID is set for invoice events.
	InvoiceID uint64
	// SubscriptionID is set for subscription events.
	SubscriptionID uint64
	// Data is the raw provider payload, stored for forensics.
	Data json.RawMessage
}

// ErrSignatureInvalid means the request did not carry a valid signature.
var ErrSignatureInvalid = errors.New("payment webhook signature is invalid")

// Verifier authenticates and parses one provider's webhook.
type Verifier interface {
	// Name is the path segment the provider posts to and the value stored with
	// each event.
	Name() string
	Verify(header http.Header, body []byte) (Event, error)
}

// envelope is the JSON body every verifier understands. Providers with a
// different shape are adapted by their Verifier implementation.
type envelope struct {
	ID             string          `json:"id"`
	Type           string          `json:"type"`
	TenantID       uint64          `json:"tenantId"`
	InvoiceID      uint64          `json:"invoiceId"`
	SubscriptionID uint64          `json:"subscriptionId"`
	Data           json.RawMessage `json:"data"`
}

// HMACVerifier checks an HMAC-SHA256 signature over "<timestamp>.<body>".
//
// The signature header is "t=<unix>,v1=<hex>". Including the timestamp in the
// signed payload is what makes a captured request expire: without it an
// attacker could replay a valid signature forever. A provider that signs only
// the body is supported by omitting the timestamp part of the header; the
// event id still makes the replay harmless at the storage layer.
type HMACVerifier struct {
	Provider        string
	Secret          string
	SignatureHeader string
	EventIDHeader   string
	Tolerance       time.Duration
}

// Name implements Verifier.
func (v HMACVerifier) Name() string { return v.Provider }

// Verify implements Verifier.
func (v HMACVerifier) Verify(header http.Header, body []byte) (Event, error) {
	if strings.TrimSpace(v.Secret) == "" {
		return Event{}, fmt.Errorf("%w: no webhook secret configured", ErrSignatureInvalid)
	}
	raw := strings.TrimSpace(header.Get(v.SignatureHeader))
	if raw == "" {
		return Event{}, fmt.Errorf("%w: missing %s", ErrSignatureInvalid, v.SignatureHeader)
	}
	timestamp, signature, err := parseSignature(raw)
	if err != nil {
		return Event{}, err
	}
	if timestamp != 0 && v.Tolerance > 0 {
		age := time.Since(time.Unix(timestamp, 0))
		if age > v.Tolerance || age < -v.Tolerance {
			return Event{}, fmt.Errorf("%w: timestamp outside tolerance", ErrSignatureInvalid)
		}
	}
	mac := hmac.New(sha256.New, []byte(v.Secret))
	if timestamp != 0 {
		mac.Write([]byte(strconv.FormatInt(timestamp, 10)))
		mac.Write([]byte("."))
	}
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	if !hmac.Equal([]byte(expected), []byte(signature)) {
		return Event{}, fmt.Errorf("%w: digest mismatch", ErrSignatureInvalid)
	}
	return decodeEvent(header, body, v.EventIDHeader)
}

// UnsignedVerifier accepts an unsigned body. It exists for local development
// and for a provider that authenticates by a shared header instead; it is
// rejected by the handler when the deployment is not in development mode.
type UnsignedVerifier struct {
	Provider      string
	EventIDHeader string
}

// Name implements Verifier.
func (v UnsignedVerifier) Name() string { return v.Provider }

// Verify implements Verifier.
func (v UnsignedVerifier) Verify(header http.Header, body []byte) (Event, error) {
	return decodeEvent(header, body, v.EventIDHeader)
}

// NewVerifier builds the verifier named by configuration. An unknown name is
// an error at startup, not a silent fallback to unsigned delivery.
func NewVerifier(provider, secret, signatureHeader, eventIDHeader string, tolerance time.Duration) (Verifier, error) {
	name := strings.ToLower(strings.TrimSpace(provider))
	switch name {
	case "", "none", "unsigned":
		return UnsignedVerifier{Provider: "none", EventIDHeader: eventIDHeader}, nil
	case "hmac":
		if signatureHeader == "" {
			signatureHeader = "X-Signature"
		}
		return HMACVerifier{
			Provider: "hmac", Secret: secret, SignatureHeader: signatureHeader,
			EventIDHeader: eventIDHeader, Tolerance: tolerance,
		}, nil
	default:
		return nil, fmt.Errorf("unknown payment provider %q", provider)
	}
}

func decodeEvent(header http.Header, body []byte, eventIDHeader string) (Event, error) {
	var bodyEnv envelope
	if err := json.Unmarshal(body, &bodyEnv); err != nil {
		return Event{}, fmt.Errorf("decode payment event: %w", err)
	}
	id := bodyEnv.ID
	if eventIDHeader != "" {
		if fromHeader := strings.TrimSpace(header.Get(eventIDHeader)); fromHeader != "" {
			id = fromHeader
		}
	}
	if id == "" || bodyEnv.Type == "" {
		return Event{}, fmt.Errorf("payment event is missing id or type")
	}
	data := bodyEnv.Data
	if len(data) == 0 {
		data = body
	}
	return Event{
		ID: id, Type: bodyEnv.Type, TenantID: bodyEnv.TenantID,
		InvoiceID: bodyEnv.InvoiceID, SubscriptionID: bodyEnv.SubscriptionID,
		Data: data,
	}, nil
}

// parseSignature accepts "t=<unix>,v1=<hex>" and the bare "<hex>" form.
func parseSignature(raw string) (int64, string, error) {
	if !strings.Contains(raw, "=") {
		return 0, raw, nil
	}
	var (
		timestamp int64
		signature string
	)
	for _, part := range strings.Split(raw, ",") {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch strings.TrimSpace(key) {
		case "t":
			parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
			if err != nil {
				return 0, "", fmt.Errorf("%w: bad timestamp", ErrSignatureInvalid)
			}
			timestamp = parsed
		case "v1", "v0", "s":
			signature = strings.TrimSpace(value)
		}
	}
	if signature == "" {
		return 0, "", fmt.Errorf("%w: no digest in signature header", ErrSignatureInvalid)
	}
	return timestamp, signature, nil
}
