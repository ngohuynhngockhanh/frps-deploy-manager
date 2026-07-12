// Package config holds runtime configuration for the manager, resolved from
// flags and environment with sensible defaults.
package config

import (
	"os"
	"path/filepath"
	"strings"
)

// Config is the resolved application configuration.
type Config struct {
	// BindAddr is the loopback address the HTTP server listens on.
	BindAddr string
	// DataDir holds the SQLite DB, build cache, known_hosts and master key.
	DataDir string
	// FRPSrcDir is the local frp source tree used to cross-compile frps.
	FRPSrcDir string
	// FpmuSrcDir is the vendored fp-multiuser module directory.
	FpmuSrcDir string
	// GoBin is the Go toolchain used to cross-compile frps/fp-multiuser.
	GoBin string
	// AdminInitialPassword seeds the default admin account on first run.
	AdminInitialPassword string
}

// Default returns a Config populated with defaults, overridable via env.
func Default() Config {
	home, _ := os.UserHomeDir()
	return Config{
		BindAddr:             envOr("FDM_ADDR", "127.0.0.1:2050"),
		DataDir:              envOr("FDM_DATA_DIR", "data"),
		FRPSrcDir:            envOr("FDM_FRP_SRC", filepath.Join(home, "ksp-frp")),
		FpmuSrcDir:           envOr("FDM_FPMU_SRC", "third_party/fp-multiuser"),
		GoBin:                envOr("FDM_GO_BIN", filepath.Join(home, "go-sdk", "bin", "go")),
		AdminInitialPassword: envOr("ADMIN_INITIAL_PASSWORD", "NhapHang123@"),
	}
}

// DBPath is the SQLite database file path.
func (c Config) DBPath() string { return filepath.Join(c.DataDir, "app.db") }

// KnownHostsPath is where pinned SSH host keys are stored.
func (c Config) KnownHostsPath() string { return filepath.Join(c.DataDir, "known_hosts") }

// MasterKeyPath is the AES key file used to encrypt secrets at rest.
func (c Config) MasterKeyPath() string { return filepath.Join(c.DataDir, "master.key") }

// BuildCacheDir caches cross-compiled frps / fp-multiuser binaries.
func (c Config) BuildCacheDir() string { return filepath.Join(c.DataDir, "build-cache") }

// CloudflareTokenPath is the gitignored file holding the Cloudflare API token.
func (c Config) CloudflareTokenPath() string { return filepath.Join(c.DataDir, "cloudflare.token") }

// CloudflareToken returns the Cloudflare API token from env or the token file
// (empty if not configured).
func (c Config) CloudflareToken() string {
	if v := os.Getenv("FDM_CLOUDFLARE_TOKEN"); v != "" {
		return strings.TrimSpace(v)
	}
	b, _ := os.ReadFile(c.CloudflareTokenPath())
	return strings.TrimSpace(string(b))
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
