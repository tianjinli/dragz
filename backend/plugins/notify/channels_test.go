package notify

import (
	"strings"
	"testing"
	"time"
)

func TestAliyunPercentEncode(t *testing.T) {
	cases := map[string]string{
		"abc":       "abc",
		"a b":       "a%20b",
		"~!*()'":    "~%21%2A%28%29%27",
		"中文":       "%E4%B8%AD%E6%96%87",
		"a+b/c?d=e": "a%2Bb%2Fc%3Fd%3De",
	}
	for in, want := range cases {
		if got := aliyunPercentEncode(in); got != want {
			t.Errorf("aliyunPercentEncode(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestAliyunSignIsStableAndSecretDependent(t *testing.T) {
	params := map[string]string{
		"AccessKeyId": "testkey",
		"Action":      "SendSms",
		"PhoneNumbers": "13800138000",
		"SignName":    "测试签名",
	}
	sig1 := aliyunSign(params, "secret1")
	sig2 := aliyunSign(params, "secret1")
	sig3 := aliyunSign(params, "secret2")
	if sig1 != sig2 {
		t.Fatalf("signature not stable: %q vs %q", sig1, sig2)
	}
	if sig1 == sig3 {
		t.Fatal("signature does not depend on secret")
	}
	if strings.ContainsAny(sig1, "+/=") == false {
		// base64 of a 20-byte HMAC always has padding; sanity check only.
		t.Logf("signature: %s", sig1)
	}
}

func TestTC3SignShape(t *testing.T) {
	cfg := map[string]any{
		"secret_id":  "AKIDtest",
		"secret_key": "secretkey",
	}
	auth := tc3Sign(cfg, []byte(`{"a":1}`), time.Date(2026, 11, 23, 6, 0, 0, 0, time.UTC))
	for _, want := range []string{
		"TC3-HMAC-SHA256 Credential=AKIDtest/",
		"/sms/tc3_request",
		", SignedHeaders=content-type;host, Signature=",
	} {
		if !strings.Contains(auth, want) {
			t.Errorf("TC3 authorization %q missing %q", auth, want)
		}
	}
	// A different payload must change the signature.
	auth2 := tc3Sign(cfg, []byte(`{"a":2}`), time.Date(2026, 11, 23, 6, 0, 0, 0, time.UTC))
	if auth == auth2 {
		t.Fatal("TC3 signature does not depend on payload")
	}
}

func TestRender(t *testing.T) {
	got := render("Hello {{name}}, code {{code}}", map[string]any{"name": "Alice", "code": 1234})
	if got != "Hello Alice, code 1234" {
		t.Fatalf("render = %q", got)
	}
}

func TestChannelValidation(t *testing.T) {
	if err := registry["smtp"].Validate(map[string]any{}); err == nil {
		t.Fatal("empty smtp config must fail validation")
	}
	if err := registry["aliyun_sms"].Validate(map[string]any{
		"access_key_id": "a", "access_key_secret": "b", "sign_name": "c", "template_code": "d",
	}); err != nil {
		t.Fatalf("valid aliyun config rejected: %v", err)
	}
	if _, err := channelOf("unknown"); err == nil {
		t.Fatal("unknown channel must fail")
	}
}
