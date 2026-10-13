package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"sitaw/internal/store"
)

// The page must always load the JS it was built with: assets live under
// /v/<build>/, cached forever there, and the entry points are rewritten.
func TestVersionedAssets(t *testing.T) {
	st, err := store.Open(filepath.Join(t.TempDir(), "s.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	static := fstest.MapFS{
		"index.html":           {Data: []byte(`<link href="/css/app.css"><script src="/vendor/l.js"></script><script type="module" src="/js/app.js"></script><link rel="manifest" href="/manifest.webmanifest">`)},
		"js/app.js":            {Data: []byte(`import './sync.js';`)},
		"js/sync.js":           {Data: []byte(`export {}`)},
		"css/app.css":          {Data: []byte(`body{}`)},
		"vendor/l.js":          {Data: []byte(`var L`)},
		"manifest.webmanifest": {Data: []byte(`{}`)},
		"sw.js":                {Data: []byte("const VERSION = 'dev'; // x\nconst SHELL = ['/', '/js/app.js', '/css/app.css'];")},
	}
	ts := httptest.NewServer(New(st, static, "a", "", slog.New(slog.NewTextHandler(io.Discard, nil))).Handler())
	t.Cleanup(ts.Close)
	get := func(p string) (*http.Response, string) {
		r, err := http.Get(ts.URL + p)
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(r.Body)
		return r, string(b)
	}

	_, page := get("/")
	m := regexp.MustCompile(`/v/([0-9a-f]{12})/js/app\.js`).FindStringSubmatch(page)
	if m == nil || !strings.Contains(page, "/v/"+m[1]+"/css/app.css") || !strings.Contains(page, "/v/"+m[1]+"/vendor/l.js") ||
		!strings.Contains(page, `href="/manifest.webmanifest"`) {
		t.Fatalf("index not rewritten: %s", page)
	}
	build := m[1]
	if _, deep := get("/i/6f1c2c1e-0000-4000-8000-000000000001"); deep != page {
		t.Fatal("deep links must get the same rewritten page")
	}

	r, body := get("/v/" + build + "/js/app.js")
	if r.StatusCode != 200 || body != `import './sync.js';` || !strings.Contains(r.Header.Get("Cache-Control"), "immutable") {
		t.Fatalf("versioned file: %d %q %q", r.StatusCode, body, r.Header.Get("Cache-Control"))
	}
	// A relative import from that file stays in the same build.
	if r, _ := get("/v/" + build + "/js/sync.js"); r.StatusCode != 200 {
		t.Fatalf("relative import: %d", r.StatusCode)
	}
	// An old build's URL still works but must not be cached under that name.
	if r, _ := get("/v/000000000000/js/app.js"); r.StatusCode != 200 || r.Header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("old build: %d %q", r.StatusCode, r.Header.Get("Cache-Control"))
	}

	_, sw := get("/sw.js")
	if !strings.Contains(sw, "const VERSION = '"+build+"';") || !strings.Contains(sw, "'/v/"+build+"/js/app.js'") || !strings.Contains(sw, "'/',") {
		t.Fatalf("sw not rewritten: %s", sw)
	}
}
