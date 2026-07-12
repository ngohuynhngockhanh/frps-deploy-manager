package crypto

import (
	"bytes"
	"path/filepath"
	"testing"
)

func TestSealerRoundTrip(t *testing.T) {
	key := filepath.Join(t.TempDir(), "master.key")
	s, err := LoadOrCreateSealer(key)
	if err != nil {
		t.Fatalf("create sealer: %v", err)
	}
	plain := []byte("secret123")
	blob, err := s.Seal(plain)
	if err != nil {
		t.Fatalf("seal: %v", err)
	}
	if bytes.Contains(blob, plain) {
		t.Fatal("ciphertext should not contain plaintext")
	}
	got, err := s.Open(blob)
	if err != nil || !bytes.Equal(got, plain) {
		t.Fatalf("open: got=%q err=%v", got, err)
	}

	// A sealer reloaded from the same key file decrypts prior blobs.
	s2, err := LoadOrCreateSealer(key)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	got2, err := s2.Open(blob)
	if err != nil || !bytes.Equal(got2, plain) {
		t.Fatalf("reopen with reloaded key: got=%q err=%v", got2, err)
	}
}

func TestOpenRejectsShortBlob(t *testing.T) {
	s, _ := LoadOrCreateSealer(filepath.Join(t.TempDir(), "k"))
	if _, err := s.Open([]byte("x")); err == nil {
		t.Fatal("expected error on short blob")
	}
}
