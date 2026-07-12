package crypto

import "testing"

func TestPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("NhapHang123@")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	ok, err := VerifyPassword("NhapHang123@", hash)
	if err != nil || !ok {
		t.Fatalf("verify correct: ok=%v err=%v", ok, err)
	}
	bad, err := VerifyPassword("wrong", hash)
	if err != nil || bad {
		t.Fatalf("verify wrong: bad=%v err=%v", bad, err)
	}
}

func TestVerifyRejectsBadFormat(t *testing.T) {
	if _, err := VerifyPassword("x", "not-a-phc-string"); err == nil {
		t.Fatal("expected error on malformed hash")
	}
}
