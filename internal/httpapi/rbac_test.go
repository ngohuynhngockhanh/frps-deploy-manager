package httpapi

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	appcrypto "github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/web"
)

func testHTTPDeps(t *testing.T) (Deps, *store.Store) {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	renderer, err := web.NewRenderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return Deps{Store: st, Auth: auth.NewManager(st), Render: renderer}, st
}

func createOperator(t *testing.T, st *store.Store, username, password, role string) store.AppUser {
	t.Helper()
	hash, err := appcrypto.HashPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	u, err := st.CreateAppUser(username, hash, role, false)
	if err != nil {
		t.Fatalf("create operator: %v", err)
	}
	return u
}

func authenticatedRequest(t *testing.T, d Deps, method, target, username, password string, form url.Values) *http.Request {
	t.Helper()
	token, csrf, err := d.Auth.Login(username, password, "127.0.0.1")
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	if form == nil {
		form = url.Values{}
	}
	form.Set("_csrf", csrf)
	r := httptest.NewRequest(method, target, strings.NewReader(form.Encode()))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.AddCookie(&http.Cookie{Name: auth.CookieName, Value: token})
	return r
}

func TestAccountCreateAlwaysCreatesManager(t *testing.T) {
	d, st := testHTTPDeps(t)
	createOperator(t, st, "root", "root-password", store.RoleAdmin)
	form := url.Values{"username": {"child"}, "password": {"child-password"}, "role": {store.RoleAdmin}}
	r := authenticatedRequest(t, d, http.MethodPost, "/accounts", "root", "root-password", form)
	w := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	u, err := st.GetAppUserByUsername("child")
	if err != nil || u.Role != store.RoleManager {
		t.Fatalf("crafted role must still create manager: %+v err=%v", u, err)
	}
}

func TestManagerCannotDeleteAnotherOwnersFrpUser(t *testing.T) {
	d, st := testHTTPDeps(t)
	admin := createOperator(t, st, "root", "root-password", store.RoleAdmin)
	manager := createOperator(t, st, "manager", "manager-password", store.RoleManager)
	s, err := st.CreateServer(store.Server{
		Name: "site", Host: "127.0.0.1", SSHPort: 22, SSHUser: "root", AuthKind: "agent",
		Project: "frps", RemoteBase: "/root/frps", BindPort: 9000, DashboardPort: 9001,
		VhostHTTPPort: 9002, FpmuPort: 9003,
	})
	if err != nil {
		t.Fatalf("create server: %v", err)
	}
	if err := st.SetUserServers(manager.ID, []int64{s.ID}); err != nil {
		t.Fatalf("assign server: %v", err)
	}
	u, err := st.AddFrpUser(s.ID, "protected", []byte("sealed"), admin.ID)
	if err != nil {
		t.Fatalf("add frp user: %v", err)
	}
	target := "/servers/" + strconv.FormatInt(s.ID, 10) + "/frp-users/" + strconv.FormatInt(u.ID, 10) + "/delete"
	r := authenticatedRequest(t, d, http.MethodPost, target, "manager", "manager-password", nil)
	w := httptest.NewRecorder()
	NewRouter(d).ServeHTTP(w, r)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", w.Code, w.Body.String())
	}
	if _, err := st.GetFrpUser(u.ID); err != nil {
		t.Fatalf("protected FRP user was deleted: %v", err)
	}
}
