package optioncandidates

import (
	"testing"

	"github.com/andrewrong/optdesk/internal/contracts"
)

func TestCandidatesSkipNoQuote(t *testing.T) {
	t.Parallel()
	e := newSrcEnv()
	e.underlying(7, "ABC")
	for id := int64(11); id <= 13; id++ {
		e.put(7, id, "90")
	}
	noQuote := func(id int) contracts.OptionOpen {
		r := openRowOf(7, id, 3, 0, 0)
		r.ErrorCode = new("NO_QUOTE")
		return r
	}
	e.open = []contracts.OptionOpen{openRowOf(7, 11, 1, 2.0, 2.1), noQuote(12), noQuote(13)}
	e.run(t, true, entryFor("ABC", "cik:1", "91.5"))
	in := e.inputOf("ABC")
	if in.ChainDay != runDay || len(in.Quotes) != 1 || in.Quotes[0].InstrumentID != "11" || !in.Quotes[0].QuoteOK {
		t.Fatalf("chain day %s quotes %+v, want the one ok put of the open chain", in.ChainDay, in.Quotes)
	}
}
