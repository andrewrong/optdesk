package optionledger

import "github.com/andrewrong/optdesk/internal/jobs"

// Types of the optionledger job; each name is an alias of the one in package jobs.
type (
	OptionLedgerStores       = jobs.OptionLedgerStores
	FlexRun                  = jobs.FlexRun
	OptionLedgerConfig       = jobs.OptionLedgerConfig
	NewOptionLedgerJobFunc   = jobs.NewOptionLedgerJobFunc
	ImportOptionTraderFunc   = jobs.ImportOptionTraderFunc
	FlexAttributesSHA256Func = jobs.FlexAttributesSHA256Func
)
