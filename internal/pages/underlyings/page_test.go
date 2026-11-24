package underlyings

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/andrewrong/optdesk/internal/pages"
)

func TestPage(t *testing.T) {
	t.Parallel()
	p := Page()
	if want := (pages.Meta{Name: "underlyings", Schema: "page.underlyings.v2"}); p.Meta != want {
		t.Fatalf("meta = %+v", p.Meta)
	}
	out, err := p.Build(t.Context(), nil, "2026-11-04", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if m["session_day"] != "2026-11-04" || m["form"] != nil {
		t.Fatalf("got %s", out)
	}
	validate(t, out)
}

func TestBuildNilRows(t *testing.T) {
	t.Parallel()
	out, err := Build(Input{Now: fixedNow})
	if err != nil {
		t.Fatal(err)
	}
	m := decode(t, out)
	if rows, ok := m["rows"].([]any); !ok || len(rows) != 0 {
		t.Fatalf("rows = %#v", m["rows"])
	}
	if m["sentence"] != noNamesText {
		t.Fatalf("sentence = %v", m["sentence"])
	}
	if m["session_day"] != "2000-01-01" {
		t.Fatalf("session_day = %v", m["session_day"])
	}
	validate(t, out)
}

func decode(t *testing.T, b []byte) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	return m
}

func TestUnderlyingsTextsAreChinese(t *testing.T) {
	t.Parallel()
	old := []string{
		"Days missed while a group", "Pending: resolves after the close",
		"Followed by", "Added: held position", "0 names.", "Editing is off",
		"Held names could not be read",
	}
	ro := fixtureInput()
	ro.Form = nil
	readOnly := string(mustBuild(t, ro))
	confirm := fixtureInput()
	confirm.Confirm = &Confirm{Stops: []Stop{{Group: "bars", Symbol: "CCC", Stopped: true}}}
	withConfirm := string(mustBuild(t, confirm))
	for name, c := range map[string]struct{ page, want string }{
		"read-only sentence": {readOnly, readOnlyText},
		"refill sentence":    {withConfirm, refillText},
		"pending text":       {readOnly, pendingText},
		"followed-by text":   {readOnly, followedPref},
		"followed by none":   {readOnly, followedNone},
	} {
		if !strings.Contains(c.page, c.want) {
			t.Errorf("%s: page does not hold %q", name, c.want)
		}
		if !hanRe.MatchString(c.want) {
			t.Errorf("%s: %q holds no Chinese character", name, c.want)
		}
	}
	for _, page := range []string{readOnly, withConfirm, string(mustBuild(t, Input{Now: fixedNow}))} {
		for _, o := range old {
			if strings.Contains(page, o) {
				t.Errorf("page still holds the old English sentence %q", o)
			}
		}
	}
}

var hanRe = regexp.MustCompile(`\p{Han}`)

func TestReadOnlyAndHeldReadTextsAreChinese(t *testing.T) {
	t.Parallel()
	for _, s := range []string{readOnlyText, heldReadText} {
		if !hanRe.MatchString(s) {
			t.Errorf("%q holds no Chinese character", s)
		}
	}
}
