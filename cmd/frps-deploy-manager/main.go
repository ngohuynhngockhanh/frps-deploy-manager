// Command frps-deploy-manager is a local admin tool to deploy and manage frps
// (frp server) instances on remote VPS hosts over SSH, from the operator's
// workstation. It listens only on loopback (127.0.0.1:2050) behind a login.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/builder"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/config"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/httpapi"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/remote"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/web"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		addr        string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&addr, "addr", "", "local bind address (loopback); overrides FDM_ADDR")
	flag.Parse()

	if showVersion {
		fmt.Printf("frps-deploy-manager %s\n", version)
		return
	}

	cfg := config.Default()
	if addr != "" {
		cfg.BindAddr = addr
	}

	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run(cfg config.Config) error {
	if err := os.MkdirAll(cfg.DataDir, 0o700); err != nil {
		return fmt.Errorf("create data dir: %w", err)
	}

	st, err := store.Open(cfg.DBPath())
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer st.Close()

	authMgr := auth.NewManager(st)
	if err := authMgr.SeedAdmin("admin", cfg.AdminInitialPassword); err != nil {
		return fmt.Errorf("seed admin: %w", err)
	}

	sealer, err := crypto.LoadOrCreateSealer(cfg.MasterKeyPath())
	if err != nil {
		return fmt.Errorf("init sealer: %w", err)
	}
	st.SetSealer(sealer) // encrypt dashboard passwords at rest
	bld := &builder.Builder{
		GoBin: cfg.GoBin, FRPSrcDir: cfg.FRPSrcDir,
		FpmuSrcDir: cfg.FpmuSrcDir, CacheDir: cfg.BuildCacheDir(),
	}
	deployer := &remote.Deployer{Store: st, Builder: bld, Sealer: sealer, CFToken: loadCloudflareToken(cfg, sealer)}

	renderer, err := web.NewRenderer()
	if err != nil {
		return fmt.Errorf("parse templates: %w", err)
	}

	deps := httpapi.Deps{
		Render: renderer, Store: st, Auth: authMgr,
		Deployer: deployer, Sealer: sealer,
	}
	handler := httpapi.NewRouter(deps)

	// Security: this tool holds SSH-root over managed VPS. Warn if it listens on
	// anything but loopback (must then be a trusted/VPN network, never internet).
	if !strings.HasPrefix(cfg.BindAddr, "127.0.0.1") && !strings.HasPrefix(cfg.BindAddr, "localhost") {
		log.Printf("SECURITY WARNING: listening on %s (non-loopback) over plain HTTP. "+
			"Keep this on a trusted/VPN network + firewall; NEVER expose to the internet.", cfg.BindAddr)
	}

	srv := &http.Server{
		Addr:              cfg.BindAddr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Graceful shutdown on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Background certificate-expiry refresh (every 12h, after a short delay).
	go certRefreshLoop(ctx, deployer)

	errCh := make(chan error, 1)
	go func() {
		log.Printf("frps-deploy-manager %s listening on http://%s", version, cfg.BindAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
		log.Println("shutting down…")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutCtx)
	}
}

// loadCloudflareToken returns the Cloudflare API token, encrypting a legacy
// plaintext token file to <path>.enc (sealed at rest) on first run. Env
// FDM_CLOUDFLARE_TOKEN takes precedence.
func loadCloudflareToken(cfg config.Config, sealer *crypto.Sealer) string {
	if v := strings.TrimSpace(os.Getenv("FDM_CLOUDFLARE_TOKEN")); v != "" {
		return v
	}
	enc := cfg.CloudflareTokenPath() + ".enc"
	if b, err := os.ReadFile(enc); err == nil {
		if pt, err := sealer.Open(b); err == nil {
			return strings.TrimSpace(string(pt))
		}
	}
	// Migrate legacy plaintext token -> sealed, then remove the plaintext.
	if b, err := os.ReadFile(cfg.CloudflareTokenPath()); err == nil {
		tok := strings.TrimSpace(string(b))
		if sealed, err := sealer.Seal([]byte(tok)); err == nil {
			if os.WriteFile(enc, sealed, 0o600) == nil {
				_ = os.Remove(cfg.CloudflareTokenPath())
				log.Println("cloudflare token: migrated to encrypted-at-rest (data/cloudflare.token.enc)")
			}
		}
		return tok
	}
	return ""
}

// certRefreshLoop periodically re-reads certificate expiry for all deployed
// servers so the dashboard "days remaining" badges stay current.
func certRefreshLoop(ctx context.Context, deployer *remote.Deployer) {
	select {
	case <-time.After(30 * time.Second):
	case <-ctx.Done():
		return
	}
	ticker := time.NewTicker(12 * time.Hour)
	defer ticker.Stop()
	for {
		for _, err := range deployer.RefreshAllCerts() {
			log.Printf("cert refresh: %v", err)
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}
