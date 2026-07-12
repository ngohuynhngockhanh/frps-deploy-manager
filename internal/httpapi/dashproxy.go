package httpapi

import (
	"encoding/base64"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"

	"github.com/ngohuynhngockhanh/frps-deploy-manager/internal/store"
)

// dashProxy reverse-proxies a server's frps dashboard under the /dash/ path of
// the admin UI (same origin, so it shares the session — no separate port, and
// no separate login to frps: Basic auth is injected from the stored dashboard
// creds). The frp dashboard SPA loads its assets relatively (served under
// /dash/static/) and calls its API at the absolute /api/* — which the admin UI
// does not otherwise use, so /api/* is proxied through here too.
//
// It handles both the /dash/* and /api/* routes. Session + CSRF are already
// enforced by the auth middleware wrapping the protected mux.
func (d Deps) dashProxy(w http.ResponseWriter, r *http.Request) {
	// First hit: /dash/?server=<id> selects the target and remembers it, so the
	// SPA's later absolute /api/* calls know which dashboard to reach.
	if sid := r.URL.Query().Get("server"); sid != "" {
		http.SetCookie(w, &http.Cookie{Name: "fdm_dash_srv", Value: sid, Path: "/", HttpOnly: true, SameSite: http.SameSiteStrictMode})
		http.Redirect(w, r, "/dash/", http.StatusSeeOther)
		return
	}
	sidStr := ""
	if c, err := r.Cookie("fdm_dash_srv"); err == nil {
		sidStr = c.Value
	}
	sid, err := strconv.ParseInt(sidStr, 10, 64)
	if err != nil {
		http.Error(w, "No dashboard selected — open it from a server's page.", http.StatusBadRequest)
		return
	}
	s, err := d.Store.GetServer(sid)
	if err != nil || s.Domain == "" {
		http.Error(w, "Server not found or has no domain.", http.StatusNotFound)
		return
	}
	dashboardProxy(s).ServeHTTP(w, r)
}

// dashboardProxy proxies to the server's frps dashboard over its HTTPS domain,
// injecting Basic auth. It strips the /dash prefix for dashboard paths (leaving
// /api/* untouched) and rewrites redirect Locations back under /dash.
func dashboardProxy(s store.Server) *httputil.ReverseProxy {
	target, _ := url.Parse("https://" + s.Domain)
	cred := base64.StdEncoding.EncodeToString([]byte(s.DashUser + ":" + s.DashPassword))

	return &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			req.Header.Set("Authorization", "Basic "+cred)
			// /dash/static/x -> /static/x ; /api/... stays /api/...
			req.URL.Path = strings.TrimPrefix(req.URL.Path, "/dash")
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			loc := resp.Header.Get("Location")
			if loc == "" {
				return nil
			}
			u, err := url.Parse(loc)
			if err != nil {
				return nil
			}
			// Make target-absolute redirects relative, then keep dashboard paths
			// under /dash (leave /api and already-/dash paths alone).
			if u.Host == target.Host {
				u.Scheme, u.Host = "", ""
			}
			if u.Host == "" && strings.HasPrefix(u.Path, "/") &&
				!strings.HasPrefix(u.Path, "/dash") && !strings.HasPrefix(u.Path, "/api") {
				u.Path = "/dash" + u.Path
			}
			resp.Header.Set("Location", u.String())
			return nil
		},
	}
}
