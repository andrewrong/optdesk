package cutover

import (
	"context"
	"io"

	"github.com/andrewrong/optdesk/internal/contracts"
	"github.com/andrewrong/optdesk/internal/store"
)

// VerifyAccount checks account.jsonl against the store and changes nothing. A line whose alias and day have
// no stored capture is Missing; one whose stored hash differs is Different. The file is checked as in
// ImportAccount. It implements VerifyAccountFunc.
func VerifyAccount(ctx context.Context, r io.Reader, rd store.AccountReader, aliasMap map[string]string, through contracts.Day) (VerifyResult, error) {
	entries, err := loadAccount(r, aliasMap, through)
	if err != nil {
		return VerifyResult{}, err
	}
	res := VerifyResult{Lines: len(entries)}
	for _, e := range entries {
		if err := ctx.Err(); err != nil {
			return res, err
		}
		h, found, err := storedHash(ctx, rd, e.capture.Alias, e.capture.SessionDay)
		if err != nil {
			return res, err
		}
		switch {
		case !found && e.empty:
		case !found:
			res.Missing++
		case h != e.hash:
			res.Different++
		}
	}
	return res, nil
}
