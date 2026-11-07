package option_data_page

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "..", "contracts", "fixtures", "page.option_data_page.v1."+name+".json"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func sameJSON(t *testing.T, got []byte, name string) {
	t.Helper()
	g, w := decode(t, got), decode(t, fixture(t, name))
	if !reflect.DeepEqual(g, w) {
		gb, _ := json.MarshalIndent(g, "", " ")
		wb, _ := json.MarshalIndent(w, "", " ")
		t.Fatalf("page differs from %s\n got: %s\nwant: %s", name, gb, wb)
	}
}

func TestBuildChoicesGolden(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	e, q := fromInput(in)
	c := inputCfg(in, e, q)
	c.Notes = []string{"Synthetic fixture."}
	for _, id := range []int64{9000, 9010, 9020} {
		c.Choices = append(c.Choices, Choice{ID: id, Label: fmt.Sprintf("Underlying %d", id), Href: fmt.Sprintf("/options/data/%d", id)})
	}
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, "choices")
}

func TestBuildNoNewFields(t *testing.T) {
	t.Parallel()
	in := loadInput(t)
	e, q := fromInput(in)
	c := inputCfg(in, e, q)
	c.Notes = []string{"Synthetic fixture."}
	b, err := Build(t.Context(), c, "2026-11-06", fixedAt)
	if err != nil {
		t.Fatal(err)
	}
	sameJSON(t, b, "ok")
	if _, ok := decode(t, b)["underlyings"]; ok {
		t.Fatal("underlyings present without choices")
	}
}

func TestBuildChoices(t *testing.T) {
	t.Parallel()
	base := func() Config {
		return cfg(&fakeEod{rows: []contracts.OptionEod{eodRow(t, 1, f64(1), "2026-11-06T21:00:00Z")}}, &fakeQuotes{})
	}
	t.Run("capped at 50 in order", func(t *testing.T) {
		c := base()
		for i := 0; i < 60; i++ {
			c.Choices = append(c.Choices, Choice{ID: int64(i), Label: "L", Href: "/o/" + fmt.Sprint(i)})
		}
		list := buildMap(t, c)["underlyings"].([]any)
		if len(list) != 50 || list[49].(map[string]any)["id"] != float64(49) {
			t.Fatalf("got %d choices", len(list))
		}
	})
	t.Run("current marks the underlying only", func(t *testing.T) {
		c := base()
		c.Choices = []Choice{{ID: 1, Label: "a", Href: "/a"}, {ID: 9000, Label: "b", Href: "/b"}, {ID: 2, Label: "c", Href: "/c"}}
		var got []bool
		for _, x := range buildMap(t, c)["underlyings"].([]any) {
			got = append(got, x.(map[string]any)["current"].(bool))
		}
		if !reflect.DeepEqual(got, []bool{false, true, false}) {
			t.Fatalf("current = %v", got)
		}
		c.UnderlyingID = 77
		for _, x := range buildMap(t, c)["underlyings"].([]any) {
			if x.(map[string]any)["current"].(bool) {
				t.Fatal("a choice is current without a match")
			}
		}
	})
	for _, href := range []string{"https://x", "javascript:x", "", "/a b"} {
		t.Run("bad href "+href, func(t *testing.T) {
			c := base()
			c.Choices = []Choice{{ID: 1, Label: "a", Href: "/ok"}, {ID: 2, Label: "b", Href: href}}
			b, err := Build(t.Context(), c, "2026-11-06", time.Time{})
			if err == nil || b != nil {
				t.Fatalf("err = %v, bytes = %d", err, len(b))
			}
		})
	}
}
