// Package remote composes the low-level ssh primitives into high-level
// operations against a managed server: connecting, gathering facts, and (in
// later milestones) deploying frps + fp-multiuser, nginx and certbot.
package remote

import (
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// SSHConfig builds an ssh.Config from a stored server record.
func SSHConfig(s store.Server) ssh.Config {
	return ssh.Config{
		Host:          s.Host,
		Port:          s.SSHPort,
		User:          s.SSHUser,
		AuthKind:      s.AuthKind,
		KeyRef:        s.KeyRef,
		PinnedHostKey: s.HostKey,
	}
}

// InterestingPorts returns the frps port set to check for conflicts.
func InterestingPorts(s store.Server) []int {
	return []int{s.BindPort, s.DashboardPort, s.VhostHTTPPort, s.FpmuPort}
}

// PortRole labels a port with its frps role for display.
func PortRole(s store.Server, port int) string {
	switch port {
	case s.BindPort:
		return "bindPort (frpc control)"
	case s.DashboardPort:
		return "dashboard"
	case s.VhostHTTPPort:
		return "vhostHTTP"
	case s.FpmuPort:
		return "fp-multiuser"
	default:
		return "unknown"
	}
}
