package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"dragz-admin/backend/internal/app"
	"dragz-admin/backend/internal/pkg/translate"
)

// stubProvider is a Provider whose answer is fixed, so a handler test can cover
// success and failure without leaving the process.
type stubProvider struct {
	out string
	err error
}

func (s stubProvider) Name() string { return "stub" }

func (s stubProvider) Translate(context.Context, string, string, string) (string, error) {
	return s.out, s.err
}

// translateReply is the envelope the translate endpoint answers with.
type translateReply struct {
	Code int `json:"code"`
	Data struct {
		Text     string `json:"text"`
		Provider string `json:"provider"`
		Cached   bool   `json:"cached"`
	} `json:"data"`
	Message string `json:"message"`
}

// translateTestContext builds a context whose translator is the given provider.
func translateTestContext(provider translate.Provider) *app.Context {
	return &app.Context{
		Translate: translate.NewWithProviders([]translate.Provider{provider}, time.Second),
	}
}

func callTranslate(t *testing.T, ctx *app.Context, body string) (*httptest.ResponseRecorder, translateReply) {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/system/translations/translate", strings.NewReader(body))
	translateTextHandler(ctx)(rec, req)
	var reply translateReply
	if err := json.Unmarshal(rec.Body.Bytes(), &reply); err != nil {
		t.Fatalf("decode reply: %v (body %s)", err, rec.Body.String())
	}
	return rec, reply
}

func TestTranslateHandlerReturnsTranslatedText(t *testing.T) {
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "Department"}), `{"text":"部门"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if reply.Data.Text != "Department" || reply.Data.Provider != "stub" {
		t.Fatalf("data = %+v, want stub/Department", reply.Data)
	}
}

func TestTranslateHandlerRejectsEmptyText(t *testing.T) {
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "x"}), `{"text":"   "}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if reply.Message != "errors.translation_source_required" {
		t.Fatalf("message = %q, want errors.translation_source_required", reply.Message)
	}
}

func TestTranslateHandlerRejectsOversizedText(t *testing.T) {
	body, err := json.Marshal(map[string]string{"text": strings.Repeat("长", translate.MaxTextLength+1)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "x"}), string(body))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if reply.Message != "errors.translation_text_too_long" {
		t.Fatalf("message = %q, want errors.translation_text_too_long", reply.Message)
	}
}

func TestTranslateHandlerReportsUnavailableDependency(t *testing.T) {
	// Every provider failing is a dependency problem, not a bad request: the
	// page turns 502 into "type the translation yourself".
	rec, reply := callTranslate(t, translateTestContext(stubProvider{err: errors.New("boom")}), `{"text":"部门"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if reply.Message != "errors.translation_service_unavailable" {
		t.Fatalf("message = %q, want errors.translation_service_unavailable", reply.Message)
	}
}

func TestTranslateHandlerWithoutClientIsUnavailable(t *testing.T) {
	rec, reply := callTranslate(t, &app.Context{}, `{"text":"部门"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
	if reply.Message != "errors.translation_service_unavailable" {
		t.Fatalf("message = %q, want errors.translation_service_unavailable", reply.Message)
	}
}

func TestTranslateHandlerRejectsUnsupportedTarget(t *testing.T) {
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "x"}), `{"text":"部门","target":"ja"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if reply.Message != "errors.translation_lang_unsupported" {
		t.Fatalf("message = %q, want errors.translation_lang_unsupported", reply.Message)
	}
}

func TestTranslateHandlerTranslatesBackIntoChinese(t *testing.T) {
	// Both columns carry a translate button, so a translator who already has the
	// English text asks for the Chinese one: the request names English as source
	// and Chinese as target.
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "部门"}),
		`{"text":"Department","source":"en","target":"zh"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (body %s)", rec.Code, rec.Body.String())
	}
	if reply.Data.Text != "部门" {
		t.Fatalf("text = %q, want 部门", reply.Data.Text)
	}
}

func TestTranslateHandlerRejectsUnsupportedSource(t *testing.T) {
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "x"}),
		`{"text":"Department","source":"ja"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if reply.Message != "errors.translation_lang_unsupported" {
		t.Fatalf("message = %q, want errors.translation_lang_unsupported", reply.Message)
	}
}

func TestTranslateHandlerRejectsSameLanguagePair(t *testing.T) {
	// Echoing the input back would look like a successful translation and let the
	// page overwrite one column with the other's text.
	rec, reply := callTranslate(t, translateTestContext(stubProvider{out: "x"}),
		`{"text":"部门","source":"zh","target":"zh"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if reply.Message != "errors.translation_invalid_request" {
		t.Fatalf("message = %q, want errors.translation_invalid_request", reply.Message)
	}
}

func TestTranslateHandlerRejectsMalformedBody(t *testing.T) {
	rec, _ := callTranslate(t, translateTestContext(stubProvider{out: "x"}), `{"text":`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
