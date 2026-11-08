package contracttest

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute"
)

const handCase = `{"name":"a","group":"g","input":1,"expected":2,"tolerance":0,"working":"1+1"}`

func TestFixtureShape(t *testing.T) {
	t.Parallel()

	names, err := Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f, err := Load(name)
			if err != nil {
				t.Fatalf("Load(%s): %v", name, err)
			}
			if len(f.Cases) == 0 {
				t.Fatalf("%s has no case", name)
			}
		})
	}

	t.Run("hand file without working fails", func(t *testing.T) {
		t.Parallel()
		data := `{"cases":[{"name":"a","group":"g","input":1,"expected":2,"tolerance":0}]}`
		if _, err := Parse("copy.json", []byte(data)); err == nil {
			t.Fatal("a hand file case without working was accepted")
		}
	})
	t.Run("provenance file without working passes", func(t *testing.T) {
		t.Parallel()
		data := `{"provenance":{"generator":"x"},"cases":[{"name":"a","group":"g","input":1,"expected":2,"tolerance":0}]}`
		f, err := Parse("copy.json", []byte(data))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if len(f.Provenance) == 0 {
			t.Fatal("provenance was dropped")
		}
	})
	t.Run("hand file with working passes", func(t *testing.T) {
		t.Parallel()
		if _, err := Parse("copy.json", []byte(`{"cases":[`+handCase+`]}`)); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})
	for _, key := range []string{"name", "group", "input", "expected", "tolerance"} {
		t.Run("missing "+key+" fails", func(t *testing.T) {
			t.Parallel()
			var c map[string]json.RawMessage
			if err := json.Unmarshal([]byte(handCase), &c); err != nil {
				t.Fatal(err)
			}
			delete(c, key)
			b, _ := json.Marshal(c)
			if _, err := Parse("copy.json", []byte(`{"cases":[`+string(b)+`]}`)); err == nil {
				t.Fatalf("a case without %q was accepted", key)
			}
		})
	}
	t.Run("duplicate name fails", func(t *testing.T) {
		t.Parallel()
		if _, err := Parse("copy.json", []byte(`{"cases":[`+handCase+","+handCase+`]}`)); err == nil {
			t.Fatal("a duplicate case name was accepted")
		}
	})
	t.Run("provenance that is not an object fails", func(t *testing.T) {
		t.Parallel()
		data := `{"provenance":"x","cases":[` + handCase + `]}`
		if _, err := Parse("copy.json", []byte(data)); err == nil {
			t.Fatal("a provenance string was accepted")
		}
	})
}

func TestUnitsFirst(t *testing.T) {
	t.Parallel()
	names, err := Names()
	if err != nil {
		t.Fatalf("Names: %v", err)
	}
	if len(names) != 6 {
		t.Fatalf("names = %v, want the four hand files and the two generated ones", names)
	}
	if names[0] != "units.json" {
		t.Fatalf("first name = %q, want units.json", names[0])
	}
	want := []string{"units.json", "figures.json", "indicators.json", "iv30_independent.json", "series.json", "stats.json"}
	if !reflect.DeepEqual(names, want) {
		t.Fatalf("names = %v, want %v", names, want)
	}
}

