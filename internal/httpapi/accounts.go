package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/crypto"
	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

type accountRow struct {
	store.AppUser
	ServerIDs map[int64]bool
}

type accountsView struct {
	Accounts []accountRow
	Servers  []store.Server // all sites, for assignment checkboxes
	Err      string
	Msg      string
}

func (d Deps) accountsPage(w http.ResponseWriter, r *http.Request) {
	d.renderAccounts(w, r, "", "")
}

func (d Deps) accountCreate(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	username := strings.TrimSpace(r.PostFormValue("username"))
	password := r.PostFormValue("password")
	role := r.PostFormValue("role")
	if role != store.RoleAdmin {
		role = store.RoleManager
	}
	if username == "" || password == "" {
		d.renderAccounts(w, r, "Username and password are required.", "")
		return
	}
	hash, err := crypto.HashPassword(password)
	if err != nil {
		d.renderAccounts(w, r, err.Error(), "")
		return
	}
	u, err := d.Store.CreateAppUser(username, hash, role)
	if err != nil {
		msg := err.Error()
		if strings.Contains(msg, "UNIQUE") {
			msg = "That username already exists."
		}
		d.renderAccounts(w, r, msg, "")
		return
	}
	if role == store.RoleManager {
		_ = d.Store.SetUserServers(u.ID, formServerIDs(r))
	}
	d.renderAccounts(w, r, "", "Created account "+username+".")
}

func (d Deps) accountDelete(w http.ResponseWriter, r *http.Request) {
	id := parseInt64(r.PathValue("id"))
	// Don't allow deleting yourself.
	if id == auth.CurrentUserID(r) {
		d.renderAccounts(w, r, "You cannot delete your own account.", "")
		return
	}
	_ = d.Store.DeleteAppUser(id)
	d.renderAccounts(w, r, "", "Account deleted.")
}

func (d Deps) accountPassword(w http.ResponseWriter, r *http.Request) {
	id := parseInt64(r.PathValue("id"))
	pw := r.PostFormValue("password")
	if pw == "" {
		d.renderAccounts(w, r, "Password cannot be empty.", "")
		return
	}
	hash, err := crypto.HashPassword(pw)
	if err != nil {
		d.renderAccounts(w, r, err.Error(), "")
		return
	}
	_ = d.Store.SetAppUserPassword(id, hash)
	d.renderAccounts(w, r, "", "Password updated.")
}

func (d Deps) accountServers(w http.ResponseWriter, r *http.Request) {
	_ = r.ParseForm()
	id := parseInt64(r.PathValue("id"))
	_ = d.Store.SetUserServers(id, formServerIDs(r))
	d.renderAccounts(w, r, "", "Site access updated.")
}

func (d Deps) renderAccounts(w http.ResponseWriter, r *http.Request, errMsg, okMsg string) {
	users, err := d.Store.ListAppUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	servers, _ := d.Store.ListServers()
	rows := make([]accountRow, 0, len(users))
	for _, u := range users {
		ids, _ := d.Store.UserServerIDs(u.ID)
		rows = append(rows, accountRow{AppUser: u, ServerIDs: ids})
	}
	view := accountsView{Accounts: rows, Servers: servers, Err: errMsg, Msg: okMsg}
	if err := d.Render.Render(w, "accounts.html", d.page(r, view)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func formServerIDs(r *http.Request) []int64 {
	var ids []int64
	for _, v := range r.PostForm["servers"] {
		if id := parseInt64(v); id != 0 {
			ids = append(ids, id)
		}
	}
	return ids
}

func parseInt64(s string) int64 {
	n, _ := strconv.ParseInt(s, 10, 64)
	return n
}
