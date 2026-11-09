package cutover

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// LineError is an error of one line of a JSON Lines file. It carries the 1-based line number and the cause
// and never the content of the line.
type LineError struct {
	Line int
	Err  error
}

func (e *LineError) Error() string { return fmt.Sprintf("line %d: %v", e.Line, e.Err) }

// Unwrap returns the cause.
func (e *LineError) Unwrap() error { return e.Err }

// ReadLines calls fn once per non-blank line of r with its 1-based number (blank lines are counted, not
// passed) and the line without its end of line. An error of fn or of reading is returned as a *LineError.
func ReadLines(r io.Reader, fn func(line int, raw []byte) error) error {
	br := bufio.NewReader(r)
	for n := 1; ; n++ {
		raw, err := br.ReadBytes('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return &LineError{Line: n, Err: errors.New("read failed")}
		}
		if text := bytes.TrimRight(raw, "\r\n"); len(bytes.TrimSpace(text)) > 0 {
			if ferr := fn(n, text); ferr != nil {
				return &LineError{Line: n, Err: ferr}
			}
		}
		if err != nil {
			return nil
		}
	}
}

// CanonicalHash is the lower-case hex SHA-256 of one JSON object line re-encoded with its keys ascending by
// byte order at every depth, no whitespace and no HTML escaping. Numbers keep their text. A line that is not
// one JSON object is refused; the error holds no content of the line.
func CanonicalHash(line []byte) (string, error) {
	dec := json.NewDecoder(bytes.NewReader(line))
	dec.UseNumber()
	var v map[string]any
	if err := dec.Decode(&v); err != nil || v == nil {
		return "", errors.New("not a JSON object")
	}
	if _, err := dec.Token(); !errors.Is(err, io.EOF) {
		return "", errors.New("data after the JSON object")
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil { // maps encode with sorted keys
		return "", errors.New("cannot encode the JSON object")
	}
	sum := sha256.Sum256(bytes.TrimRight(buf.Bytes(), "\n"))
	return hex.EncodeToString(sum[:]), nil
}
