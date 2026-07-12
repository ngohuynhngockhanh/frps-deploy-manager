package builder

import (
	"os"
	"path/filepath"
	"testing"
)

// newTestBuilder wires the builder to the real local sources, skipping the test
// when the toolchain or sources are unavailable.
func newTestBuilder(t *testing.T) *Builder {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping cross-compile build in -short mode")
	}
	home, _ := os.UserHomeDir()
	goBin := filepath.Join(home, "go-sdk", "bin", "go")
	if _, err := os.Stat(goBin); err != nil {
		t.Skipf("go toolchain not found at %s", goBin)
	}
	frp := filepath.Join(home, "ksp-frp")
	if _, err := os.Stat(frp); err != nil {
		t.Skipf("frp source not found at %s", frp)
	}
	// Package CWD is internal/builder; repo root is two levels up.
	fpmu := filepath.Join("..", "..", "third_party", "fp-multiuser")
	return &Builder{GoBin: goBin, FRPSrcDir: frp, FpmuSrcDir: fpmu, CacheDir: t.TempDir()}
}

func TestBuildFrps(t *testing.T) {
	b := newTestBuilder(t)
	a, err := b.BuildFrps(false)
	if err != nil {
		t.Fatalf("build frps: %v", err)
	}
	if a.Version != "0.54.0" {
		t.Errorf("frps version = %q, want 0.54.0", a.Version)
	}
	if _, err := os.Stat(a.Path); err != nil {
		t.Errorf("artifact missing: %v", err)
	}
	// Second call should hit the cache and return the same path.
	a2, err := b.BuildFrps(false)
	if err != nil || a2.Path != a.Path {
		t.Errorf("expected cache hit; got path=%q err=%v", a2.Path, err)
	}
}

func TestBuildFpMultiuser(t *testing.T) {
	b := newTestBuilder(t)
	a, err := b.BuildFpMultiuser(false)
	if err != nil {
		t.Fatalf("build fp-multiuser: %v", err)
	}
	if a.Version != "0.0.2" {
		t.Errorf("fp-multiuser version = %q, want 0.0.2", a.Version)
	}
}
