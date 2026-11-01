// Package translate fills the target-language column of preset data by talking
// to free public translation services.
//
// The translation management page shows the Chinese source next to the target
// language. Typing the source is the only input an operator should have to
// give, so the target text is produced here instead of being typed twice.
// Nothing in this package needs an API key: a fresh install must be able to
// translate its first menu name without any setup.
//
// Those free endpoints are best-effort, which shapes the design:
//   - several providers are tried in order, so one blocked or rate-limited
//     service does not take the feature down (google is unreachable from
//     mainland China, mymemory is reachable but has a daily quota per IP);
//   - results are cached per process, so re-opening a form or re-translating
//     the same name does not spend quota twice;
//   - every failure degrades to an error the caller can show as "type it
//     yourself" — never to a silently mistranslated column.
package translate

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Errors callers may distinguish. Anything else is a transport failure.
var (
	// ErrEmptyText is returned for blank input, which no provider can translate.
	ErrEmptyText = errors.New("translate: empty text")
	// ErrTooLong is returned for text the free endpoints reject; the caller
	// should ask the operator to type the translation instead.
	ErrTooLong = errors.New("translate: text too long")
	// ErrUnavailable means every configured provider failed.
	ErrUnavailable = errors.New("translate: no provider available")
)

const (
	// DefaultTimeout bounds one provider attempt, DNS included.
	DefaultTimeout = 8 * time.Second
	// MaxTextLength is the longest source text worth sending. Preset names
	// (menus, buttons, roles, departments) are a few characters; the free
	// endpoints cap a query at roughly 500 bytes, which is ~160 Han characters.
	MaxTextLength = 160
	// cacheLimit keeps the process cache bounded; the working set is the few
	// hundred texts a preset seed contains.
	cacheLimit = 2000

	// myMemoryBaseURL and googleBaseURL are the keyless public endpoints. They
	// are constants rather than literals inside the providers so a deployment
	// behind a mirror can point the same code at a reachable host.
	myMemoryBaseURL = "https://api.mymemory.translated.net"
	googleBaseURL   = "https://translate.googleapis.com"
)

// Provider is one translation backend.
type Provider interface {
	// Name identifies the backend in results and logs.
	Name() string
	// Translate returns text translated from from into to. Language codes are
	// the product's short codes ("zh", "en"); providers normalize them.
	Translate(ctx context.Context, text, from, to string) (string, error)
}

// Result is a successful translation plus where it came from.
type Result struct {
	// Provider is the backend that produced the text.
	Provider string
	// Text is the translated text.
	Text string
	// Cached reports that Text came from the process cache rather than a call.
	Cached bool
}

// Client tries providers in order and caches what they return.
type Client struct {
	providers []Provider
	timeout   time.Duration
	transport *http.Client

	mu    sync.Mutex
	cache map[string]string
}

// New builds a client for the named providers, in priority order. Unknown names
// are ignored, so a typo in configuration degrades to the remaining providers
// instead of failing startup. An empty list falls back to DefaultProviders and a
// non-positive timeout to DefaultTimeout.
func New(providers []string, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	names := providers
	if len(names) == 0 {
		names = DefaultProviders()
	}
	transport := &http.Client{Timeout: timeout}
	built := make([]Provider, 0, len(names))
	for _, name := range names {
		switch strings.ToLower(strings.TrimSpace(name)) {
		case "mymemory":
			built = append(built, &myMemoryProvider{transport: transport, baseURL: myMemoryBaseURL})
		case "google":
			built = append(built, &googleProvider{transport: transport, baseURL: googleBaseURL})
		}
	}
	return newClient(built, timeout, transport)
}

// NewWithProviders builds a client over explicit providers. Tests use it to
// point the providers at a local server; callers that add a backend of their own
// use it to insert it into the chain.
func NewWithProviders(providers []Provider, timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return newClient(providers, timeout, &http.Client{Timeout: timeout})
}

func newClient(providers []Provider, timeout time.Duration, transport *http.Client) *Client {
	return &Client{
		providers: providers,
		timeout:   timeout,
		transport: transport,
		cache:     make(map[string]string),
	}
}

// DefaultProviders is the order used when configuration names none: mymemory
// first because it answers from mainland China, google as the fallback for
// networks where the reverse is true.
//
// It is a function, not a var, so callers cannot mutate the default in place.
func DefaultProviders() []string {
	return []string{"mymemory", "google"}
}

// Providers reports the backends this client will try, in order.
func (c *Client) Providers() []string {
	names := make([]string, 0, len(c.providers))
	for _, p := range c.providers {
		names = append(names, p.Name())
	}
	return names
}

