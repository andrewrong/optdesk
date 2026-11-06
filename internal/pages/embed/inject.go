package embed

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// The vm element is the one place the page reads its data from.
const (
	vmOpen    = `<script id="vm" type="application/json">`
	vmClose   = `</script>`
	vmElement = vmOpen + vmClose
)

// jsonEscaper turns the characters that could end or confuse a script element
// into \u escapes; the parsed JSON is unchanged.
var jsonEscaper = strings.NewReplacer(
	"<", "\\u003c",
	">", "\\u003e",
	"&", "\\u0026",
	"\u2028", "\\u2028",
	"\u2029", "\\u2029",
)

// Render returns index with viewModel written into its vm element. viewModel
// must be valid JSON; index must hold the vm element exactly once.
func Render(index, viewModel []byte) ([]byte, error) {
	var compact bytes.Buffer
	if err := json.Compact(&compact, viewModel); err != nil {
		return nil, fmt.Errorf("embed: view model is not valid JSON: %w", err)
	}
	if bytes.Count(index, []byte(vmElement)) != 1 {
		return nil, errors.New("embed: index must hold the vm element exactly once")
	}
	data := jsonEscaper.Replace(compact.String())
	return bytes.Replace(index, []byte(vmElement), []byte(vmOpen+data+vmClose), 1), nil
}
