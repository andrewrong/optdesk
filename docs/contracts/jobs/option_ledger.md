# option_ledger

## Option ledger

- `optionledger.FlexAttributesSHA256Func` (implemented by `internal/jobs/optionledger.AttributesSHA256`, shared by the `option_ledger` job and the option-trader import): remove the keys `accountId` and `acctAlias`; encode the rest as one JSON object, keys ascending by byte order, no whitespace, strings in UTF-8 with only `"`, `\` and control characters below 0x20 escaped and no HTML escaping; the hash is the lower-case hex SHA-256 of that text. Control characters are written `\b \t \n \f \r` or `\u00xx`. `internal/jobs/optionledger/testdata/attrs_hash.json` lists `{attrs, canonical, sha256}` entries.
- `optionledger.FlexRun` carries `Required`; a login whose token variable is empty reaches the job with a nil `Source`. The constructor accepts it and never fails for a missing token; `Run` returns a failed result when the login is `Required`, else skips it with the `degraded` log record.
- Contract tests: `internal/jobs/optionledger/types_test.go` `TestOptionLedgerJobContract`, `TestOptionLedgerImportContract`; `internal/jobs/optionledger/hash_test.go` `TestAttributesSHA256Fixture`, `TestAttributesSHA256Ignores`, `TestAttributesSHA256KeyOrder`.

## Version history

- v1: `option_ledger`, interface only (`optionledger.OptionLedgerStores`, `optionledger.OptionLedgerConfig`, `optionledger.FlexRun`, `optionledger.NewOptionLedgerJobFunc`, `optionledger.ImportOptionTraderFunc`, `jobs.ImportResult`, `optionledger.FlexAttributesSHA256Func`), with `optionledger.AttributesSHA256`.
- v1: `optionledger.OptionLedgerStores.Sections` (`flexsection.Writer`, additive; `docs/design/account-flex-fx.md`): nil switches the Flex section step off, and the other fields and `optionledger.NewOptionLedgerJobFunc` are unchanged. Delivery rule (Decision 10): nothing the section step handles reaches notify; a job result or `job_event` carries a job id, a status and an alias only, never a cash amount, balance, net value or position. Test: `internal/jobs/optionledger/types_test.go` `TestOptionLedgerStoresSectionsSet`, `TestOptionLedgerStoresSectionsNil`.
