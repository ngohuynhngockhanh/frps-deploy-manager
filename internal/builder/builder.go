// Package builder cross-compiles the frps and fp-multiuser binaries
// (linux/amd64, static) from local source, caching outputs by content so a
// re-deploy skips rebuilding unchanged sources.
package builder

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Builder holds paths needed to compile the remote binaries.
type Builder struct {
	GoBin      string // path to the go toolchain
	FRPSrcDir  string // frp source module (has ./cmd/frps)
	FpmuSrcDir string // fp-multiuser module (has ./cmd/fp-multiuser)
	CacheDir   string // where built binaries are cached
}

// Artifact is a built binary and its reported version.
type Artifact struct {
	Path    string
	Version string
}

// BuildFrps cross-compiles frps.
func (b *Builder) BuildFrps(force bool) (Artifact, error) {
	return b.build("frps", b.FRPSrcDir, "./cmd/frps", force)
}

// BuildFpMultiuser cross-compiles fp-multiuser.
func (b *Builder) BuildFpMultiuser(force bool) (Artifact, error) {
	return b.build("fp-multiuser", b.FpmuSrcDir, "./cmd/fp-multiuser", force)
}

// build compiles pkg within moduleDir to CacheDir/<name>-<srchash>, reusing a
// cached binary when the source fingerprint is unchanged.
func (b *Builder) build(name, moduleDir, pkg string, force bool) (Artifact, error) {
	abs, err := filepath.Abs(moduleDir)
	if err != nil {
		return Artifact{}, err
	}
	if fi, err := os.Stat(abs); err != nil || !fi.IsDir() {
		return Artifact{}, fmt.Errorf("source dir not found: %s", abs)
	}

	hash, err := fingerprintSources(abs)
	if err != nil {
		return Artifact{}, fmt.Errorf("fingerprint %s: %w", name, err)
	}
	if err := os.MkdirAll(b.CacheDir, 0o755); err != nil {
		return Artifact{}, err
	}
	out := filepath.Join(b.CacheDir, fmt.Sprintf("%s-%s", name, hash[:12]))

	if !force {
		if _, err := os.Stat(out); err == nil {
			return Artifact{Path: out, Version: b.versionOf(out)}, nil
		}
	}

	tmp := out + ".building"
	cmd := exec.Command(b.GoBin, "build", "-trimpath", "-o", tmp, pkg)
	cmd.Dir = abs
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0", "GOOS=linux", "GOARCH=amd64",
		"GODEBUG=netdns=cgo", // host's pure-Go resolver times out; cgo for module fetch
	)
	var errb bytes.Buffer
	cmd.Stderr = &errb
	if err := cmd.Run(); err != nil {
		_ = os.Remove(tmp)
		return Artifact{}, fmt.Errorf("build %s failed: %w: %s", name, err, strings.TrimSpace(errb.String()))
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return Artifact{}, err
	}
	if err := os.Rename(tmp, out); err != nil {
		return Artifact{}, err
	}
	return Artifact{Path: out, Version: b.versionOf(out)}, nil
}

// versionOf runs the (linux/amd64) binary with --version. The manager host is
// linux/amd64, so this executes locally; failures return "" (non-fatal).
func (b *Builder) versionOf(path string) string {
	out, err := exec.Command(path, "--version").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// fingerprintSources hashes all *.go files, go.mod and go.sum under dir so any
// source change invalidates the cache. Vendored module caches are skipped.
func fingerprintSources(dir string) (string, error) {
	h := sha256.New()
	err := filepath.WalkDir(dir, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if de.IsDir() {
			base := de.Name()
			if base == ".git" || base == "vendor" || base == "bin" {
				return filepath.SkipDir
			}
			return nil
		}
		name := de.Name()
		if !strings.HasSuffix(name, ".go") && name != "go.mod" && name != "go.sum" {
			return nil
		}
		rel, _ := filepath.Rel(dir, path)
		fmt.Fprintf(h, "%s\n", rel)
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		h.Write(data)
		return nil
	})
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", h.Sum(nil)), nil
}
