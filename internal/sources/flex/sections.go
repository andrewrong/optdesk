package flex

import (
	"encoding/xml"
	"errors"
	"io"

	"github.com/andrewrong/optdesk/internal/sources"
)

// sectionsOf reads the statement sections of a report body by the container rule: for each direct
// child of a statement other than Trades, if it has child elements each child element is a row under
// the child's element name; if it has none its own attributes are one row under its own name. Every
// attribute is kept as text as sent. No section is named here. The result has one map per
// statement, in document order, never nil.
func sectionsOf(body []byte) ([]map[string][]map[string]string, error) {
	d := newDecoder(body)
	var (
		path    []string
		out     []map[string][]map[string]string
		cur     map[string][]map[string]string
		child   xml.StartElement
		hasRows bool
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, failCode(kindNotReport, "xml")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch len(path) {
			case 2:
				if t.Name.Local == "FlexStatement" {
					cur = map[string][]map[string]string{}
					out = append(out, cur)
				}
			case 3:
				child, hasRows = t, false
			case 4:
				if cur != nil && child.Name.Local != "Trades" {
					hasRows = true
					cur[t.Name.Local] = append(cur[t.Name.Local], attrs(t))
				}
			}
			path = append(path, t.Name.Local)
		case xml.EndElement:
			path = path[:len(path)-1]
			if len(path) == 3 && cur != nil && !hasRows && child.Name.Local != "Trades" {
				cur[child.Name.Local] = append(cur[child.Name.Local], attrs(child))
			}
			if len(path) == 2 {
				cur = nil
			}
		}
	}
}

// fillSections sets Sections of every statement from the body.
func fillSections(body []byte, sts []sources.FlexStatement) error {
	secs, err := sectionsOf(body)
	if err != nil {
		return err
	}
	for i := range sts {
		if i < len(secs) {
			sts[i].Sections = secs[i]
		}
	}
	return nil
}
