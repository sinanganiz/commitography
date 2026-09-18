package server

import (
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/sinanganiz/commitography/internal/core"
	"github.com/sinanganiz/commitography/internal/pipeline/collect"
	"github.com/sinanganiz/commitography/internal/pipeline/render"
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

// App is the local HTTP application and its in-memory job manager.
type App struct {
	Jobs         *Manager
	sessionToken string

	// collector validates a requested repository before a job is created.
	collector *collect.Collector

	// allowedRoots holds the resolved form of each root, used for every
	// containment check. suppliedRoots holds the same roots in the form the
	// operator gave them, and is the only form a message may name (ADR-0067
	// clause 5). The two are parallel slices of equal length.
	allowedRoots  []string
	suppliedRoots []string

	allowedHosts map[string]bool
}

// NewApp constructs the application around its job manager, the collect stage
// that validates requested repositories, and the random source its session
// secret is drawn from, all constructed by the caller (ADR-0042 clause 1).
// roots are the allowed repository roots as the operator supplied them; none
// means the working directory.
func NewApp(manager *Manager, collector *collect.Collector, random core.Random, roots []string) (*App, error) {
	allowedRoots, suppliedRoots, err := canonicalRoots(roots)
	if err != nil {
		return nil, err
	}
	token, err := newSessionToken(random)
	if err != nil {
		return nil, err
	}
	return &App{
		Jobs:          manager,
		sessionToken:  token,
		collector:     collector,
		allowedRoots:  allowedRoots,
		suppliedRoots: suppliedRoots,
		allowedHosts:  loopbackHosts(),
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
// be issued to it. The document route answers in plain text, so it takes the
// status and the message from the same condition the API route reports rather
// than restating either.
func rejectHost(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeAPIError(w, refuse(conditionInvalidHost))
		return
	}
	status, _, message := conditionInvalidHost.response()
	http.Error(w, message, status)
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
			status, _, message := conditionMethodNotAllowed.response()
			http.Error(w, message, status)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if r.Method == http.MethodHead {
			return
		}
		_, _ = fmt.Fprint(w, indexShell)
	})
	// The recovery layer is outermost, so every route is behind it, including
	// the host and session checks above the mux (ADR-0041 clause 6).
	return withRecovery(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		applySecurityHeaders(w)
		if !a.hostAllowed(r.Host) {
			rejectHost(w, r)
			return
		}
		if !a.authorize(w, r) {
			return
		}
		mux.ServeHTTP(w, r)
	}))
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

func newSessionToken(random core.Random) (string, error) {
	data := make([]byte, 32)
	if _, err := io.ReadFull(random, data); err != nil {
		return "", core.Internalf(err, "generating the local session secret")
	}
	return hex.EncodeToString(data), nil
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
			writeAPIError(w, refuse(conditionInvalidSession))
			return false
		} else {
			http.SetCookie(w, a.sessionCookie())
		}
	}
	if r.Method == http.MethodPost || r.Method == http.MethodDelete {
		if !sameOrigin(r) {
			writeAPIError(w, refuse(conditionInvalidOrigin))
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
