// Package webhook delivers outbound webhook events, signing each payload and
// blocking private-network targets.
package webhook

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

// Target is one resolved webhook destination. Secret and Headers arrive already decrypted.
type Target struct {
	URL                 string
	Secret              string
	Headers             map[string]string
	AllowPrivateNetwork bool
	Timeout             time.Duration
	MaxRetries          int
}

// Result describes the outcome of a delivery: the event id, the last status code, how many attempts were made and the truncated response body.
type Result struct {
	EventID    string `json:"eventId"`
	StatusCode int    `json:"statusCode"`
	Attempts   int    `json:"attempts"`
	Response   string `json:"response"`
}

// Deliver posts one event, signing the payload with HMAC-SHA256 over timestamp and body, and retries until MaxRetries is exhausted. Only a 2xx response counts as success.
func Deliver(ctx context.Context, target Target, event string, data any) (Result, error) {
	eventID := uuid.NewString()
	payload, err := json.Marshal(map[string]any{
		"id": eventID, "event": event, "occurredAt": time.Now().UTC(), "data": data,
	})
	if err != nil {
		return Result{}, err
	}
	client := &http.Client{
		Timeout:   target.Timeout,
		Transport: safeTransport(target.AllowPrivateNetwork),
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return errors.New("too many webhook redirects")
			}
			return validateURL(request.URL)
		},
	}
	var lastErr error
	result := Result{EventID: eventID}
	for attempt := 1; attempt <= target.MaxRetries+1; attempt++ {
		result.Attempts = attempt
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, target.URL, bytes.NewReader(payload))
		if err != nil {
			return result, err
		}
		timestamp := strconv.FormatInt(time.Now().Unix(), 10)
		signature := sign(target.Secret, timestamp, payload)
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("User-Agent", "Universal-Admin-Webhook/1.0")
		request.Header.Set("X-Webhook-ID", eventID)
		request.Header.Set("X-Webhook-Event", event)
		request.Header.Set("X-Webhook-Timestamp", timestamp)
		request.Header.Set("X-Webhook-Signature", "sha256="+signature)
		for key, value := range target.Headers {
			if !reservedHeader(key) {
				request.Header.Set(key, value)
			}
		}
		response, err := client.Do(request)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 4096))
		_ = response.Body.Close()
		result.StatusCode, result.Response = response.StatusCode, string(body)
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if response.StatusCode >= 200 && response.StatusCode < 300 {
			return result, nil
		}
		lastErr = fmt.Errorf("webhook returned HTTP %d", response.StatusCode)
	}
	return result, lastErr
}

// ValidateTarget checks that a URL is http(s) with a real host. It does not resolve the host: the private-network policy is enforced at dial time so a DNS rebind cannot slip past.
func ValidateTarget(rawURL string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return err
	}
	return validateURL(parsed)
}

func validateURL(parsed *url.URL) error {
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return errors.New("webhook URL must use http or https")
	}
	if parsed.Hostname() == "" || parsed.User != nil {
		return errors.New("webhook URL host is invalid")
	}
	return nil
}

func safeTransport(allowPrivate bool) *http.Transport {
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	return &http.Transport{
		Proxy:           nil,
		TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12},
		DialContext: func(ctx context.Context, network, address string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(address)
			if err != nil {
				return nil, err
			}
			addresses, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, err
			}
			for _, address := range addresses {
				if !allowPrivate && unsafeIP(address.IP) {
					continue
				}
				connection, err := dialer.DialContext(ctx, network, net.JoinHostPort(address.IP.String(), port))
				if err == nil {
					return connection, nil
				}
			}
			return nil, errors.New("webhook target has no permitted address")
		},
	}
}

func unsafeIP(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast()
}

func sign(secret, timestamp string, payload []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	_, _ = mac.Write([]byte(timestamp))
	_, _ = mac.Write([]byte("."))
	_, _ = mac.Write(payload)
	return hex.EncodeToString(mac.Sum(nil))
}

func reservedHeader(key string) bool {
	switch strings.ToLower(strings.TrimSpace(key)) {
	case "host", "content-length", "content-type", "user-agent", "x-webhook-id",
		"x-webhook-event", "x-webhook-timestamp", "x-webhook-signature":
		return true
	default:
		return false
	}
}
