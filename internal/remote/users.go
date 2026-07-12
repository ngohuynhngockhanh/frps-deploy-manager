package remote

import (
	"fmt"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/ssh"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// SyncFrpUsers re-renders the remote tokens.txt from the DB and restarts the
// fp-multiuser plugin so new credentials take effect. Existing tunnels are
// unaffected — the plugin is only consulted at frpc Login.
func (d *Deployer) SyncFrpUsers(s store.Server) error {
	client, err := ssh.Dial(SSHConfig(s))
	if err != nil {
		return err
	}
	defer client.Close()

	tokens, err := d.renderTokens(s.ID)
	if err != nil {
		return err
	}
	if err := client.WriteFile(s.RemotePath()+"/conf/tokens.txt", tokens, 0o600); err != nil {
		return fmt.Errorf("write tokens: %w", err)
	}
	if _, err := client.MustRun(fmt.Sprintf("systemctl restart fp-multiuser-%s", s.Project)); err != nil {
		return fmt.Errorf("restart fp-multiuser: %w", err)
	}
	return nil
}
