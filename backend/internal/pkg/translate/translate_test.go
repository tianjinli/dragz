package translate

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// newTestClient builds a client whose providers point at the given servers, in
// the order supplied. Each server stands in for one public endpoint.
func newTestClient(t *testing.T, providers ...Provider) *Client {
	t.Helper()
	return NewWithProviders(providers, 3*time.Second)
}

func myMemoryServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/get" {
			t.Errorf("mymemory path = %q, want /get", got)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func googleServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/translate_a/single" {
			t.Errorf("google path = %q, want /translate_a/single", got)
		}
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTranslatePrefersTheFirstWorkingProvider(t *testing.T) {
	// MyMemory answers, so Google must never be called.
	var googleCalls atomic.Int32
	google := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		googleCalls.Add(1)
		_, _ = w.Write([]byte(`[[["unused"]]]`))
	}))
	t.Cleanup(google.Close)
	mm := myMemoryServer(t, `{"responseData":{"translatedText":"Department"},"responseStatus":200}`)

	client := newTestClient(t,
		&myMemoryProvider{transport: http.DefaultClient, baseURL: mm.URL},
		&googleProvider{transport: http.DefaultClient, baseURL: google.URL},
	)
	result, err := client.Translate(context.Background(), "部门", "zh", "en")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.Text != "Department" || result.Provider != "mymemory" || result.Cached {
		t.Fatalf("result = %+v, want mymemory/Department/cached=false", result)
	}
	if googleCalls.Load() != 0 {
		t.Fatalf("google called %d times, want 0", googleCalls.Load())
	}
}

func TestTranslateFallsBackWhenFirstProviderFails(t *testing.T) {
	// A quota reply is a 200 with a warning string, so this also covers the
	// "looks healthy, translated nothing" case.
	mm := myMemoryServer(t, `{"responseData":{"translatedText":"MYMEMORY WARNING: YOU USED ALL AVAILABLE FREE TRANSLATIONS FOR TODAY"},"responseStatus":200}`)
	google := googleServer(t, `[[["Head","部",null,null,10]],null,"zh-CN"]`)

	client := newTestClient(t,
		&myMemoryProvider{transport: http.DefaultClient, baseURL: mm.URL},
		&googleProvider{transport: http.DefaultClient, baseURL: google.URL},
	)
	result, err := client.Translate(context.Background(), "部", "zh", "en")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if result.Text != "Head" || result.Provider != "google" {
		t.Fatalf("result = %+v, want google/Head", result)
	}
}

func TestTranslateCachesRepeatedText(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		_, _ = w.Write([]byte(`{"responseData":{"translatedText":"Users"},"responseStatus":200}`))
	}))
	t.Cleanup(srv.Close)

	client := newTestClient(t, &myMemoryProvider{transport: http.DefaultClient, baseURL: srv.URL})
	first, err := client.Translate(context.Background(), "用户", "zh", "en")
	if err != nil {
		t.Fatalf("first Translate: %v", err)
	}
	if first.Cached {
		t.Fatal("first call reported cached")
	}
	second, err := client.Translate(context.Background(), "用户", "zh", "en")
	if err != nil {
		t.Fatalf("second Translate: %v", err)
	}
	if !second.Cached || second.Text != "Users" {
		t.Fatalf("second = %+v, want cached Users", second)
	}
	if calls.Load() != 1 {
		t.Fatalf("endpoint called %d times, want 1", calls.Load())
	}
	// The cache is keyed by language pair, so another target is a fresh call.
	if _, err := client.Translate(context.Background(), "用户", "zh", "ja"); err != nil {
		t.Fatalf("third Translate: %v", err)
	}
	if calls.Load() != 2 {
		t.Fatalf("endpoint called %d times after a new target language, want 2", calls.Load())
	}
}

func TestTranslateRejectsEmptyAndOversizedText(t *testing.T) {
	client := NewWithProviders(nil, time.Second)
	if _, err := client.Translate(context.Background(), "   ", "zh", "en"); !errors.Is(err, ErrEmptyText) {
		t.Fatalf("empty text err = %v, want ErrEmptyText", err)
	}
	long := strings.Repeat("长", MaxTextLength+1)
	if _, err := client.Translate(context.Background(), long, "zh", "en"); !errors.Is(err, ErrTooLong) {
		t.Fatalf("long text err = %v, want ErrTooLong", err)
	}
}

func TestTranslateReportsUnavailableWhenEveryProviderFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	t.Cleanup(srv.Close)

	client := newTestClient(t,
		&myMemoryProvider{transport: http.DefaultClient, baseURL: srv.URL},
		&googleProvider{transport: http.DefaultClient, baseURL: srv.URL},
	)
	_, err := client.Translate(context.Background(), "部门", "zh", "en")
	if !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestTranslateReportsUnavailableWithoutProviders(t *testing.T) {
	client := NewWithProviders(nil, time.Second)
	if _, err := client.Translate(context.Background(), "部门", "zh", "en"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
}

func TestGoogleConcatenatesSentenceSegments(t *testing.T) {
	// A two-segment reply: the text must come back as one string, not truncated
	// at the first segment.
	srv := googleServer(t, `[[["Clean up expired ","清理过期",null,null,10],["login sessions","登录会话",null,null,3]],null,"zh-CN"]`)
	client := newTestClient(t, &googleProvider{transport: http.DefaultClient, baseURL: srv.URL})
	result, err := client.Translate(context.Background(), "清理过期登录会话", "zh", "en")
	if err != nil {
		t.Fatalf("Translate: %v", err)
	}
	if want := "Clean up expired login sessions"; result.Text != want {
		t.Fatalf("text = %q, want %q", result.Text, want)
	}
}

func TestNewIgnoresUnknownProviderNames(t *testing.T) {
	client := New([]string{"nope", "Mymemory"}, time.Second)
	names := client.Providers()
	if len(names) != 1 || names[0] != "mymemory" {
		t.Fatalf("providers = %v, want [mymemory]", names)
	}
}

func TestProviderLangMapsProductCodes(t *testing.T) {
	cases := map[string]string{
		"zh":    "zh-CN",
		"zh-CN": "zh-CN",
		"zh-tw": "zh-TW",
		"en":    "en",
		"en-US": "en",
		"":      "",
	}
	for in, want := range cases {
		if got := providerLang(in); got != want {
			t.Errorf("providerLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLangPairJoinsNormalizedCodes(t *testing.T) {
	if got := langPair("zh", "en"); got != "zh-CN|en" {
		t.Fatalf("langPair = %q, want zh-CN|en", got)
	}
}

// TestTranslateHonoursContextCancellation guarantees a hanging endpoint cannot
// hold a request open past its deadline.
func TestTranslateHonoursContextCancellation(t *testing.T) {
	block := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-block
	}))
	t.Cleanup(func() { close(block); srv.Close() })

	client := NewWithProviders([]Provider{&myMemoryProvider{transport: http.DefaultClient, baseURL: srv.URL}}, 50*time.Millisecond)
	start := time.Now()
	if _, err := client.Translate(context.Background(), "部门", "zh", "en"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("err = %v, want ErrUnavailable", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("translate took %v, want it bounded by the timeout", elapsed)
	}
}
