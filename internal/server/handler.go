package server

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/sinanganiz/commitography/internal/jobs"
	"github.com/sinanganiz/commitography/internal/render"
)

const indexShell = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<meta name="robots" content="noindex">
<title>Commitography</title>
<link rel="stylesheet" href="/assets/app.css">
</head>
<body>
<div id="commitography-root"></div>
<script type="application/json" id="commitography-data"></script>
<script src="/assets/app.js"></script>
</body>
</html>
`

// NewHandler serves the embedded local application shell and assets. API paths
// are reserved for the job server and return 404 until their handlers are
// registered by the later server milestones.
func NewHandler() http.Handler {
	return NewApp(nil).Handler()
}

// App is the local HTTP application and its in-memory job manager.
type App struct {
	Jobs         *jobs.Manager
	sessionToken string
	allowedRoots []string
	allowedHosts map[string]bool
}

// NewApp constructs an application around a job manager. A default manager is
// created when manager is nil.
func NewApp(manager *jobs.Manager) *App {
	app, err := NewAppWithAllowedRoots(manager, nil)
	if err != nil {
		panic(err)
	}
	return app
}

// NewAppWithAllowedRoots constructs an application with canonical filesystem
// roots used to validate repository requests.
func NewAppWithAllowedRoots(manager *jobs.Manager, roots []string) (*App, error) {
	if manager == nil {
		manager = jobs.New(jobs.Options{})
	}
	allowedRoots, err := canonicalRoots(roots)
	if err != nil {
		return nil, err
	}
	return &App{
		Jobs:         manager,
		sessionToken: newSessionToken(),
		allowedRoots: allowedRoots,
		allowedHosts: loopbackHosts(),
	}, nil
}

// loopbackHosts are the names a browser uses for this machine. A request that
// names any other host is refused, so a page that rebinds its own DNS name to
// 127.0.0.1 cannot obtain a session or reach the API. Inside a container the
// server listens on every interface, and this rule still holds there.
func loopbackHosts() map[string]bool {
	return map[string]bool{"localhost": true, "127.0.0.1": true, "::1": true}
}

// AllowListenHost also accepts the host of an explicit listen address, such as
// an interface address chosen with --listen. A wildcard address adds nothing:
// it names no host, and the loopback names already cover this machine.
func (a *App) AllowListenHost(address string) {
	host, _, err := net.SplitHostPort(address)
	if err != nil || host == "" {
		return
	}
	if ip := net.ParseIP(host); ip != nil && ip.IsUnspecified() {
		return
	}
	a.allowedHosts[normalizeHost(host)] = true
}

// hostAllowed compares only the host name. The port is ignored because a
// published container port may be remapped on the host.
func (a *App) hostAllowed(hostport string) bool {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	host = normalizeHost(strings.TrimSuffix(strings.TrimPrefix(host, "["), "]"))
	return host != "" && a.allowedHosts[host]
}

func normalizeHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(host), ".")
}

// rejectHost refuses a request for another host before a session cookie can
// be issued to it.
func rejectHost(w http.ResponseWriter, r *http.Request) {
	const message = "request host is not allowed; open the dashboard at http://127.0.0.1 or http://localhost"
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeAPIError(w, http.StatusForbidden, "invalid_host", message)
		return
	}
	http.Error(w, message, http.StatusForbidden)
}

// Handler returns the application and versioned API routes.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	assets := http.FileServer(http.FS(render.AssetFS()))
	mux.Handle("/assets/", http.StripPrefix("/", filesOnly(assets)))
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", a.apiHandler()))
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = fmt.Fprint(w, indexShell)
	})
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applySecurityHeaders(w)
		if !a.hostAllowed(r.Host) {
			rejectHost(w, r)
			return
		}
		if !a.authorize(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	})
}

// filesOnly refuses directory paths. The embedded assets can be fetched by
// name, but http.FileServer would otherwise list the directory, and the
// security contract allows no directory listing.
func filesOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/") {
			http.NotFound(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func applySecurityHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
	w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
}

const sessionCookieName = "commitography_session"

func newSessionToken() string {
	data := make([]byte, 32)
	if _, err := rand.Read(data); err != nil {
		panic(fmt.Sprintf("generating local session secret: %v", err))
	}
	return hex.EncodeToString(data)
}

func (a *App) sessionCookie() *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    a.sessionToken,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	}
}

func (a *App) authorize(w http.ResponseWriter, r *http.Request) bool {
	cookie, err := r.Cookie(sessionCookieName)
	valid := err == nil && cookie.Value == a.sessionToken
	if !valid {
		if r.URL.Path == "/api/v1/capabilities" && r.Method == http.MethodGet {
			http.SetCookie(w, a.sessionCookie())
		} else if strings.HasPrefix(r.URL.Path, "/api/") {
			writeAPIError(w, http.StatusUnauthorized, "invalid_session", "a valid local session is required")
			return false
		} else {
			http.SetCookie(w, a.sessionCookie())
		}
	}
	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		if !sameOrigin(r) {
			writeAPIError(w, http.StatusForbidden, "invalid_origin", "request origin is not allowed")
			return false
		}
	}
	return true
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	return parsed.Host == r.Host
}
