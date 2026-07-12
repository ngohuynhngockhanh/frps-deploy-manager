package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type cred struct {
	Username string
	Token    string
}

type frpUsersView struct {
	Server  store.Server
	Users   []store.FrpUser
	NewCred *cred  // shown once after add/rotate (plaintext, unrecoverable later)
	SyncErr string // non-fatal remote sync error
	CSRF    string
}

// frpUsersList renders the user panel (GET, loaded via htmx on the detail page).
func (d Deps) frpUsersList(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	d.renderFrpUsers(w, r, s, nil, "")
}

// frpUserAdd creates a user, syncs the remote tokens file and restarts fpmu.
func (d Deps) frpUserAdd(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	username := strings.TrimSpace(r.PostFormValue("username"))
	token := strings.TrimSpace(r.PostFormValue("token"))
	if username == "" {
		d.renderFrpUsers(w, r, s, nil, "Username is required.")
		return
	}
	if token == "" {
		token = randToken()
	}
	blob, err := d.Sealer.Seal([]byte(token))
	if err != nil {
		d.renderFrpUsers(w, r, s, nil, err.Error())
		return
	}
	if _, err := d.Store.AddFrpUser(s.ID, username, blob); err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE") {
			msg = "A user with that name already exists on this server."
		}
		d.renderFrpUsers(w, r, s, nil, msg)
		return
	}
	syncErr := d.syncUsers(s)
	d.renderFrpUsers(w, r, s, &cred{Username: username, Token: token}, syncErr)
}

// frpUserRotate replaces a user's token.
func (d Deps) frpUserRotate(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	u, ok := d.lookupFrpUser(w, r, s)
	if !ok {
		return
	}
	token := randToken()
	blob, err := d.Sealer.Seal([]byte(token))
	if err != nil {
		d.renderFrpUsers(w, r, s, nil, err.Error())
		return
	}
	if err := d.Store.UpdateFrpToken(u.ID, blob); err != nil {
		d.renderFrpUsers(w, r, s, nil, err.Error())
		return
	}
	syncErr := d.syncUsers(s)
	d.renderFrpUsers(w, r, s, &cred{Username: u.Username, Token: token}, syncErr)
}

// frpUserDelete removes a user.
func (d Deps) frpUserDelete(w http.ResponseWriter, r *http.Request) {
	s, ok := d.lookupServer(w, r)
	if !ok {
		return
	}
	u, ok := d.lookupFrpUser(w, r, s)
	if !ok {
		return
	}
	if err := d.Store.DeleteFrpUser(u.ID); err != nil {
		d.renderFrpUsers(w, r, s, nil, err.Error())
		return
	}
	syncErr := d.syncUsers(s)
	d.renderFrpUsers(w, r, s, nil, syncErr)
}

// --- helpers ---

// syncUsers pushes tokens to the remote only when the server is deployed.
// Returns a user-facing message on failure (non-fatal — DB is source of truth).
func (d Deps) syncUsers(s store.Server) string {
	if s.Status != "ready" {
		return "Saved. Server not deployed yet — will apply on next deploy."
	}
	if err := d.Deployer.SyncFrpUsers(s); err != nil {
		return "Saved to DB, but remote sync failed: " + err.Error()
	}
	return ""
}

func (d Deps) renderFrpUsers(w http.ResponseWriter, r *http.Request, s store.Server, nc *cred, syncErr string) {
	users, err := d.Store.ListFrpUsers(s.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	view := frpUsersView{Server: s, Users: users, NewCred: nc, SyncErr: syncErr, CSRF: auth.CSRFToken(r)}
	if err := d.Render.RenderPartial(w, "_frpusers.html", view); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d Deps) lookupFrpUser(w http.ResponseWriter, r *http.Request, s store.Server) (store.FrpUser, bool) {
	uid, err := strconv.ParseInt(r.PathValue("uid"), 10, 64)
	if err != nil {
		http.NotFound(w, r)
		return store.FrpUser{}, false
	}
	u, err := d.Store.GetFrpUser(uid)
	if err != nil || u.ServerID != s.ID {
		http.NotFound(w, r)
		return store.FrpUser{}, false
	}
	return u, true
}

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
