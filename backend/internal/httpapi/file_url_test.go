package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSystemPublicFileURL(t *testing.T) {
	for _, tt := range []struct {
		name, origin, forwardedProto, forwardedHost, want string
	}{
		{"https origin", "https://admin.example/api/system/files/upload", "", "", "https://admin.example/api/system/files/42/content"},
		{"http localhost", "http://localhost:8888/api/system/files/upload", "", "", "http://localhost:8888/api/system/files/42/content"},
		{"forwarded https", "http://admin.example/api/system/files/upload", "https", "", "https://admin.example/api/system/files/42/content"},
		{"forwarded host", "http://internal:8080/api/system/files/upload", "https", "cdn.example", "https://cdn.example/api/system/files/42/content"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodPost, tt.origin, nil)
			if tt.forwardedProto != "" {
				r.Header.Set("X-Forwarded-Proto", tt.forwardedProto)
			}
			if tt.forwardedHost != "" {
				r.Header.Set("X-Forwarded-Host", tt.forwardedHost)
			}
			if got := systemPublicFileURL(r, 42); got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}
