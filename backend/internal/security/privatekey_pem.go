package security

import (
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf16"
)

var pemBeginRE = regexp.MustCompile(`(?i)\\?-{5,}BEGIN ([^-]+?)-{5,}`)

// PrivateKeyPEMShape is safe diagnostic metadata (no key material).
type PrivateKeyPEMShape struct {
	Len        int
	HasBegin   bool
	HasEnd     bool
	SingleLine bool
	BodyPrefix string // LS0t | MII | other | empty
}

// DescribePrivateKeyPEMShape returns non-sensitive shape info for logs.
// Uses light cleanup only (not full repair) so SingleLine reflects stored form.
func DescribePrivateKeyPEMShape(raw string) PrivateKeyPEMShape {
	cleaned := sanitizePEMChars(stripUTF8BOM(raw))
	if decoded, ok := decodeUTF16IfNeeded([]byte(cleaned)); ok {
		cleaned = decoded
	}
	cleaned = normalizePEMNewlines(cleaned)
	shape := PrivateKeyPEMShape{Len: len(cleaned)}
	upper := strings.ToUpper(cleaned)
	shape.HasBegin = strings.Contains(upper, "BEGIN ")
	shape.HasEnd = strings.Contains(upper, "END ")
	shape.SingleLine = shape.HasBegin && shape.HasEnd && !strings.Contains(cleaned, "\n")
	body := pemBodyPrefix(cleaned)
	switch {
	case body == "":
		shape.BodyPrefix = "empty"
	case strings.HasPrefix(body, "LS0t"):
		shape.BodyPrefix = "LS0t"
	case strings.HasPrefix(body, "MII"):
		shape.BodyPrefix = "MII"
	default:
		if len(body) >= 4 {
			shape.BodyPrefix = "other:" + sanitizePrefix(body[:4])
		} else {
			shape.BodyPrefix = "other"
		}
	}
	return shape
}

// FormatPEMShapeLog renders a PEM shape for the logs. It reports only structural facts (length, markers, body prefix), never the key material itself.
func FormatPEMShapeLog(kind string, shape PrivateKeyPEMShape, err error) string {
	return fmt.Sprintf("kind=%s len=%d hasBegin=%v hasEnd=%v singleLine=%v bodyPrefix=%s err=%v",
		kind, shape.Len, shape.HasBegin, shape.HasEnd, shape.SingleLine, shape.BodyPrefix, err)
}

func sanitizePrefix(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteByte('?')
		}
	}
	return b.String()
}

func pemBodyPrefix(raw string) string {
	lines := strings.Split(raw, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-----") {
			continue
		}
		return line
	}
	return strings.TrimSpace(strings.Split(raw, "\n")[0])
}

// NormalizePrivateKeyPEM returns PKCS#8 PEM accepted by payment SDKs.
// Accepts standard apiclient_key.pem, WeChat nested PEM, cert+key paste bundles,
// single-line paste, literal "\n" escapes, and bare base64 DER (LS0t… / MII…).
func NormalizePrivateKeyPEM(raw string) (string, error) {
	raw = preparePEMInput(raw)
	if raw == "" {
		return "", errors.New("empty private key pem")
	}
	upper := strings.ToUpper(raw)
	hasPriv := looksLikePrivateKeyPEM(raw)
	if strings.Contains(upper, "BEGIN CERTIFICATE") && !hasPriv {
		return "", errors.New("got certificate pem; paste apiclient_key.pem private key")
	}
	if strings.Contains(upper, "BEGIN PUBLIC KEY") && !hasPriv {
		return "", errors.New("got public key pem; paste apiclient_key.pem private key")
	}
	if strings.Contains(upper, "BEGIN ENCRYPTED PRIVATE KEY") {
		return "", errors.New("encrypted private key not supported; use unencrypted apiclient_key.pem")
	}
	if !strings.Contains(raw, "BEGIN") {
		compact := strings.Join(strings.Fields(raw), "")
		switch {
		case strings.HasPrefix(compact, "LS0t"):
			decoded, err := decodeBase64Flexible(compact)
			if err == nil {
				inner := preparePEMInput(string(decoded))
				if looksLikePrivateKeyPEM(inner) {
					return normalizePrivateKeyPEMDepth(inner, 0)
				}
			}
			raw = "-----BEGIN PRIVATE KEY-----\n" + foldBase64(compact) + "\n-----END PRIVATE KEY-----"
		case strings.HasPrefix(compact, "MII"):
			raw = "-----BEGIN PRIVATE KEY-----\n" + foldBase64(compact) + "\n-----END PRIVATE KEY-----"
			if _, err := parseAndEncodePrivateKeyPEM(raw); err != nil {
				raw = "-----BEGIN RSA PRIVATE KEY-----\n" + foldBase64(compact) + "\n-----END RSA PRIVATE KEY-----"
			}
		}
	}
	return normalizePrivateKeyPEMDepth(raw, 0)
}

