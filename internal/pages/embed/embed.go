// Package embed serves the static page with the view model injected at serve time.
package embed

import (
	_ "embed" // go:embed of the built page
	"net/http"
)

//go:embed dist/index.html
var indexHTML []byte

// Handler serves the page with viewModel in its vm element; the page makes no request.
// A viewModel that is not valid JSON is answered with 500.
func Handler(viewModel []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		out, err := Render(indexHTML, viewModel)
		if err != nil {
			http.Error(w, "view model unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(out)
	})
}
