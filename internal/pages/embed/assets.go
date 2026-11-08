package embed

import "net/http"

// Assets serves /assets/* of the web bundle. This stub answers 404 for every path until the
// bundle is embedded.
func Assets() http.Handler {
	return http.NotFoundHandler()
}
