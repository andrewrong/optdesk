package cutover

import (
	"errors"
	"fmt"
	"io"

	"github.com/pelletier/go-toml/v2"
)

// figureEntry is the part of one [[figure]] of the register that the comparison reads.
type figureEntry struct {
	ID       string `toml:"id"`
	Version  int    `toml:"version"`
	AgreedIn string `toml:"agreed_in"`
}

// readRegister reads the [[figure]] entries of the register in file order. An entry with no id or a
// version below 1 is refused.
func readRegister(r io.Reader) ([]figureEntry, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, errors.New("register: read failed")
	}
	var doc struct {
		Figure []figureEntry `toml:"figure"`
	}
	if err := toml.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("register: %w", err)
	}
	for i, f := range doc.Figure {
		if f.ID == "" || f.Version < 1 {
			return nil, fmt.Errorf("register: figure %d has no id or no version", i+1)
		}
	}
	return doc.Figure, nil
}
