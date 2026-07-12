package remote

import "testing"

func TestLooksLikeIPv4(t *testing.T) {
	ok := []string{"103.90.234.132", "0.0.0.0", "1.2.3.4", " 8.8.8.8 "}
	bad := []string{"", "1.2.3", "1.2.3.4.5", "1.2.3.x", "notanip", "1234.1.1.1"}
	for _, s := range ok {
		if !looksLikeIPv4(s) {
			t.Errorf("looksLikeIPv4(%q) = false, want true", s)
		}
	}
	for _, s := range bad {
		if looksLikeIPv4(s) {
			t.Errorf("looksLikeIPv4(%q) = true, want false", s)
		}
	}
}
