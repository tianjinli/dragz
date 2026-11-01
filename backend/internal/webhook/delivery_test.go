package webhook

import (
	"net"
	"testing"
)

func TestUnsafeIP(t *testing.T) {
	for _, value := range []string{"127.0.0.1", "10.0.0.1", "169.254.1.1", "::1"} {
		if !unsafeIP(net.ParseIP(value)) {
			t.Fatalf("expected %s to be rejected", value)
		}
	}
	if unsafeIP(net.ParseIP("8.8.8.8")) {
		t.Fatal("expected public IP to be allowed")
	}
}

func TestValidateTarget(t *testing.T) {
	if err := ValidateTarget("https://example.com/hooks/admin"); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"file:///tmp/hook", "ftp://example.com/hook", "https://user:pass@example.com/hook"} {
		if err := ValidateTarget(value); err == nil {
			t.Fatalf("expected %q to be rejected", value)
		}
	}
}
