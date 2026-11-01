package security

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"strings"
	"testing"
	"unicode/utf16"
)

func TestNormalizePrivateKeyPEM_acceptsValidPKCS8(t *testing.T) {
	pemText := mustPKCS8PEM(t)
	got, err := NormalizePrivateKeyPEM(pemText)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_unwrapsNestedPEM(t *testing.T) {
	inner := mustPKCS8PEM(t)
	wrapped := wrapNestedPEM(inner)
	got, err := NormalizePrivateKeyPEM(wrapped)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_unwrapsNestedPEMWithCRLF(t *testing.T) {
	inner := mustPKCS8PEM(t)
	wrapped := strings.ReplaceAll(wrapNestedPEM(inner), "\n", "\r\n")
	got, err := NormalizePrivateKeyPEM(wrapped)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_unwrapsEscapedSixDashNestedPEM(t *testing.T) {
	inner := mustPKCS8PEM(t)
	wrapped := "\\------BEGIN PRIVATE KEY------\n" +
		base64.StdEncoding.EncodeToString([]byte(inner)) +
		"\n\\------END PRIVATE KEY------"
	got, err := NormalizePrivateKeyPEM(wrapped)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_unwrapsNestedPEMWithCROnBodyLines(t *testing.T) {
	inner := mustPKCS8PEM(t)
	b64 := base64.StdEncoding.EncodeToString([]byte(inner))
	var body strings.Builder
	for i := 0; i < len(b64); i += 64 {
		end := i + 64
		if end > len(b64) {
			end = len(b64)
		}
		body.WriteString(b64[i:end])
		body.WriteByte('\r')
		body.WriteByte('\n')
	}
	wrapped := "-----BEGIN PRIVATE KEY-----\r\n" + body.String() + "-----END PRIVATE KEY-----\r\n"
	got, err := NormalizePrivateKeyPEM(wrapped)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_utf8BOM(t *testing.T) {
	inner := mustPKCS8PEM(t)
	got, err := NormalizePrivateKeyPEM("\ufeff" + wrapNestedPEM(inner))
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_bareLS0tBody(t *testing.T) {
	inner := mustPKCS8PEM(t)
	bare := base64.StdEncoding.EncodeToString([]byte(inner))
	got, err := NormalizePrivateKeyPEM(bare)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_utf16LEWithBOM(t *testing.T) {
	inner := mustPKCS8PEM(t)
	wrapped := wrapNestedPEM(inner)
	encoded := encodeUTF16LEWithBOM(wrapped)
	got, err := NormalizePrivateKeyPEM(string(encoded))
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_bareMIIBody(t *testing.T) {
	inner := mustPKCS8PEM(t)
	block, _ := pem.Decode([]byte(inner))
	if block == nil {
		t.Fatal("decode fixture")
	}
	bare := base64.StdEncoding.EncodeToString(block.Bytes)
	got, err := NormalizePrivateKeyPEM(bare)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_literalBackslashN(t *testing.T) {
	inner := mustPKCS8PEM(t)
	escaped := strings.ReplaceAll(inner, "\n", `\n`)
	got, err := NormalizePrivateKeyPEM(escaped)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_rejectsCertificate(t *testing.T) {
	cert := "-----BEGIN CERTIFICATE-----\nMIIB\n-----END CERTIFICATE-----"
	_, err := NormalizePrivateKeyPEM(cert)
	if err == nil {
		t.Fatal("expected error for certificate pem")
	}
	if !strings.Contains(err.Error(), "certificate") {
		t.Fatalf("err = %v, want certificate hint", err)
	}
}

func TestNormalizePrivateKeyPEM_skipsLeadingCertificate(t *testing.T) {
	key := mustPKCS8PEM(t)
	cert := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: []byte{0x30, 0x03, 0x01, 0x01, 0x00}}))
	got, err := NormalizePrivateKeyPEM(cert + key)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_singleLinePKCS8(t *testing.T) {
	inner := mustPKCS8PEM(t)
	single := strings.ReplaceAll(inner, "\n", "")
	got, err := NormalizePrivateKeyPEM(single)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePrivateKeyPEM_singleLineNested(t *testing.T) {
	inner := mustPKCS8PEM(t)
	single := strings.ReplaceAll(wrapNestedPEM(inner), "\n", "")
	got, err := NormalizePrivateKeyPEM(single)
	if err != nil {
		t.Fatalf("NormalizePrivateKeyPEM() error = %v", err)
	}
	if err := validatePrivateKeyPEM(got); err != nil {
		t.Fatalf("normalized pem invalid: %v", err)
	}
}

func TestNormalizePublicKeyPEM_missingEndFooter(t *testing.T) {
	inner := mustPublicKeyPEM(t)
	// Strip END line to simulate truncated paste.
	lines := strings.Split(inner, "\n")
	var kept []string
	for _, line := range lines {
		if strings.Contains(line, "END ") {
			continue
		}
		kept = append(kept, line)
	}
	truncated := strings.Join(kept, "\n")
	got, err := NormalizePublicKeyPEM(truncated)
	if err != nil {
		t.Fatalf("NormalizePublicKeyPEM() error = %v", err)
	}
	if err := validatePublicKeyPEM(got); err != nil {
		t.Fatalf("normalized public pem invalid: %v", err)
	}
}

func TestNormalizePublicKeyPEM_singleLineNested(t *testing.T) {
	inner := mustPublicKeyPEM(t)
	single := strings.ReplaceAll(wrapNestedPublicPEM(inner), "\n", "")
	got, err := NormalizePublicKeyPEM(single)
	if err != nil {
		t.Fatalf("NormalizePublicKeyPEM() error = %v", err)
	}
	if err := validatePublicKeyPEM(got); err != nil {
		t.Fatalf("normalized public pem invalid: %v", err)
	}
}

func TestDescribePrivateKeyPEMShape_nested(t *testing.T) {
	inner := mustPKCS8PEM(t)
	shape := DescribePrivateKeyPEMShape(wrapNestedPEM(inner))
	if !shape.HasBegin || !shape.HasEnd {
		t.Fatalf("shape = %+v", shape)
	}
	if shape.BodyPrefix != "LS0t" {
		t.Fatalf("BodyPrefix = %q, want LS0t", shape.BodyPrefix)
	}
	if shape.SingleLine {
		t.Fatalf("expected multi-line shape, got SingleLine=true")
	}
}

func TestFormatPEMShapeLog(t *testing.T) {
	shape := PrivateKeyPEMShape{Len: 10, HasBegin: true, HasEnd: true, SingleLine: true, BodyPrefix: "LS0t"}
	got := FormatPEMShapeLog("private", shape, errors.New("boom"))
	for _, want := range []string{"kind=private", "len=10", "hasBegin=true", "hasEnd=true", "singleLine=true", "bodyPrefix=LS0t", "err=boom"} {
		if !strings.Contains(got, want) {
			t.Fatalf("FormatPEMShapeLog() = %q, missing %q", got, want)
		}
	}
}

func TestNormalizePublicKeyPEM_unwrapsNestedPEMWithCRLF(t *testing.T) {
	inner := mustPublicKeyPEM(t)
	wrapped := strings.ReplaceAll(wrapNestedPublicPEM(inner), "\n", "\r\n")
	got, err := NormalizePublicKeyPEM(wrapped)
	if err != nil {
		t.Fatalf("NormalizePublicKeyPEM() error = %v", err)
	}
	if err := validatePublicKeyPEM(got); err != nil {
		t.Fatalf("normalized public pem invalid: %v", err)
	}
}

func TestNormalizePublicKeyPEM_rejectsNestedPrivateKey(t *testing.T) {
	inner := mustPKCS8PEM(t)
	wrapped := "\\-----BEGIN PUBLIC KEY-----\n" +
		base64.StdEncoding.EncodeToString([]byte(inner)) +
		"\n\\-----END PUBLIC KEY-----"

	_, err := NormalizePublicKeyPEM(wrapped)
	if err == nil {
		t.Fatal("expected nested private key to be rejected")
	}
	if !strings.Contains(err.Error(), "private key") {
		t.Fatalf("error = %q, want private key hint", err)
	}
}

func wrapNestedPEM(inner string) string {
	return "-----BEGIN PRIVATE KEY-----\n" +
		base64.StdEncoding.EncodeToString([]byte(inner)) +
		"\n-----END PRIVATE KEY-----"
}

func wrapNestedPublicPEM(inner string) string {
	return "-----BEGIN PUBLIC KEY-----\n" +
		base64.StdEncoding.EncodeToString([]byte(inner)) +
		"\n-----END PUBLIC KEY-----"
}

func encodeUTF16LEWithBOM(s string) []byte {
	u16 := utf16.Encode([]rune(s))
	out := make([]byte, 2+len(u16)*2)
	out[0], out[1] = 0xFF, 0xFE
	for i, v := range u16 {
		binary.LittleEndian.PutUint16(out[2+i*2:], v)
	}
	return out
}

func mustPKCS8PEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func mustPublicKeyPEM(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}
