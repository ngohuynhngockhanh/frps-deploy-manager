package auth

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTrustedLoopbackProxyHeaders(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := ClientIP(r); got != "203.0.113.9" {
		t.Fatalf("ClientIP = %q", got)
	}
	w := httptest.NewRecorder()
	SetSessionCookie(w, r, "token")
	if cookie := w.Header().Get("Set-Cookie"); !strings.Contains(cookie, "Secure") {
		t.Fatalf("expected Secure cookie, got %q", cookie)
	}
}

func TestUntrustedProxyHeadersAreIgnored(t *testing.T) {
	r := httptest.NewRequest("GET", "http://example.test/", nil)
	r.RemoteAddr = "198.51.100.4:12345"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	r.Header.Set("X-Forwarded-Proto", "https")
	if got := ClientIP(r); got != "198.51.100.4" {
		t.Fatalf("ClientIP = %q", got)
	}
	w := httptest.NewRecorder()
	SetSessionCookie(w, r, "token")
	if cookie := w.Header().Get("Set-Cookie"); strings.Contains(cookie, "Secure") {
		t.Fatalf("did not expect Secure from spoofed proxy headers, got %q", cookie)
	}
}
