package main

import (
	"compress/gzip"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// gzipTypes are the media types gzipHandler compresses.
var gzipTypes = map[string]bool{
	"text/html":              true,
	"application/json":       true,
	"text/css":               true,
	"application/javascript": true,
}

// gzipHandler wraps h: it always sets Vary: Accept-Encoding and compresses the response with gzip when the
// client lists gzip, the method is not HEAD, there is no Range header, the response has no Content-Encoding,
// its status is not 204 or 304 and its Content-Type is one of gzipTypes.
func gzipHandler(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Accept-Encoding")
		if r.Method == http.MethodHead || r.Header.Get("Range") != "" || !acceptsGzip(r.Header.Values("Accept-Encoding")) {
			h.ServeHTTP(w, r)
			return
		}
		gw := &gzipWriter{ResponseWriter: w}
		defer gw.close()
		h.ServeHTTP(gw, r)
	})
}

// acceptsGzip reports whether an Accept-Encoding header lists gzip with a quality above zero.
func acceptsGzip(values []string) bool {
	for _, v := range values {
		for _, part := range strings.Split(v, ",") {
			name, params, _ := strings.Cut(strings.TrimSpace(part), ";")
			if !strings.EqualFold(strings.TrimSpace(name), "gzip") {
				continue
			}
			q := 1.0
			for _, p := range strings.Split(params, ";") {
				k, val, ok := strings.Cut(strings.TrimSpace(p), "=")
				if ok && strings.EqualFold(strings.TrimSpace(k), "q") {
					if f, err := strconv.ParseFloat(strings.TrimSpace(val), 64); err == nil {
						q = f
					}
				}
			}
			if q > 0 {
				return true
			}
		}
	}
	return false
}

// gzipWriter decides on the first WriteHeader or Write whether the response is compressed.
type gzipWriter struct {
	http.ResponseWriter
	zw      *gzip.Writer
	decided bool
}

func (g *gzipWriter) decide(status int, first []byte) {
	if g.decided {
		return
	}
	g.decided = true
	hd := g.Header()
	if status == http.StatusNoContent || status == http.StatusNotModified || hd.Get("Content-Encoding") != "" {
		return
	}
	ct := hd.Get("Content-Type")
	if ct == "" && len(first) > 0 {
		ct = http.DetectContentType(first)
	}
	mt, _, err := mime.ParseMediaType(ct)
	if err != nil || !gzipTypes[mt] {
		return
	}
	hd.Set("Content-Encoding", "gzip")
	hd.Del("Content-Length")
	g.zw = gzip.NewWriter(g.ResponseWriter)
}

func (g *gzipWriter) WriteHeader(status int) {
	if !g.decided {
		g.decide(status, nil)
	}
	g.ResponseWriter.WriteHeader(status)
}

func (g *gzipWriter) Write(p []byte) (int, error) {
	if !g.decided {
		g.decide(http.StatusOK, p)
	}
	if g.zw != nil {
		return g.zw.Write(p)
	}
	return g.ResponseWriter.Write(p)
}

// Flush flushes the gzip writer and then the underlying writer, so a streaming route is not held back.
func (g *gzipWriter) Flush() {
	if g.zw != nil {
		_ = g.zw.Flush()
	}
	if f, ok := g.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (g *gzipWriter) Unwrap() http.ResponseWriter { return g.ResponseWriter }

func (g *gzipWriter) close() {
	if g.zw != nil {
		_ = g.zw.Close()
	}
}
