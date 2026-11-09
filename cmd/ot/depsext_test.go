package main

import (
	"strings"
	"testing"
)

type fakeExtDeps struct{ name string }

func TestDepsOfReturnsTheValueOfTheKey(t *testing.T) {
	t.Parallel()
	d := deps{ext: map[string]any{"fake": fakeExtDeps{name: "x"}}}
	if got := depsOf[fakeExtDeps](d, "fake"); got.name != "x" {
		t.Fatalf("depsOf = %+v, want name x", got)
	}
}

func TestDepsOfPanicsOnAMissingKey(t *testing.T) {
	t.Parallel()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("depsOf of a missing key did not panic")
		}
		if msg, _ := r.(string); !strings.Contains(msg, `"nokey"`) {
			t.Fatalf("panic = %v, want it to name the key", r)
		}
	}()
	depsOf[fakeExtDeps](deps{ext: map[string]any{}}, "nokey")
}

func TestDepsOfPanicsOnAWrongType(t *testing.T) {
	t.Parallel()
	defer func() {
		if recover() == nil {
			t.Fatal("depsOf of a value of another type did not panic")
		}
	}()
	depsOf[fakeExtDeps](deps{ext: map[string]any{"k": 1}}, "k")
}

func TestNewDepsFillsExtFromTheMap(t *testing.T) {
	t.Parallel()
	d := newDeps()
	if d.ext == nil {
		t.Fatal("ext is nil")
	}
	if len(d.ext) != len(extDeps) {
		t.Fatalf("ext has %d entries, extDeps %d", len(d.ext), len(extDeps))
	}
}