func normalizePrivateKeyPEMDepth(raw string, depth int) (string, error) {
	if depth > 3 {
		return "", errors.New("private key pem nesting too deep")
	}
	if encoded, err := parseAndEncodePrivateKeyPEM(raw); err == nil {
		return encoded, nil
	}
	unwrapped, err := unwrapNestedPrivateKeyPEM(raw)
	if err != nil {
		return "", fmt.Errorf("invalid private key pem: unwrap: %w", err)
	}
	if encoded, err := parseAndEncodePrivateKeyPEM(unwrapped); err == nil {
		return encoded, nil
	}
	normalized, err := normalizePrivateKeyPEMDepth(unwrapped, depth+1)
	if err != nil {
		return "", fmt.Errorf("invalid private key pem: after unwrap: %w", err)
	}
	return normalized, nil
}

func unwrapNestedPrivateKeyPEM(raw string) (string, error) {
	rest := []byte(raw)
	sawBlock := false
	for len(rest) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		sawBlock = true
		typ := strings.ToUpper(strings.TrimSpace(block.Type))
		if typ != "PRIVATE KEY" && typ != "RSA PRIVATE KEY" {
			continue
		}
		inner := strings.TrimSpace(string(block.Bytes))
		if looksLikePrivateKeyPEM(inner) {
			return preparePEMInput(inner), nil
		}
		if decoded, err := decodeBase64Flexible(strings.Join(strings.Fields(inner), "")); err == nil {
			asText := strings.TrimSpace(string(decoded))
			if looksLikePrivateKeyPEM(asText) {
				return preparePEMInput(asText), nil
			}
		}
	}
	if !sawBlock {
		return "", errors.New("invalid pem block")
	}
	return "", errors.New("inner content is not pem")
}

func looksLikePrivateKeyPEM(s string) bool {
	upper := strings.ToUpper(s)
	return strings.Contains(upper, "BEGIN PRIVATE KEY") ||
		strings.Contains(upper, "BEGIN RSA PRIVATE KEY")
}

// parseAndEncodePrivateKeyPEM finds the first parseable private-key PEM block
// (skipping certificates / public keys in multi-block pastes) and returns PKCS#8 PEM.
func parseAndEncodePrivateKeyPEM(raw string) (string, error) {
	rest := []byte(raw)
	sawPriv := false
	for len(rest) > 0 {
		block, next := pem.Decode(rest)
		if block == nil {
			break
		}
		rest = next
		typ := strings.ToUpper(strings.TrimSpace(block.Type))
		switch typ {
		case "ENCRYPTED PRIVATE KEY":
			return "", errors.New("encrypted private key not supported; use unencrypted apiclient_key.pem")
		case "PRIVATE KEY":
			sawPriv = true
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				return marshalPKCS8PEM(key)
			}
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				return marshalPKCS8PEM(key)
			}
		case "RSA PRIVATE KEY":
			sawPriv = true
			if key, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
				return marshalPKCS8PEM(key)
			}
			if key, err := x509.ParsePKCS8PrivateKey(block.Bytes); err == nil {
				return marshalPKCS8PEM(key)
			}
		}
	}
	if !sawPriv && looksLikePrivateKeyPEM(raw) {
		return "", errors.New("invalid pem block")
	}
	if !sawPriv {
		return "", errors.New("no private key pem block found")
	}
	return "", errors.New("unsupported private key format")
}

func marshalPKCS8PEM(key any) (string, error) {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return "", err
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})), nil
}

func validatePrivateKeyPEM(raw string) error {
	_, err := parseAndEncodePrivateKeyPEM(raw)
	return err
}