func TestRampExpands(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, in, want string
		wantErr        bool
	}{
		{"count step from", `{"ramp":{"from":1,"step":1,"count":4}}`, `[1,2,3,4]`, false},
		{"step two from five", `{"ramp":{"from":5,"step":2,"count":3}}`, `[5,7,9]`, false},
		{"negative step", `{"ramp":{"from":1,"step":-0.5,"count":3}}`, `[1,0.5,0]`, false},
		{"zero step", `{"ramp":{"from":7,"step":0,"count":3}}`, `[7,7,7]`, false},
		{"count zero", `{"ramp":{"from":1,"step":1,"count":0}}`, `[]`, false},
		{"spliced into an array", `{"v":[{"ramp":{"from":1,"step":1,"count":2}},9]}`, `{"v":[1,2,9]}`, false},
		{"nested under keys", `{"a":{"b":{"ramp":{"from":0,"step":1,"count":2}}}}`, `{"a":{"b":[0,1]}}`, false},
		{"plain value untouched", `{"x":[1,"s",null,true]}`, `{"x":[1,"s",null,true]}`, false},
		{"missing step", `{"ramp":{"from":1,"count":2}}`, ``, true},
		{"unknown key", `{"ramp":{"from":1,"step":1,"count":2,"x":1}}`, ``, true},
		{"negative count", `{"ramp":{"from":1,"step":1,"count":-1}}`, ``, true},
		{"fractional count", `{"ramp":{"from":1,"step":1,"count":1.5}}`, ``, true},
		{"ramp beside another key", `{"ramp":{"from":1,"step":1,"count":1},"x":1}`, ``, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ExpandRamps(json.RawMessage(tc.in))
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ExpandRamps(%s) = %s, want an error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ExpandRamps: %v", err)
			}
			var g, w any
			if err := json.Unmarshal(got, &g); err != nil {
				t.Fatalf("result is not JSON: %v", err)
			}
			if err := json.Unmarshal([]byte(tc.want), &w); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(g, w) {
				t.Fatalf("ExpandRamps(%s) = %s, want %s", tc.in, got, tc.want)
			}
		})
	}

	t.Run("252 values", func(t *testing.T) {
		t.Parallel()
		got, err := ExpandRamps(json.RawMessage(`{"ramp":{"from":1,"step":1,"count":252}}`))
		if err != nil {
			t.Fatal(err)
		}
		var v []float64
		if err := json.Unmarshal(got, &v); err != nil {
			t.Fatal(err)
		}
		if len(v) != 252 || v[0] != 1 || v[251] != 252 {
			t.Fatalf("got %d values from %v to %v", len(v), v[0], v[len(v)-1])
		}
	})
	t.Run("loaded cases carry no ramp", func(t *testing.T) {
		t.Parallel()
		names, _ := Names()
		for _, name := range names {
			f, err := Load(name)
			if err != nil {
				t.Fatal(err)
			}
			for _, c := range f.Cases {
				if strings.Contains(string(c.Input), `"ramp"`) || strings.Contains(string(c.Expected), `"ramp"`) {
					t.Errorf("%s/%s still holds a ramp", name, c.Name)
				}
			}
		}
	})
}

func TestGoldenCoversFigures(t *testing.T) {
	t.Parallel()
	f, err := Load("figures.json")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	okByMethod := map[string]bool{}
	byReason := map[compute.Reason]bool{}
	for _, c := range f.Cases {
		var exp struct {
			Status compute.Status `json:"status"`
			Reason compute.Reason `json:"reason"`
		}
		if err := json.Unmarshal(c.Expected, &exp); err != nil {
			t.Fatalf("%s: expected: %v", c.Name, err)
		}
		switch exp.Status {
		case compute.StatusOK:
			if exp.Reason != "" {
				t.Errorf("%s: ok case with reason %q", c.Name, exp.Reason)
			}
			okByMethod[c.Group] = true
		case compute.StatusMissing:
			byReason[exp.Reason] = true
		default:
			t.Errorf("%s: status %q", c.Name, exp.Status)
		}
	}

	typ := reflect.TypeOf((*compute.Figures)(nil)).Elem()
	for i := 0; i < typ.NumMethod(); i++ {
		m := typ.Method(i).Name
		if !okByMethod[m] {
			t.Errorf("no ok case for Figures.%s", m)
		}
	}
	for _, g := range groupsOf(f) {
		if _, ok := typ.MethodByName(g); !ok {
			t.Errorf("group %q is not a Figures method", g)
		}
	}
	for _, r := range []compute.Reason{compute.NoInput, compute.ShortHistory, compute.StaleInput, compute.ZeroRange, compute.BadInput} {
		if !byReason[r] {
			t.Errorf("no case for reason %q", r)
		}
	}
}

func groupsOf(f File) []string {
	seen := map[string]bool{}
	var out []string
	for _, c := range f.Cases {
		if !seen[c.Group] {
			seen[c.Group] = true
			out = append(out, c.Group)
		}
	}
	return out
}
