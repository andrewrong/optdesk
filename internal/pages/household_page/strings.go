package household_page

var (
	headsComposition   = []string{headPosition, headAmount, headShare}
	headsConcentration = []string{headCompany, headHouseholdShare, headMarketValue}
)

var accountColumns = []string{headAccount, headNet, headCash, headStocks, headOptions, headDataDay}

// gapTexts are the words of each gap reason a line of pagefig can carry.
var gapTexts = map[string]string{
	"missing_quote":        textMissingQuote,
	"missing_input":        textMissingInput,
	"unsupported_currency": textUnsupportedCurrency,
	"no_data":              textNoData,
	"not_built":            textNotBuilt,
	"no_reader":            textNoReader,
	"stale_input":          textStale,
}

func gapText(reason string) string {
	if t, ok := gapTexts[reason]; ok {
		return t
	}
	return textNoDataYet
}
