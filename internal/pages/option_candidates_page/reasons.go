package option_candidates_page

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
)

var (
	gatePattern = regexp.MustCompile(`^[a-z_]{1,32}$`)
	codePattern = regexp.MustCompile(`^[a-z_]{1,48}$`)
)

var errBadGates = errors.New("option_candidates_page: unreadable gates")

// reason is one gate that did not pass.
type reason struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Code   string `json:"code"`
	Text   string `json:"text"`
}

type gateResult struct {
	Gate   string `json:"gate"`
	Status string `json:"status"`
	Reason string `json:"reason"`
}

// parseGates reads the stored JSON array text of the gate results; a text that is not one, or a
// result the page could not show, is an error.
func parseGates(text string) ([]gateResult, error) {
	var gates []gateResult
	if err := json.Unmarshal([]byte(text), &gates); err != nil || gates == nil {
		return nil, errBadGates
	}
	for _, g := range gates {
		if !gatePattern.MatchString(g.Gate) {
			return nil, errBadGates
		}
		switch g.Status {
		case "pass":
		case "block", "unknown":
			if !codePattern.MatchString(g.Reason) {
				return nil, errBadGates
			}
		default:
			return nil, errBadGates
		}
	}
	return gates, nil
}

// reasonText is the sentence of a known code, else the code itself.
func reasonText(code string) string {
	if s, ok := reasonTexts[code]; ok {
		return s
	}
	return code
}

// reasonsOf is one reason per gate that is not a pass, in the stored order.
func reasonsOf(gates []gateResult) []reason {
	out := []reason{}
	for _, g := range gates {
		if g.Status == "pass" {
			continue
		}
		out = append(out, reason{Gate: g.Gate, Status: g.Status, Code: g.Reason, Text: reasonText(g.Reason)})
	}
	return out
}

// reasonsLine is the texts of the reasons in one line.
func reasonsLine(rs []reason) string {
	texts := make([]string, len(rs))
	for i, r := range rs {
		texts[i] = r.Text
	}
	return strings.Join(texts, listSeparator)
}

// parseMarks reads the stored JSON array text of the marks.
func parseMarks(text string) ([]string, error) {
	var marks []string
	if err := json.Unmarshal([]byte(text), &marks); err != nil || marks == nil {
		return nil, errBadGates
	}
	return marks, nil
}

// parseFits reads the stored JSON object text from alias to fit in the stored order of its keys.
func parseFits(text string) ([][2]string, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	if t, err := dec.Token(); err != nil || t != json.Delim('{') {
		return nil, errBadGates
	}
	out := [][2]string{}
	for dec.More() {
		k, err := dec.Token()
		key, ok := k.(string)
		if err != nil || !ok {
			return nil, errBadGates
		}
		var v string
		if err := dec.Decode(&v); err != nil {
			return nil, errBadGates
		}
		out = append(out, [2]string{key, v})
	}
	if t, err := dec.Token(); err != nil || t != json.Delim('}') {
		return nil, errBadGates
	}
	if _, err := dec.Token(); err == nil {
		return nil, errBadGates
	}
	return out, nil
}
