package alertrules

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/pelletier/go-toml/v2"

	"github.com/andrewrong/optdesk/internal/compute"
)

//go:embed rules.toml
var defaultFile []byte

// MaxTitleLen is the longest title of a rule, in characters.
const MaxTitleLen = 60

var (
	_ compute.LoadAlertRulesFunc    = Load
	_ compute.DefaultAlertRulesFunc = DefaultRules
	_ compute.EvaluateAlertsFunc    = Evaluate
)

var decimalRe = regexp.MustCompile(`^-?[0-9]+(\.[0-9]+)?$`)

var (
	ops     = map[string]bool{"gt": true, "ge": true, "lt": true, "le": true}
	levels  = map[string]bool{"n": true, "w": true, "d": true}
	kinds   = map[string]bool{"market": true, "underlying": true}
	unitSet = map[string]bool{
		"decimal": true, "percent": true, "vol_point": true, "per_calendar_day": true,
		"count": true, "days": true, "money": true, "ratio": true, "text_tag": true,
	}
)

type file struct {
	Version        int        `toml:"version"`
	PublicSubjects []string   `toml:"public_subjects"`
	Rule           []ruleItem `toml:"rule"`
}

type ruleItem struct {
	ID             string `toml:"id"`
	Title          string `toml:"title"`
	Figure         string `toml:"figure"`
	FigureVersion  int    `toml:"figure_version"`
	SubjectKind    string `toml:"subject_kind"`
	Subject        string `toml:"subject"`
	Unit           string `toml:"unit"`
	Op             string `toml:"op"`
	Threshold      string `toml:"threshold"`
	Level          string `toml:"level"`
	MaxDataAgeDays int    `toml:"max_data_age_days"`
}

// DefaultRules returns the rules embedded in the program.
func DefaultRules() (compute.AlertRules, error) {
	return Load(bytes.NewReader(defaultFile))
}

// Load decodes a rule file strictly and checks it. An error names the key and the rule id
// and never a value.
func Load(r io.Reader) (compute.AlertRules, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return compute.AlertRules{}, errors.New("alertrules: the rule file cannot be read")
	}
	var f file
	dec := toml.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&f); err != nil {
		return compute.AlertRules{}, decodeError(err)
	}
	if f.Version != 1 {
		return compute.AlertRules{}, errors.New(`alertrules: key "version" must be 1`)
	}
	out := compute.AlertRules{PublicSubjects: f.PublicSubjects}
	seen := map[string]bool{}
	for i, it := range f.Rule {
		if err := check(it, i, seen); err != nil {
			return compute.AlertRules{}, err
		}
		out.Rules = append(out.Rules, compute.AlertRule(it))
	}
	sum := sha256.Sum256(raw)
	out.Hash = hex.EncodeToString(sum[:])
	return out, nil
}

func decodeError(err error) error {
	var sm *toml.StrictMissingError
	if errors.As(err, &sm) && len(sm.Errors) > 0 {
		return fmt.Errorf("alertrules: unknown key %q", strings.Join(sm.Errors[0].Key(), "."))
	}
	var de *toml.DecodeError
	if errors.As(err, &de) {
		line, col := de.Position()
		return fmt.Errorf("alertrules: the rule file does not decode at line %d, column %d", line, col)
	}
	return errors.New("alertrules: the rule file does not decode")
}

func check(it ruleItem, i int, seen map[string]bool) error {
	bad := func(key, why string) error {
		id := it.ID
		if id == "" {
			id = fmt.Sprintf("#%d", i+1)
		}
		return fmt.Errorf("alertrules: rule %q: key %q %s", id, key, why)
	}
	if it.ID == "" {
		return bad("id", "is empty")
	}
	if seen[it.ID] {
		return bad("id", "is a duplicate")
	}
	seen[it.ID] = true
	switch {
	case it.Title == "":
		return bad("title", "is empty")
	case utf8.RuneCountInString(it.Title) > MaxTitleLen:
		return bad("title", fmt.Sprintf("is longer than %d characters", MaxTitleLen))
	case it.Figure == "":
		return bad("figure", "is empty")
	case it.FigureVersion < 1:
		return bad("figure_version", "is below 1")
	case !kinds[it.SubjectKind]:
		return bad("subject_kind", "is not market or underlying")
	case it.Subject == "":
		return bad("subject", "is empty")
	case !unitSet[it.Unit]:
		return bad("unit", "is not a reading unit")
	case !ops[it.Op]:
		return bad("op", "is not one of gt, ge, lt, le")
	case !decimalRe.MatchString(it.Threshold):
		return bad("threshold", "is not a decimal")
	case !levels[it.Level]:
		return bad("level", "is not one of n, w, d")
	case it.MaxDataAgeDays < 0:
		return bad("max_data_age_days", "is below 0")
	}
	return nil
}