// NormalizePublicKeyPEM returns a PEM accepted by utils.LoadPublicKey.
func NormalizePublicKeyPEM(raw string) (string, error) {
	raw = preparePEMInput(raw)
	if raw == "" {
		return "", errors.New("empty public key pem")
	}
	if looksLikePrivateKeyPEM(raw) {
		return "", errors.New("got private key pem; paste pub_key.pem public key")
	}
	if !strings.Contains(raw, "BEGIN") && strings.HasPrefix(strings.TrimSpace(raw), "LS0t") {
		decoded, err := decodeBase64Flexible(strings.Join(strings.Fields(raw), ""))
		if err == nil {
			inner := preparePEMInput(string(decoded))
			if strings.Contains(inner, "BEGIN PUBLIC KEY") || strings.Contains(inner, "BEGIN RSA PUBLIC KEY") {
				return normalizePublicKeyPEMDepth(inner, 0)
			}
			if looksLikePrivateKeyPEM(inner) {
				return "", errors.New("got private key pem; paste pub_key.pem public key")
			}
		}
		raw = "-----BEGIN PUBLIC KEY-----\n" + foldBase64(strings.Join(strings.Fields(raw), "")) + "\n-----END PUBLIC KEY-----"
	}
	return normalizePublicKeyPEMDepth(raw, 0)
}

func normalizePublicKeyPEMDepth(raw string, depth int) (string, error) {
	if depth > 3 {
		return "", errors.New("public key pem nesting too deep")
	}
	if err := validatePublicKeyPEM(raw); err == nil {
		return raw, nil
	}
	unwrapped, err := unwrapNestedPublicKeyPEM(raw)
	if err != nil {
		return "", fmt.Errorf("invalid public key pem: unwrap: %w", err)
	}
	if err := validatePublicKeyPEM(unwrapped); err == nil {
		return unwrapped, nil
	}
	normalized, err := normalizePublicKeyPEMDepth(unwrapped, depth+1)
	if err != nil {
		return "", fmt.Errorf("invalid public key pem: after unwrap: %w", err)
	}
	return normalized, nil
}

func unwrapNestedPublicKeyPEM(raw string) (string, error) {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return "", errors.New("invalid pem block")
	}
	inner := strings.TrimSpace(string(block.Bytes))
	if strings.Contains(inner, "BEGIN PUBLIC KEY") || strings.Contains(inner, "BEGIN RSA PUBLIC KEY") {
		return preparePEMInput(inner), nil
	}
	if looksLikePrivateKeyPEM(inner) {
		return "", errors.New("got private key pem; paste pub_key.pem public key")
	}
	if decoded, err := decodeBase64Flexible(strings.Join(strings.Fields(inner), "")); err == nil {
		asText := strings.TrimSpace(string(decoded))
		if strings.Contains(asText, "BEGIN PUBLIC KEY") || strings.Contains(asText, "BEGIN RSA PUBLIC KEY") {
			return preparePEMInput(asText), nil
		}
		if looksLikePrivateKeyPEM(asText) {
			return "", errors.New("got private key pem; paste pub_key.pem public key")
		}
	}
	return "", errors.New("inner content is not pem")
}

func validatePublicKeyPEM(raw string) error {
	block, _ := pem.Decode([]byte(raw))
	if block == nil {
		return errors.New("invalid pem block")
	}
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return err
	}
	if pub == nil {
		return errors.New("empty public key")
	}
	return nil
}

func preparePEMInput(raw string) string {
	raw = stripUTF8BOM(raw)
	if decoded, ok := decodeUTF16IfNeeded([]byte(raw)); ok {
		raw = decoded
	}
	raw = sanitizePEMChars(raw)
	raw = unescapePEMLiterals(raw)
	raw = normalizePEMNewlines(raw)
	raw = repairPEMLineBreaks(raw)
	return strings.TrimSpace(raw)
}

// unescapePEMLiterals turns pasted JSON-style "\n" sequences into real newlines
// when the payload still looks like a PEM (BEGIN present, no real line breaks).
func unescapePEMLiterals(raw string) string {
	if raw == "" || !strings.Contains(raw, `\n`) {
		return raw
	}
	upper := strings.ToUpper(raw)
	if !strings.Contains(upper, "BEGIN ") {
		return raw
	}
	if strings.Contains(raw, "\n") && !strings.Contains(raw, `\n`) {
		return raw
	}
	// Only rewrite when escapes are doing the line-breaking (single-line or mostly escaped).
	if strings.Count(raw, "\n") > 2 && strings.Count(raw, `\n`) < 2 {
		return raw
	}
	raw = strings.ReplaceAll(raw, `\r\n`, "\n")
	raw = strings.ReplaceAll(raw, `\n`, "\n")
	raw = strings.ReplaceAll(raw, `\r`, "\n")
	return raw
}

func sanitizePEMChars(raw string) string {
	replacer := strings.NewReplacer(
		"\u200b", "", // zero-width space
		"\u200c", "",
		"\u200d", "",
		"\ufeff", "",
		"\u00a0", " ", // nbsp
		"－", "-", // fullwidth hyphen-minus
		"—", "-", // em dash
		"–", "-", // en dash
		"﹣", "-", // small hyphen-minus
		"\u201c", "", // left double quote
		"\u201d", "",
		"\u2018", "",
		"\u2019", "",
	)
	return replacer.Replace(raw)
}

