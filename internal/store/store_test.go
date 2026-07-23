package store

import (
	"path/filepath"
	"testing"
)

func testStore(t *testing.T) *Store {
	t.Helper()
	st, err := Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func sampleServer() Server {
	return Server{
		Name: "inut-jsc", Host: "77.88.206.18", SSHPort: 6464, SSHUser: "root",
		AuthKind: "agent", Project: "frps", RemoteBase: "/root/inut-jsc",
		Domain: "io.vnmap3d.com", BindPort: 9000, DashboardPort: 9001,
		VhostHTTPPort: 9002, FpmuPort: 9003, DashUser: "admin", CertEmail: "kspprovn@gmail.com",
	}
}

func TestServerCRUD(t *testing.T) {
	st := testStore(t)

	s, err := st.CreateServer(sampleServer())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.ID == 0 || s.CreatedAt == 0 {
		t.Fatalf("expected id/created_at set, got %+v", s)
	}
	if s.RemotePath() != "/root/inut-jsc/frps" {
		t.Fatalf("RemotePath = %q", s.RemotePath())
	}

	got, err := st.GetServer(s.ID)
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Host != "77.88.206.18" || got.FpmuPort != 9003 || got.Status != "unprovisioned" {
		t.Fatalf("unexpected server: %+v", got)
	}

	got.Status = "ready"
	got.FrpsVersion = "0.54.0"
	if err := st.UpdateServer(got); err != nil {
		t.Fatalf("update: %v", err)
	}
	got2, _ := st.GetServer(s.ID)
	if got2.Status != "ready" || got2.FrpsVersion != "0.54.0" {
		t.Fatalf("update not persisted: %+v", got2)
	}

	list, err := st.ListServers()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}

	if err := st.DeleteServer(s.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := st.GetServer(s.ID); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestUniqueHostProject(t *testing.T) {
	st := testStore(t)
	if _, err := st.CreateServer(sampleServer()); err != nil {
		t.Fatalf("first create: %v", err)
	}
	if _, err := st.CreateServer(sampleServer()); err == nil {
		t.Fatal("expected UNIQUE(host, project) violation on duplicate")
	}
}

func TestMigrationsIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "m.db")
	st1, err := Open(path)
	if err != nil {
		t.Fatalf("open1: %v", err)
	}
	st1.Close()
	// Re-opening applies no new migrations and must not error.
	st2, err := Open(path)
	if err != nil {
		t.Fatalf("open2: %v", err)
	}
	st2.Close()
}

func TestSingleAdminAndFrpUserOwnership(t *testing.T) {
	st := testStore(t)
	admin, err := st.CreateAppUser("root", "hash", RoleAdmin, false)
	if err != nil {
		t.Fatalf("create admin: %v", err)
	}
	if _, err := st.CreateAppUser("second-root", "hash", RoleAdmin, false); err == nil {
		t.Fatal("expected the single-admin index to reject a second admin")
	}
	manager, err := st.CreateAppUser("site-manager", "hash", RoleManager, false)
	if err != nil {
		t.Fatalf("create manager: %v", err)
	}
	s, err := st.CreateServer(sampleServer())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	u, err := st.AddFrpUser(s.ID, "camera", []byte("sealed"), manager.ID)
	if err != nil {
		t.Fatalf("add frp user: %v", err)
	}
	got, err := st.GetFrpUser(u.ID)
	if err != nil || got.CreatedByID != manager.ID || got.CreatorName != manager.Username {
		t.Fatalf("unexpected owner: %+v err=%v", got, err)
	}
	if err := st.DeleteAppUser(manager.ID); err != nil {
		t.Fatalf("delete manager: %v", err)
	}
	got, err = st.GetFrpUser(u.ID)
	if err != nil || got.CreatedByID != 0 || got.CreatorName != "" {
		t.Fatalf("deleted manager should leave an admin-owned FRP user: %+v err=%v", got, err)
	}
	if admin.ID == 0 {
		t.Fatal("expected root admin id")
	}
}

func TestServerNATDefaults(t *testing.T) {
	st := testStore(t)
	s, err := st.CreateServer(sampleServer())
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	got, err := st.GetServer(s.ID)
	if err != nil {
		t.Fatalf("get server: %v", err)
	}
	if got.NatPortStart != 10000 || got.NatPortEnd != 60000 || got.MaxPortsPerClient != 50 {
		t.Fatalf("unexpected NAT defaults: %+v", got)
	}
}
