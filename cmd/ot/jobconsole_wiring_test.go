package main

import "testing"

func TestOpenersRegisteredInExtDeps(t *testing.T) {
	t.Parallel()
	o, ok := newExt()[jobConsoleExtKey].(jobRequestOpeners)
	if !ok {
		t.Fatalf("ext deps hold no jobRequestOpeners under %q", jobConsoleExtKey)
	}
	if o.Reader == nil || o.Poster == nil || o.Consumer == nil {
		t.Fatalf("an opener is nil: %+v", o)
	}
	if _, ok := jobRequestOpenersOf(newDeps()); !ok {
		t.Fatal("newDeps carries no openers")
	}
	if _, ok := jobRequestOpenersOf(deps{}); ok {
		t.Fatal("a deps literal with no ext reports openers")
	}
}
