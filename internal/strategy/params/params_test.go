package params_test

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/strategy/params"
)

func TestParamsHashIsOrderFree(t *testing.T) {
	t.Parallel()
	// SHA-1 of "a=1\nb=2" written out from an independent tool.
	const want = "009d8b2343a806c6208f48c4aeead21e8388d3de"
	a := map[string]string{"a": "1", "b": "2"}
	b := map[string]string{}
	b["b"] = "2"
	b["a"] = "1"
	if params.Hash(a) != params.Hash(b) {
		t.Fatalf("order matters: %s vs %s", params.Hash(a), params.Hash(b))
	}
	if got := params.Hash(a); got != want {
		t.Fatalf("hash = %s, want %s", got, want)
	}
	if params.Hash(a) == params.Hash(map[string]string{"a": "1", "b": "3"}) {
		t.Fatal("a changed value gave the same hash")
	}
}
