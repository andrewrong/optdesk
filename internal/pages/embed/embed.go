// Package embed serves the built web bundle: the page with the view model injected at serve
// time and the files under /assets/. The bundle is written to dist by `pnpm --dir web build`
// and is not committed; dist/.gitignore keeps the embed pattern non-empty.
package embed

import (
	"embed"
	"io/fs"
	"net/http"
)

//go:embed all:dist
var distFS embed.FS

// noBundleText is the body of the 500 answered when dist/index.html is absent.
const noBundleText = "web bundle not built: run pnpm --dir web build"

// bundle is the content of dist; the directory is embedded by the pattern above, so Sub cannot fail.
func bundle() fs.FS {
	sub, _ := fs.Sub(distFS, "dist")
	return sub
}

// Handler serves dist/index.html with viewModel in its vm element; the page makes no request.
// A viewModel that is not valid JSON is answered with 500, and so is a missing index.html.
func Handler(viewModel []byte) http.Handler {
	return newHandler(bundle(), viewModel)
}

func newHandler(fsys fs.FS, viewModel []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		index, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			http.Error(w, noBundleText, http.StatusInternalServerError)
			return
		}
		out, err := Render(index, viewModel)
		if err != nil {
			http.Error(w, "view model unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out)
	})
}
