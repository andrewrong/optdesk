package pages_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/pages"
	"github.com/andrewrong/optdesk/internal/store"
)

func TestPagesContract(t *testing.T) {
	t.Parallel()
	var build pages.BuildMacroRatesFunc = func(_ context.Context, _ store.SeriesReader, _ contracts.Day, at time.Time) ([]byte, error) {
		return []byte(at.UTC().Format(time.RFC3339)), nil
	}
	var handler pages.NewHandlerFunc = func(vm []byte) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(vm) })
	}
	vm, err := build(context.Background(), nil, "2026-11-04", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handler(vm).ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	body, _ := io.ReadAll(rec.Result().Body)
	if string(body) != "2000-01-01T00:00:00Z" {
		t.Fatalf("body = %q", body)
	}
}
