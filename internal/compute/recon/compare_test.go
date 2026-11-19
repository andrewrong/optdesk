package recon

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"

	"github.com/andrewrong/optdesk/internal/compute/reconapi"
)

func vals(kv ...string) []reconapi.Value {
	var out []reconapi.Value
	for i := 0; i+1 < len(kv); i += 2 {
		out = append(out, reconapi.Value{Key: kv[i], Amount: kv[i+1]})
	}
	return out
}

func TestMatchInsideTolerance(t *testing.T) {
	t.Parallel()
	o := Compare(reconapi.Input{
		Ours: vals("a", "100.00", "b", "5", "c", "7"), Other: vals("a", "100.09", "b", "5.0", "c", "7.0000"),
		Tol: reconapi.Tolerance{Rel: "0.001"},
	})
	if o.N != 3 || o.Matched != 3 || o.Diff != 0 {
		t.Errorf("out = %+v", o)
	}
	// absolute room, and exact when the tolerance is empty
	o = Compare(reconapi.Input{Ours: vals("a", "10.5"), Other: vals("a", "10.00"), Tol: reconapi.Tolerance{Abs: "1"}})
	if o.Matched != 1 {
		t.Errorf("abs: %+v", o)
	}
	o = Compare(reconapi.Input{Ours: vals("a", "10"), Other: vals("a", "10.00")})
	if o.Matched != 1 {
		t.Errorf("exact: %+v", o)
	}
}

func TestDiffOutsideTolerance(t *testing.T) {
	t.Parallel()
	o := Compare(reconapi.Input{
		Ours: vals("a", "102.00", "b", "1.5"), Other: vals("a", "100", "b", "2.25"),
		Tol: reconapi.Tolerance{Rel: "0.001"},
	})
	if o.Diff != 2 || o.N != 2 {
		t.Fatalf("out = %+v", o)
	}
	if o.Items[0].Diff != "2.00" || o.Items[1].Diff != "-0.75" {
		t.Errorf("diffs = %q %q", o.Items[0].Diff, o.Items[1].Diff)
	}
	o = Compare(reconapi.Input{Ours: vals("a", "x"), Other: vals("a", "1")})
	if o.Diff != 1 || o.Items[0].Diff != "" {
		t.Errorf("unparseable: %+v", o)
	}
}

func TestSkipKeysAreNotCounted(t *testing.T) {
	t.Parallel()
	o := Compare(reconapi.Input{
		Ours: vals("a", "1", "b", "2"), Other: vals("a", "1"),
		Skip: map[string]string{"b": "rate_limited", "z": "timeout"},
	})
	if o.N != 1 || o.Skipped != 2 || o.MissingOther != 0 {
		t.Fatalf("out = %+v", o)
	}
	if k := []string{o.Items[1].Key, o.Items[2].Key}; !reflect.DeepEqual(k, []string{"b", "z"}) {
		t.Errorf("keys = %v", k)
	}
	if it := o.Items[1]; it.Kind != reconapi.Skipped || it.Reason != "rate_limited" {
		t.Errorf("item = %+v", it)
	}
}

func TestOursOnlyKeyIsSkippedForR6(t *testing.T) {
	t.Parallel()
	o := Compare(reconapi.Input{Ours: vals("a", "1"), OtherMissingSkipped: true})
	if o.N != 0 || o.Skipped != 1 || o.Items[0].Kind != reconapi.Skipped || o.Items[0].Reason != SourceLacks {
		t.Errorf("out = %+v", o)
	}
}

func TestOursOnlyMissingOtherR1(t *testing.T) {
	t.Parallel()
	o := Compare(reconapi.Input{Ours: vals("a", "1")})
	if o.N != 1 || o.MissingOther != 1 || o.Items[0].Kind != reconapi.MissingOther {
		t.Errorf("out = %+v", o)
	}
}

func TestOtherOnlyKeyIsMissingOurs(t *testing.T) {
	t.Parallel()
	for _, skipped := range []bool{false, true} {
		o := Compare(reconapi.Input{Other: vals("a", "1"), OtherMissingSkipped: skipped})
		if o.N != 1 || o.MissingOurs != 1 || o.Items[0].Kind != reconapi.MissingOurs {
			t.Errorf("skipped=%v out = %+v", skipped, o)
		}
	}
}

func TestPairsGolden(t *testing.T) {
	t.Parallel()
	b, err := os.ReadFile("testdata/pairs.json")
	if err != nil {
		t.Fatal(err)
	}
	var g map[string]struct {
		Input reconapi.Input
		Want  reconapi.Output
	}
	if err := json.Unmarshal(b, &g); err != nil {
		t.Fatal(err)
	}
	for name, c := range g {
		got := Compare(c.Input)
		if !reflect.DeepEqual(got, c.Want) {
			t.Errorf("%s: got %+v\nwant %+v", name, got, c.Want)
		}
	}
}
