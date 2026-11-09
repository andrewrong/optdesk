package flex

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"

	"github.com/andrewrong/optdesk/internal/sources"
)

// levelExecution is the only level of detail whose Trade rows are kept.
const levelExecution = "EXECUTION"

// prelude checks a body before it is decoded: no NUL in the first 4 bytes (a UTF-16 or UTF-32
// body), no document type and no entity declaration anywhere.
func prelude(body []byte) error {
	head := body
	if len(head) > 4 {
		head = head[:4]
	}
	if bytes.IndexByte(head, 0) >= 0 {
		return failCode(kindNotReport, "encoding")
	}
	upper := bytes.ToUpper(body)
	if bytes.Contains(upper, []byte("<!DOCTYPE")) || bytes.Contains(upper, []byte("<!ENTITY")) {
		return failCode(kindNotReport, "doctype")
	}
	return nil
}

// newDecoder returns a strict decoder with an empty entity map.
func newDecoder(body []byte) *xml.Decoder {
	d := xml.NewDecoder(bytes.NewReader(body))
	d.Strict = true
	d.Entity = map[string]string{}
	return d
}

func attrs(se xml.StartElement) map[string]string {
	m := make(map[string]string, len(se.Attr))
	for _, a := range se.Attr {
		m[a.Name.Local] = a.Value
	}
	return m
}

// statusResponse is a FlexStatementResponse: the answer of SendRequest, or the error answer of
// GetStatement.
type statusResponse struct {
	Status        string
	ReferenceCode string
	ErrorCode     string
}

// rootName returns the name of the root element after the prelude checks.
func rootName(body []byte) (string, error) {
	if err := prelude(body); err != nil {
		return "", err
	}
	d := newDecoder(body)
	for {
		tok, err := d.Token()
		if err != nil {
			return "", failCode(kindNotReport, "xml")
		}
		if se, ok := tok.(xml.StartElement); ok {
			return se.Name.Local, nil
		}
	}
}

// parseStatusResponse reads a FlexStatementResponse.
func parseStatusResponse(body []byte) (statusResponse, error) {
	var r statusResponse
	root, err := rootName(body)
	if err != nil {
		return r, err
	}
	if root != "FlexStatementResponse" {
		return r, failCode(kindNotReport, "root")
	}
	d := newDecoder(body)
	var path []string
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			return r, nil
		}
		if err != nil {
			return r, failCode(kindNotReport, "xml")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			path = append(path, t.Name.Local)
		case xml.EndElement:
			path = path[:len(path)-1]
		case xml.CharData:
			if len(path) != 2 {
				continue
			}
			v := string(bytes.TrimSpace(t))
			switch path[1] {
			case "Status":
				r.Status += v
			case "ReferenceCode":
				r.ReferenceCode += v
			case "ErrorCode":
				r.ErrorCode += v
			}
		}
	}
}

// parseReport reads a FlexQueryResponse into statements. Only <Trade> rows of level EXECUTION are
// kept, as text; the rows of other levels and of other sections are counted in OtherLevels. A
// document type, another root, a statement without accountId, or a Trade without transactionID or
// levelOfDetail, or of another account than its statement, refuses the whole report.
func parseReport(body []byte) ([]sources.FlexStatement, error) {
	root, err := rootName(body)
	if err != nil {
		return nil, err
	}
	if root != "FlexQueryResponse" {
		return nil, failCode(kindNotReport, "root")
	}
	d := newDecoder(body)
	var (
		path    []string
		out     []sources.FlexStatement
		cur     *sources.FlexStatement
		section string
	)
	for {
		tok, err := d.Token()
		if errors.Is(err, io.EOF) {
			if err := fillSections(body, out); err != nil {
				return nil, err
			}
			return out, nil
		}
		if err != nil {
			return nil, failCode(kindNotReport, "xml")
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch len(path) {
			case 2: // FlexQueryResponse > FlexStatements > FlexStatement
				if t.Name.Local == "FlexStatement" {
					st, err := startStatement(t)
					if err != nil {
						return nil, err
					}
					out = append(out, st)
					cur = &out[len(out)-1]
				}
			case 3:
				section = t.Name.Local
			case 4:
				if cur != nil {
					if err := addRow(cur, section, t); err != nil {
						return nil, err
					}
				}
			}
			path = append(path, t.Name.Local)
		case xml.EndElement:
			path = path[:len(path)-1]
			if len(path) == 2 {
				cur = nil
			}
		}
	}
}

func startStatement(t xml.StartElement) (sources.FlexStatement, error) {
	a := attrs(t)
	if a["accountId"] == "" {
		return sources.FlexStatement{}, failCode(kindNotReport, "statement")
	}
	return sources.FlexStatement{
		AccountID: a["accountId"], FromDate: a["fromDate"], ToDate: a["toDate"],
		Period: a["period"], WhenGenerated: a["whenGenerated"],
	}, nil
}

// addRow files one row of a statement section: an EXECUTION Trade is kept, any other row is counted.
func addRow(cur *sources.FlexStatement, section string, t xml.StartElement) error {
	if section != "Trades" || t.Name.Local != "Trade" {
		cur.OtherLevels++
		return nil
	}
	a := attrs(t)
	if a["transactionID"] == "" || a["levelOfDetail"] == "" {
		return failCode(kindNotReport, "trade")
	}
	if acct, ok := a["accountId"]; ok && acct != cur.AccountID {
		return failCode(kindNotReport, "trade")
	}
	if a["levelOfDetail"] != levelExecution {
		cur.OtherLevels++
		return nil
	}
	cur.Trades = append(cur.Trades, a)
	return nil
}
