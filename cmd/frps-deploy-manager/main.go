// Command frps-deploy-manager is a local admin tool to deploy and manage frps
// (frp server) instances on remote VPS hosts over SSH, from the operator's
// workstation. It listens only on loopback (127.0.0.1:2050) behind a login.
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is stamped at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	var (
		showVersion bool
		addr        string
	)
	flag.BoolVar(&showVersion, "version", false, "print version and exit")
	flag.StringVar(&addr, "addr", "127.0.0.1:2050", "local bind address (loopback only)")
	flag.Parse()

	if showVersion {
		fmt.Printf("frps-deploy-manager %s\n", version)
		return
	}

	if err := run(addr); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

// run is the real entrypoint; fleshed out in later milestones (HTTP server,
// DB, auth). For M0 it is a placeholder so the binary links and prints version.
func run(addr string) error {
	fmt.Printf("frps-deploy-manager %s — not yet serving (addr=%s)\n", version, addr)
	return nil
}
