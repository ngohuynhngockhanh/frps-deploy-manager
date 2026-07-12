package httpapi

import (
	"encoding/base64"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// DashProxyHandler reverse-proxies a server's frps dashboard, injecting HTTP
// Basic auth so the operator views it without logging in over there. It runs on
// a separate port but shares the admin session cookie (cookies are not
// port-scoped), so it still requires a logged-in operator.
func DashProxyHandler(d Deps) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if d.Auth.SessionUser(r) == "" {
			http.Error(w, "Not logged in. Open the admin UI (:2050), sign in, then use the dashboard button.", http.StatusUnauthorized)
			return
		}

		// Select the target server via ?server=<id> (first hit), then remember
		// it in a cookie so the SPA's absolute /api and /static paths keep
		// hitting the right dashboard.
		if sid := r.URL.Query().Get("server"); sid != "" {
			http.SetCookie(w, &http.Cookie{Name: "fdm_dash_srv", Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		sidStr := ""
		if c, err := r.Cookie("fdm_dash_srv"); err == nil {
			sidStr = c.Value
		}
		sid, err := strconv.ParseInt(sidStr, 10, 64)
		if err != nil {
			http.Error(w, "No server selected — open via the 'frps dashboard' button.", http.StatusBadRequest)
			return
		}
		s, err := d.Store.GetServer(sid)
		if err != nil || s.Domain == "" {
			http.Error(w, "Server not found or has no domain.", http.StatusNotFound)
			return
		}

		proxyToDashboard(s).ServeHTTP(w, r)
	})
}

// proxyToDashboard builds a reverse proxy to the server's frps dashboard (via
// its HTTPS domain), injecting Basic auth and keeping redirects on this host.
func proxyToDashboard(s store.Server) *httputil.ReverseProxy {
	target, _ := url.Parse("https://" + s.Domain)
	cred := base64.StdEncoding.EncodeToString([]byte(s.DashUser + ":" + s.DashPassword))

	rp := httputil.NewSingleHostReverseProxy(target)
	base := rp.Director
	rp.Director = func(req *http.Request) {
		base(req)
		req.Host = target.Host
		req.Header.Set("Authorization", "Basic "+cred)
	}
	rp.ModifyResponse = func(resp *http.Response) error {
		// Rewrite absolute redirects back onto this proxy host.
		if loc := resp.Header.Get("Location"); loc != "" {
			if u, err := url.Parse(loc); err == nil && (u.Host == target.Host || u.Host == "") {
				u.Scheme, u.Host = "", ""
				resp.Header.Set("Location", u.String())
			}
		}
		return nil
	}
	return rp
}
