package httpapi

import (
	"net/http"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/auth"
)

// pageData is the common template payload: current user, CSRF token, and the
// page-specific payload in Data.
type pageData struct {
	User    string
	IsAdmin bool
	CSRF    string
	Path    string
	Err     string
	Data    any
}

func (d Deps) page(r *http.Request, data any) pageData {
	return pageData{User: auth.CurrentUser(r), IsAdmin: auth.IsAdmin(r), CSRF: auth.CSRFToken(r), Path: r.URL.Path, Data: data}
}

func (d Deps) loginPage(w http.ResponseWriter, r *http.Request) {
	msg := ""
	if r.URL.Query().Get("err") != "" {
		msg = "Invalid username or password."
	}
	if err := d.Render.Render(w, "login.html", pageData{Err: msg}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (d Deps) loginSubmit(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form", http.StatusBadRequest)
		return
	}
	username := r.PostFormValue("username")
	password := r.PostFormValue("password")

	token, _, err := d.Auth.Login(username, password, auth.ClientIP(r))
	if err != nil {
		http.Redirect(w, r, "/login?err=1", http.StatusSeeOther)
		return
	}
	auth.SetSessionCookie(w, r, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (d Deps) logout(w http.ResponseWriter, r *http.Request) {
	d.Auth.Logout(auth.CookieValue(r))
	auth.ClearSessionCookie(w)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
