package frontend

import (
	"io/fs"
	"net/http"
	"strings"
)

// NewSPAHandler returns an http.HandlerFunc that serves files from fsys,
// falling back to index.html for any path that doesn't match a static file.
// This lets the Next.js client-side router handle dynamic routes like /grants/:id.
func NewSPAHandler(fsys fs.FS) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")

		// Serve root
		if path == "" {
			serveFile(w, r, fsys, "index.html")
			return
		}

		// Try exact file (e.g. _next/static/chunks/main.js)
		if fileExists(fsys, path) {
			serveFile(w, r, fsys, path)
			return
		}

		// Try directory index (e.g. /backup -> backup/index.html)
		if fileExists(fsys, path+"/index.html") {
			serveFile(w, r, fsys, path+"/index.html")
			return
		}

		// Try with .html extension (e.g. /backup -> backup.html)
		if fileExists(fsys, path+".html") {
			serveFile(w, r, fsys, path+".html")
			return
		}

		// SPA fallback: serve root index.html and let client-side router decide
		serveFile(w, r, fsys, "index.html")
	}
}

// fileExists reports whether path exists in fsys and is a regular file.
func fileExists(fsys fs.FS, path string) bool {
	f, err := fsys.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	stat, err := f.Stat()
	if err != nil {
		return false
	}
	return !stat.IsDir()
}

// serveFile writes the contents of the named file from fsys to the response.
func serveFile(w http.ResponseWriter, r *http.Request, fsys fs.FS, name string) {
	http.ServeFileFS(w, r, fsys, name)
}
