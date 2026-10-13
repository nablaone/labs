package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Versioned assets. JS, CSS and vendor files are served under
// /v/<build hash>/..., and index.html and sw.js are rewritten to point there.
// A page therefore always runs exactly the code it was built with (relative
// module imports stay inside the same version), whatever a browser, the
// service worker or a CDN such as Cloudflare still has cached. Versioned files
// never change, so they are cached for a year; the entry points (/, sw.js)
// are revalidated on every load.

// versionedDirs are the static folders served under /v/<hash>/.
var versionedDirs = []string{"js/", "css/", "vendor/"}

type assets struct {
	hash  string            // build hash: of all static file contents
	index []byte            // index.html with versioned asset URLs
	sw    []byte            // sw.js with versioned shell list and cache name
	etags map[string]string // static path -> quoted content hash
}

func newAssets(fsys fs.FS) *assets {
	a := &assets{etags: hashFiles(fsys)}
	names := make([]string, 0, len(a.etags))
	for n := range a.etags {
		names = append(names, n)
	}
	sort.Strings(names)
	h := sha256.New()
	for _, n := range names {
		h.Write([]byte(n + a.etags[n]))
	}
	a.hash = hex.EncodeToString(h.Sum(nil))[:12]

	prefix := "/v/" + a.hash + "/"
	version := func(b []byte, quote string) []byte {
		for _, d := range versionedDirs {
			b = bytes.ReplaceAll(b, []byte(quote+"/"+d), []byte(quote+prefix+d))
		}
		return b
	}
	if b, err := fs.ReadFile(fsys, "index.html"); err == nil {
		a.index = version(b, `"`)
	}
	if b, err := fs.ReadFile(fsys, "sw.js"); err == nil {
		b = version(b, `'`)
		// The shell cache is named after the build, so a new build replaces it.
		a.sw = bytes.Replace(b, []byte("const VERSION = 'dev';"), []byte("const VERSION = '"+a.hash+"';"), 1)
	}
	return a
}

// serve handles the entry points and /v/<hash>/ paths; it reports false for
// anything else (plain static files such as the manifest and icon).
func (a *assets) serve(w http.ResponseWriter, r *http.Request, files http.Handler) bool {
	p := r.URL.Path
	switch {
	case p == "/" && a.index != nil:
		serveBytes(w, r, "index.html", a.index)
		return true
	case p == "/sw.js" && a.sw != nil:
		serveBytes(w, r, "sw.js", a.sw)
		return true
	case strings.HasPrefix(p, "/v/"):
		rest := strings.TrimPrefix(p, "/v/")
		hash, file, ok := strings.Cut(rest, "/")
		if !ok {
			http.NotFound(w, r)
			return true
		}
		if hash == a.hash {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		} else {
			// An old page asking for an old build: give it the current files,
			// but don't let anyone cache them under the old name.
			w.Header().Set("Cache-Control", "no-cache")
		}
		if tag, ok := a.etags[file]; ok {
			w.Header().Set("ETag", tag)
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + file
		files.ServeHTTP(w, r2)
		return true
	}
	return false
}

func serveBytes(w http.ResponseWriter, r *http.Request, name string, b []byte) {
	sum := sha256.Sum256(b)
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("ETag", `"`+hex.EncodeToString(sum[:8])+`"`)
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(b))
}
