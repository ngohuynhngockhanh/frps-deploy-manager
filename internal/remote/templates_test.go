package remote

import (
	"strings"
	"testing"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

func TestFrpsTemplateIncludesNATSafety(t *testing.T) {
	b, err := render("frps.ini.tmpl", store.Server{
		BindPort: 9000, DashboardPort: 9001, VhostHTTPPort: 9002, FpmuPort: 9003,
		NatPortStart: 10000, NatPortEnd: 60000, MaxPortsPerClient: 50,
	})
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	config := string(b)
	for _, want := range []string{"allow_ports = 10000-60000", "max_ports_per_client = 50"} {
		if !strings.Contains(config, want) {
			t.Fatalf("config missing %q:\n%s", want, config)
		}
	}
}
