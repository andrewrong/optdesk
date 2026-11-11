package option_boards

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

const (
	schemaDir  = "../../../contracts/schemas/"
	schemaHost = "https://optdesk.invalid/schemas/"
)

type fileLoader struct{}

func (fileLoader) Load(url string) (any, error) {
	file, ok := strings.CutPrefix(url, schemaHost)
	if !ok || strings.ContainsAny(file, "/\\") {
		return nil, fmt.Errorf("unexpected schema url %q", url)
	}
	b, err := os.ReadFile(schemaDir + file)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(b))
}

func compileSchema(t *testing.T, file string) *jsonschema.Schema {
	t.Helper()
	c := jsonschema.NewCompiler()
	c.AssertFormat()
	c.UseLoader(fileLoader{})
	s, err := c.Compile(schemaHost + file)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOptionPageValidatesSchema(t *testing.T) {
	t.Parallel()
	schema := compileSchema(t, "page.option_page.v2.json")
	stale := okRow(1, "AAA", 1, 1, 1)
	stale.SessionDay = "2026-11-08"
	cases := map[string]store.CandidateReader{
		"ok":        &fakeCands{byDay: map[contracts.Day][]store.Candidate{buildDay: fixtureRows()}},
		"stale_day": &fakeCands{byDay: map[contracts.Day][]store.Candidate{"2026-11-08": {stale}}},
		"no_rows":   &fakeCands{},
		"no_reader": nil,
	}
	for name, cands := range cases {
		b := buildOpt(t, cands, fixtureEod(), buildDay)
		v, err := jsonschema.UnmarshalJSON(bytes.NewReader(b))
		if err != nil {
			t.Fatal(err)
		}
		if err := schema.Validate(v); err != nil {
			t.Errorf("%s: %v\n%s", name, err, b)
		}
	}
}