// repairPEMLineBreaks turns single-line / mangled PEM into standard multi-line PEM.
func repairPEMLineBreaks(raw string) string {
	if raw == "" {
		return raw
	}
	if block, _ := pem.Decode([]byte(raw)); block != nil {
		return raw
	}
	if repaired := rebuildPEMBlocks(raw); repaired != "" {
		if block, _ := pem.Decode([]byte(repaired)); block != nil {
			return repaired
		}
		raw = repaired
	}
	// BEGIN present but END missing (truncated paste): append matching END.
	if strings.Contains(strings.ToUpper(raw), "BEGIN ") && !strings.Contains(strings.ToUpper(raw), "END ") {
		if m := pemBeginRE.FindStringSubmatch(raw); len(m) == 2 {
			typ := strings.TrimSpace(m[1])
			body := strings.TrimSpace(raw[len(m[0]):])
			body = strings.Join(strings.Fields(body), "")
			if body != "" {
				candidate := formatPEMBlock(typ, body)
				if block, _ := pem.Decode([]byte(candidate)); block != nil {
					return candidate
				}
			}
		}
	}
	return raw
}

func rebuildPEMBlocks(raw string) string {
	var parts []string
	rest := raw
	for {
		m := pemBeginRE.FindStringSubmatchIndex(rest)
		if m == nil {
			break
		}
		typ := strings.TrimSpace(rest[m[2]:m[3]])
		afterBegin := rest[m[1]:]
		endRE := regexp.MustCompile(`(?i)\\?-{5,}END ` + regexp.QuoteMeta(typ) + `-{5,}`)
		endMatch := endRE.FindStringIndex(afterBegin)
		if endMatch == nil {
			break
		}
		endIdx := endMatch[0]
		body := strings.Join(strings.Fields(afterBegin[:endIdx]), "")
		parts = append(parts, formatPEMBlock(typ, body))
		rest = afterBegin[endMatch[1]:]
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func formatPEMBlock(typ, body string) string {
	return "-----BEGIN " + typ + "-----\n" + foldBase64(body) + "\n-----END " + typ + "-----"
}

func foldBase64(s string) string {
	s = strings.Join(strings.Fields(s), "")
	if s == "" {
		return ""
	}
	var b strings.Builder
	for i := 0; i < len(s); i += 64 {
		end := i + 64
		if end > len(s) {
			end = len(s)
		}
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(s[i:end])
	}
	return b.String()
}

func stripUTF8BOM(raw string) string {
	return strings.TrimPrefix(raw, "\ufeff")
}

func decodeUTF16IfNeeded(b []byte) (string, bool) {
	if len(b) < 2 {
		return "", false
	}
	var order binary.ByteOrder
	switch {
	case b[0] == 0xFF && b[1] == 0xFE:
		order = binary.LittleEndian
		b = b[2:]
	case b[0] == 0xFE && b[1] == 0xFF:
		order = binary.BigEndian
		b = b[2:]
	default:
		if len(b) < 8 || !looksLikeUTF16(b) {
			return "", false
		}
		if b[1] == 0 && b[3] == 0 {
			order = binary.LittleEndian
		} else if b[0] == 0 && b[2] == 0 {
			order = binary.BigEndian
		} else {
			return "", false
		}
	}
	if len(b)%2 != 0 {
		b = b[:len(b)-1]
	}
	u16 := make([]uint16, len(b)/2)
	for i := 0; i < len(u16); i++ {
		u16[i] = order.Uint16(b[i*2:])
	}
	return string(utf16.Decode(u16)), true
}

func looksLikeUTF16(b []byte) bool {
	n := len(b)
	if n > 64 {
		n = 64
	}
	zeros := 0
	for i := 0; i < n; i++ {
		if b[i] == 0 {
			zeros++
		}
	}
	return zeros >= n/4
}

func normalizePEMNewlines(raw string) string {
	raw = strings.ReplaceAll(raw, "\r\n", "\n")
	raw = strings.ReplaceAll(raw, "\r", "\n")
	return strings.TrimSpace(raw)
}

func decodeBase64Flexible(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil, errors.New("empty base64")
	}
	if decoded, err := base64.StdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	if decoded, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return decoded, nil
	}
	return base64.StdEncoding.DecodeString(padBase64(s))
}

func padBase64(s string) string {
	switch len(s) % 4 {
	case 2:
		return s + "=="
	case 3:
		return s + "="
	default:
		return s
	}
}