// Translate returns text rendered in the target language.
//
// The returned error wraps ErrEmptyText, ErrTooLong or ErrUnavailable so the
// HTTP layer can pick a message; provider-level failures are folded into the
// last error, which is enough for a log line but not for control flow.
func (c *Client) Translate(ctx context.Context, text, from, to string) (Result, error) {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return Result{}, ErrEmptyText
	}
	if len([]rune(trimmed)) > MaxTextLength {
		return Result{}, ErrTooLong
	}
	key := from + "|" + to + "|" + trimmed
	if cached, ok := c.cached(key); ok {
		return Result{Provider: "cache", Text: cached, Cached: true}, nil
	}
	if len(c.providers) == 0 {
		return Result{}, ErrUnavailable
	}

	var lastErr error
	for _, provider := range c.providers {
		attempt, cancel := context.WithTimeout(ctx, c.timeout)
		out, err := provider.Translate(attempt, trimmed, from, to)
		cancel()
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", provider.Name(), err)
			continue
		}
		out = strings.TrimSpace(out)
		if out == "" {
			lastErr = fmt.Errorf("%s: empty result", provider.Name())
			continue
		}
		c.store(key, out)
		return Result{Provider: provider.Name(), Text: out}, nil
	}
	if lastErr == nil {
		lastErr = ErrUnavailable
	}
	return Result{}, fmt.Errorf("%w (%v)", ErrUnavailable, lastErr)
}

func (c *Client) cached(key string) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, ok := c.cache[key]
	return value, ok
}

func (c *Client) store(key, value string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.cache) >= cacheLimit {
		// The working set is a seed's worth of names, so a full cache means
		// something unusual; dropping it costs one extra call per entry.
		c.cache = make(map[string]string, cacheLimit)
	}
	c.cache[key] = value
}

// myMemoryProvider uses the public MyMemory translation memory. It needs no key
// and answers from mainland China, but a single IP gets a small daily quota; the
// over-quota reply is a normal 200 with a warning string, which is why the body
// is inspected rather than the status alone.
type myMemoryProvider struct {
	transport *http.Client
	baseURL   string
}

func (p *myMemoryProvider) Name() string { return "mymemory" }

func (p *myMemoryProvider) Translate(ctx context.Context, text, from, to string) (string, error) {
	endpoint := p.baseURL + "/get?q=" + url.QueryEscape(text) +
		"&langpair=" + url.QueryEscape(langPair(from, to))
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	var reply struct {
		ResponseData struct {
			TranslatedText string `json:"translatedText"`
		} `json:"responseData"`
	}
	if err := json.Unmarshal(body, &reply); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	out := strings.TrimSpace(reply.ResponseData.TranslatedText)
	// Quota and rate-limit replies arrive as text in the translatedText field.
	if out == "" || strings.Contains(strings.ToUpper(out), "MYMEMORY WARNING") || strings.Contains(strings.ToUpper(out), "QUERY LENGTH LIMIT") {
		return "", errors.New("no translation returned")
	}
	return out, nil
}

// googleProvider uses the keyless gtx endpoint behind translate.google.com. It
// is the best quality of the free options and unreachable from mainland China,
// so it is normally the fallback rather than the first attempt.
type googleProvider struct {
	transport *http.Client
	baseURL   string
}

func (p *googleProvider) Name() string { return "google" }

func (p *googleProvider) Translate(ctx context.Context, text, from, to string) (string, error) {
	endpoint := p.baseURL + "/translate_a/single?client=gtx&dt=t" +
		"&sl=" + url.QueryEscape(providerLang(from)) + "&tl=" + url.QueryEscape(providerLang(to)) +
		"&q=" + url.QueryEscape(text)
	body, err := p.get(ctx, endpoint)
	if err != nil {
		return "", err
	}
	// The reply is a nested array: [[[translation, source, ...], ...], ...].
	// Sentence-level segments are concatenated back into one string.
	var parsed []any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("decode: %w", err)
	}
	if len(parsed) == 0 {
		return "", errors.New("empty reply")
	}
	segments, ok := parsed[0].([]any)
	if !ok {
		return "", errors.New("unexpected reply shape")
	}
	var out strings.Builder
	for _, segment := range segments {
		parts, ok := segment.([]any)
		if !ok || len(parts) == 0 {
			continue
		}
		if piece, ok := parts[0].(string); ok {
			out.WriteString(piece)
		}
	}
	if strings.TrimSpace(out.String()) == "" {
		return "", errors.New("no translation returned")
	}
	return out.String(), nil
}

func (p *myMemoryProvider) get(ctx context.Context, endpoint string) ([]byte, error) {
	return httpGet(ctx, p.transport, endpoint)
}

func (p *googleProvider) get(ctx context.Context, endpoint string) ([]byte, error) {
	return httpGet(ctx, p.transport, endpoint)
}

func httpGet(ctx context.Context, client *http.Client, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "dragz-admin/1.0 (+preset translation)")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	// 1 MiB is far more than a short sentence needs and keeps a misbehaving
	// endpoint from streaming unbounded data into memory.
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d", resp.StatusCode)
	}
	return body, nil
}

// langPair renders the MyMemory language pair ("zh-CN|en").
func langPair(from, to string) string {
	return providerLang(from) + "|" + providerLang(to)
}

// providerLang maps the product's short codes onto the codes the endpoints
// expect. Source text is Chinese, so "zh" is expanded to the written form both
// services prefer; everything else is passed through as configured.
func providerLang(code string) string {
	switch strings.ToLower(strings.TrimSpace(code)) {
	case "zh", "zh-cn", "zh-hans":
		return "zh-CN"
	case "zh-tw", "zh-hant":
		return "zh-TW"
	case "en", "en-us", "en-gb":
		return "en"
	default:
		return strings.TrimSpace(code)
	}
}
