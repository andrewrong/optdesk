package embed

import (
	"bytes"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

var zeroTime time.Time

const assetsPrefix = "/assets/"

// Assets serves the files of dist/assets for request paths /assets/*. Any other path, a path
// that is not clean and a missing file are 404; a method other than GET and HEAD is 405.
func Assets() http.Handler {
	return newAssets(bundle())
}

func newAssets(fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := r.URL.Path
		if !strings.HasPrefix(p, assetsPrefix) || path.Clean(p) != p {
			http.NotFound(w, r)
			return
		}
		name := strings.TrimPrefix(p, "/")
		data, err := fs.ReadFile(fsys, name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		// ServeContent sets the type from the extension and answers HEAD without a body.
		http.ServeContent(w, r, path.Base(name), zeroTime, bytes.NewReader(data))
	})
}
