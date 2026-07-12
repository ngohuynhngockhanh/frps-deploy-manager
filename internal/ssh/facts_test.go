package ssh

import "testing"

func TestPortOf(t *testing.T) {
	cases := map[string]int{
		"0.0.0.0:9000":       9000,
		"[::]:9001":          9001,
		"127.0.0.1:9003":     9003,
		"*:22":               22,
		"garbage":            0,
		"1.2.3.4:notaport":   0,
	}
	for in, want := range cases {
		if got := portOf(in); got != want {
			t.Errorf("portOf(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestSafeFingerprintBadInput(t *testing.T) {
	if fp := safeFingerprint("not-a-key"); fp != "" {
		t.Errorf("expected empty fingerprint for bad input, got %q", fp)
	}
}
