package httpapi

import "testing"

func TestValidOAuthURL(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
	}{
		{name: "https", value: "https://accounts.example.com/oauth/token", want: true},
		{name: "local development", value: "http://127.0.0.1:5280/api/auth/oauth/1/callback", want: true},
		{name: "missing scheme", value: "accounts.example.com/oauth", want: false},
		{name: "unsupported scheme", value: "file:///etc/passwd", want: false},
		{name: "credentials", value: "https://user:pass@example.com/token", want: false},
		{name: "fragment", value: "https://example.com/token#secret", want: false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := validOAuthURL(test.value); got != test.want {
				t.Fatalf("validOAuthURL(%q) = %v, want %v", test.value, got, test.want)
			}
		})
	}
}
