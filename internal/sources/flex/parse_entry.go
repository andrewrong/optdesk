package flex

import (
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"io"

	"github.com/andrewrong/optdesk/internal/sources"
)

var _ sources.ParseFlexFunc = Parse

func badGzip() error { return failCode(kindNotReport, "gzip") }

// Parse reads a Flex body, plain or gzip (a body that starts with 1f 8b), and parses it with the
// parser of the client. The limit of 50 MB is on the decompressed bytes; RawSHA256 is the hash of
// the decompressed bytes; FetchedAt is left for the caller.
func Parse(r io.Reader) (sources.FlexReport, error) {
	br := bufio.NewReader(r)
	var src io.Reader = br
	if magic, _ := br.Peek(2); len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		zr, err := gzip.NewReader(br)
		if err != nil {
			return sources.FlexReport{}, badGzip()
		}
		defer func() { _ = zr.Close() }()
		src = zr
	}
	body, err := io.ReadAll(io.LimitReader(src, maxBody+1))
	if err != nil {
		return sources.FlexReport{}, badGzip()
	}
	if len(body) > maxBody {
		return sources.FlexReport{}, failCode(kindTooLarge, "")
	}
	stmts, err := parseReport(body)
	if err != nil {
		return sources.FlexReport{}, err
	}
	sum := sha256.Sum256(body)
	return sources.FlexReport{RawSHA256: hex.EncodeToString(sum[:]), Statements: stmts}, nil
}
