package main

import "fmt"

// extDeps is the deps source of each command: a command's file declares
// `type <name>Deps struct{...}` and in init puts a constructor keyed by the command name,
//
//	extDeps["accountimport"] = func() any { return accountImportDeps{...} }
//
// so a new command adds no field to deps and no line to newDeps. newDeps calls each
// constructor once; a test sets deps.ext to fakes.
var extDeps = map[string]func() any{}

// newExt returns the values of every constructor of extDeps, keyed by command name.
func newExt() map[string]any {
	ext := make(map[string]any, len(extDeps))
	for name, build := range extDeps {
		ext[name] = build()
	}
	return ext
}

// depsOf returns the deps of command name as T. A missing key or another type panics here,
// the one place that asserts the type; a library may replace it later (own implementation:
// the glue is a map and a generic function, and no DI library fits).
func depsOf[T any](d deps, name string) T {
	v, ok := d.ext[name]
	if !ok {
		panic(fmt.Sprintf("deps: no ext deps for command %q", name))
	}
	return v.(T)
}
