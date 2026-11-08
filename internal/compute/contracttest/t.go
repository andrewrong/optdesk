package contracttest

import "testing"

// T is the seam of the suites: testing.TB plus the two calls of a subtest tree. A caller
// passes Wrap(t); a test of a suite passes a recording fake.
type T interface {
	testing.TB
	// Run runs f as a subtest named name and reports whether it passed.
	Run(name string, f func(T)) bool
	// Parallel marks the test as one that runs beside its siblings.
	Parallel()
}

// Wrap adapts a *testing.T to T.
func Wrap(t *testing.T) T { return wrapped{t} }

type wrapped struct{ *testing.T }

// Run runs f as a subtest of the wrapped test, with the subtest's own *testing.T wrapped.
func (w wrapped) Run(name string, f func(T)) bool {
	return w.T.Run(name, func(sub *testing.T) { f(wrapped{sub}) })
}

// load reads one embedded fixture file; a file that does not load fails the suite.
func load(t T, name string) File {
	t.Helper()
	f, err := Load(name)
	if err != nil {
		t.Fatalf("load %s: %v", name, err)
	}
	return f
}

// runCases runs one parallel subtest per case of f whose group has a handler. It fails the
// suite when no case ran, so a handler table that matches nothing cannot pass.
func runCases(t T, f File, handlers map[string]func(T, Case)) {
	t.Helper()
	ran := 0
	for _, c := range f.Cases {
		h, ok := handlers[c.Group]
		if !ok {
			continue
		}
		ran++
		t.Run(c.Name, func(t T) {
			t.Parallel()
			h(t, c)
		})
	}
	if ran == 0 {
		t.Fatalf("%s: no case ran", f.Name)
	}
}
