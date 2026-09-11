package server

import (
	"fmt"
	"net/http"

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
	Jobs *jobs.Manager
}

// NewApp constructs an application around a job manager. A default manager is
// created when manager is nil.
func NewApp(manager *jobs.Manager) *App {
	if manager == nil {
		manager = jobs.New(jobs.Options{})
	}
	return &App{Jobs: manager}
}

// Handler returns the application and versioned API routes.
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	assets := http.FileServer(http.FS(render.AssetFS()))
	mux.Handle("/assets/", http.StripPrefix("/", assets))
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
	return mux
}
