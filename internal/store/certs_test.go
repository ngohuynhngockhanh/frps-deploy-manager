package store

import (
	"testing"
	"time"
)

func TestCertUpsertAndDays(t *testing.T) {
	st := testStore(t)
	s, err := st.CreateServer(sampleServer())
	if err != nil {
		t.Fatalf("server: %v", err)
	}

	future := time.Now().Add(60 * 24 * time.Hour).Unix()
	c := Cert{ServerID: s.ID, Domain: "io.vnmap3d.com", Issuer: "R10", NotAfter: future, Serial: "abc"}
	if err := st.UpsertCert(c); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	got, err := st.GetCert(s.ID, "io.vnmap3d.com")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if d := got.DaysRemaining(); d < 59 || d > 60 {
		t.Errorf("DaysRemaining = %d, want ~60", d)
	}

	// Upsert again updates in place (no duplicate).
	c.Issuer = "R11"
	if err := st.UpsertCert(c); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	got2, _ := st.GetCert(s.ID, "io.vnmap3d.com")
	if got2.Issuer != "R11" {
		t.Errorf("issuer not updated: %q", got2.Issuer)
	}
	list, _ := st.ListCertsWithDomain()
	if len(list) != 1 {
		t.Errorf("expected 1 cert, got %d", len(list))
	}
}
