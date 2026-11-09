package settingsform

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/andrewrong/optdesk/internal/compute/settingsapi"
)

const paramPrefix = "param."

// Parse reads the param.<key>, row, excluded and confirm fields. A key not in
// the registry and an id not in shown are refused; a posted value is
// normalized through the registry and kept only when it differs from cur. A
// row without an excluded entry is included. A field not posted is unchanged.
func (f form) Parse(values map[string][]string, shown []int64, cur settingsapi.FormState) (settingsapi.FormChange, error) {
	var out settingsapi.FormChange
	known := map[string]bool{}
	specs := f.p.Specs()
	for _, s := range specs {
		known[s.Key] = true
	}
	for name := range values {
		if k, ok := strings.CutPrefix(name, paramPrefix); ok && !known[k] {
			return out, fmt.Errorf("settings form: unknown key %q", k)
		}
	}
	for _, s := range specs {
		vs, ok := values[paramPrefix+s.Key]
		if !ok || len(vs) == 0 {
			continue
		}
		text, err := f.p.Normalize(s.Key, vs[0])
		if err != nil {
			return out, fmt.Errorf("settings form: %s: %w", s.Key, err)
		}
		if text != cur.Params[s.Key] {
			out.Params = append(out.Params, settingsapi.FormField{Key: s.Key, Text: text})
		}
	}

	isShown := map[int64]bool{}
	for _, id := range shown {
		isShown[id] = true
	}
	rows, err := ids(values["row"], isShown)
	if err != nil {
		return out, err
	}
	excl, err := ids(values["excluded"], isShown)
	if err != nil {
		return out, err
	}
	checked := map[int64]bool{}
	for _, id := range excl {
		checked[id] = true
	}
	for _, id := range rows {
		if checked[id] != cur.Exclusions[id] {
			if out.Exclusions == nil {
				out.Exclusions = map[int64]bool{}
			}
			out.Exclusions[id] = checked[id]
		}
	}
	for _, v := range values["confirm"] {
		if v == "1" {
			out.Confirm = true
		}
	}
	return out, nil
}

// ids reads decimal instrument ids and refuses one that is not shown.
func ids(texts []string, shown map[int64]bool) ([]int64, error) {
	var out []int64
	for _, t := range texts {
		id, err := strconv.ParseInt(t, 10, 64)
		if err != nil || !shown[id] {
			return nil, fmt.Errorf("settings form: instrument %q not shown", t)
		}
		out = append(out, id)
	}
	return out, nil
}
