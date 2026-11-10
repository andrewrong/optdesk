package cutover_test

import (
	"strings"
	"testing"

	"github.com/andrewrong/optdesk/internal/jobs/cutover"
)

func figs(entries ...string) string {
	var b strings.Builder
	for i := 0; i < len(entries); i += 2 {
		b.WriteString("[[figure]]\nid = \"" + entries[i] + "\"\nversion = " + entries[i+1] + "\nagreed_in = \"x\"\n\n")
	}
	return b.String()
}

func TestClassMatchRule(t *testing.T) {
	t.Parallel()
	classes := `
[[class]]
prefix = "home"
class = "replayable"
[[class]]
prefix = "home.options"
class = "new"
[[class]]
prefix = "m"
class = "new"
[[class]]
prefix = "m.deep"
class = "replayable"
[[class]]
id = "a.b"
class = "new"
[[class]]
id = "a.b"
version = 2
class = "replayable"
`
	reg := figs("home.options.x", "1", "home.options_summary.x", "1", "m.x", "1", "m.deep.x", "1", "a.b", "1", "a.b", "2", "home.options", "1")
	rep, err := run(t, "", reg, classes, "", &fakeReader{})
	if err != nil {
		t.Fatal(err)
	}
	// new shows as status new; replayable with no old row shows as unchecked.
	want := []cutover.FigureStatus{cutover.FigureNew, cutover.FigureUnchecked, cutover.FigureNew, cutover.FigureUnchecked,
		cutover.FigureNew, cutover.FigureUnchecked, cutover.FigureNew}
	for i, w := range want {
		if got := rep.Results[i].Status; got != w {
			t.Errorf("%s v%d = %s, want %s", rep.Results[i].ID, rep.Results[i].Version, got, w)
		}
	}
}

func TestUnclassifiedIsError(t *testing.T) {
	t.Parallel()
	classes := "[[class]]\nprefix = \"home\"\nclass = \"replayable\"\n"
	rep, err := run(t, "", figs("zeta.one", "1", "home.a", "1", "alpha.two", "1"), classes, "", &fakeReader{})
	if err == nil || !strings.Contains(err.Error(), "alpha.two, zeta.one") {
		t.Fatalf("err = %v, want the unclassified ids", err)
	}
	if rep.OK || len(rep.Results) != 0 {
		t.Fatalf("report = %+v, want zero", rep)
	}
}
