// Package contracttest holds the golden fixtures of the compute core and their loader.
//
// A fixture file is a JSON object {"provenance": {...}, "cases": [...]}. "provenance" is
// present only in a generated file. A case has the five keys name, group, input, expected
// and tolerance; a case of a hand file also has a working string that shows the arithmetic.
// The fixtures are embedded, so no caller depends on its working directory.
//
// A ramp spec {"ramp": {"from": 1, "step": 1, "count": 252}} stands for the array
// from, from+step, ... of count numbers. Under an object key it becomes that array; as an
// element of an array it is spliced into it. Load expands every ramp of input and expected.
package contracttest

import (
	"bytes"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
)

//go:embed testdata/*.json
var fixtures embed.FS

// firstFile is the first fixture the loader lists.
const firstFile = "units.json"

// Case is one golden case. Input and Expected are JSON with the ramps expanded.
type Case struct {
	Name      string
	Group     string
	Input     json.RawMessage
	Expected  json.RawMessage
	Tolerance float64
	Working   string
}

// File is one fixture file. Provenance is empty for a hand file.
type File struct {
	Name       string
	Provenance json.RawMessage
	Cases      []Case
}

// Names lists the embedded fixture files: units.json first, the others in name order.
func Names() ([]string, error) {
	entries, err := fixtures.ReadDir("testdata")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Slice(names, func(i, j int) bool {
		if (names[i] == firstFile) != (names[j] == firstFile) {
			return names[i] == firstFile
		}
		return names[i] < names[j]
	})
	return names, nil
}

// Load reads, checks and expands one embedded fixture file by name.
func Load(name string) (File, error) {
	data, err := fixtures.ReadFile("testdata/" + name)
	if err != nil {
		return File{}, err
	}
	return Parse(name, data)
}

// Parse checks the shape of a fixture file and expands its ramps. Every case needs the
// five keys, a unique name, and a working unless the file has a provenance object.
func Parse(name string, data []byte) (File, error) {
	var raw struct {
		Provenance json.RawMessage              `json:"provenance"`
		Cases      []map[string]json.RawMessage `json:"cases"`
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return File{}, fmt.Errorf("%s: %w", name, err)
	}
	f := File{Name: name}
	if raw.Provenance != nil {
		if !bytes.HasPrefix(bytes.TrimSpace(raw.Provenance), []byte("{")) {
			return File{}, fmt.Errorf("%s: provenance is not an object", name)
		}
		f.Provenance = raw.Provenance
	}
	seen := map[string]bool{}
	for i, rc := range raw.Cases {
		c, err := parseCase(rc, f.Provenance != nil)
		if err != nil {
			return File{}, fmt.Errorf("%s: case %d: %w", name, i, err)
		}
		if seen[c.Name] {
			return File{}, fmt.Errorf("%s: case %d: duplicate name %q", name, i, c.Name)
		}
		seen[c.Name] = true
		f.Cases = append(f.Cases, c)
	}
	return f, nil
}

func parseCase(rc map[string]json.RawMessage, generated bool) (Case, error) {
	keys := []string{"name", "group", "input", "expected", "tolerance"}
	if !generated {
		keys = append(keys, "working")
	}
	for _, k := range keys {
		if _, ok := rc[k]; !ok {
			return Case{}, fmt.Errorf("missing key %q", k)
		}
	}
	var c Case
	if err := json.Unmarshal(rc["name"], &c.Name); err != nil || c.Name == "" {
		return Case{}, errors.New("name is not a non-empty string")
	}
	if err := json.Unmarshal(rc["group"], &c.Group); err != nil || c.Group == "" {
		return Case{}, errors.New("group is not a non-empty string")
	}
	if err := json.Unmarshal(rc["tolerance"], &c.Tolerance); err != nil || c.Tolerance < 0 {
		return Case{}, errors.New("tolerance is not a number of at least 0")
	}
	if w, ok := rc["working"]; ok {
		if err := json.Unmarshal(w, &c.Working); err != nil {
			return Case{}, errors.New("working is not a string")
		}
		if !generated && c.Working == "" {
			return Case{}, errors.New("working is empty")
		}
	}
	var err error
	if c.Input, err = ExpandRamps(rc["input"]); err != nil {
		return Case{}, fmt.Errorf("input: %w", err)
	}
	if c.Expected, err = ExpandRamps(rc["expected"]); err != nil {
		return Case{}, fmt.Errorf("expected: %w", err)
	}
	return c, nil
}

// ExpandRamps returns raw with every ramp spec replaced by the numbers it stands for.
func ExpandRamps(raw json.RawMessage) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, errors.New("trailing data after the JSON value")
	}
	out, err := expand(v)
	if err != nil {
		return nil, err
	}
	return json.Marshal(out)
}

func expand(v any) (any, error) {
	switch x := v.(type) {
	case map[string]any:
		if spec, ok := x["ramp"]; ok {
			if len(x) != 1 {
				return nil, errors.New("a ramp has no sibling key")
			}
			return ramp(spec)
		}
		for k, e := range x {
			r, err := expand(e)
			if err != nil {
				return nil, err
			}
			x[k] = r
		}
		return x, nil
	case []any:
		out := make([]any, 0, len(x))
		for _, e := range x {
			if m, ok := e.(map[string]any); ok {
				if _, isRamp := m["ramp"]; isRamp {
					r, err := expand(m)
					if err != nil {
						return nil, err
					}
					out = append(out, r.([]any)...)
					continue
				}
			}
			r, err := expand(e)
			if err != nil {
				return nil, err
			}
			out = append(out, r)
		}
		return out, nil
	}
	return v, nil
}

func ramp(spec any) ([]any, error) {
	m, ok := spec.(map[string]any)
	if !ok || len(m) != 3 {
		return nil, errors.New("a ramp is an object of exactly from, step and count")
	}
	num := func(k string) (float64, error) {
		n, ok := m[k].(json.Number)
		if !ok {
			return 0, fmt.Errorf("ramp %s is missing or not a number", k)
		}
		return n.Float64()
	}
	from, err := num("from")
	if err != nil {
		return nil, err
	}
	step, err := num("step")
	if err != nil {
		return nil, err
	}
	cnt, err := num("count")
	if err != nil {
		return nil, err
	}
	n := int(cnt)
	if cnt < 0 || float64(n) != cnt {
		return nil, errors.New("ramp count is not a whole number of at least 0")
	}
	out := make([]any, n)
	for i := range out {
		out[i] = from + step*float64(i)
	}
	return out, nil
}
