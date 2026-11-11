# Flex XML import: the owner uploads downloaded IBKR Flex XML files on a web page and their trades are added to the option ledger by the daily job's parser, mapping and store

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 2 (round 1 of the review is folded in and answered in `## Adversarial`)

Source: owner ask optdesk-2875 (2026-11-11): the pre-2025-12-01 trade history will be exported from IBKR as Flex Query XML (the query of the option_ledger job, a custom date range, at most 365 days per file); same parsing, same account mapping, same append-only storage as the daily job; a re-import changes nothing; an unmapped account is counted, not stored; the daughter's account arrives as a second statement of the main login and maps once `flex.toml` lists its alias (after the registered alias-pattern issue). Owner, 2026-11-11 ~00:05Z: he will not use a terminal ("we are a product; I click on the web page"), so the import is a page of `ot serve`. Coordinator, same time: the run-one-job-for-one-day button is not in this design (`## Left out`).
Status: draft. Read on origin/main at 1196872127c1. "[unverified]" marks a claim not checked in code, docs or a probe.

Goal. On `/flex-import` of `ot serve --owner-edit` the owner chooses one or more `.xml` or `.gz` files, presses one button, and the page shows, per file, one of five statuses (stored, already present, nothing found, account not configured, failed) with the counts per alias, and, after a reload, the recent imports and their failures. The trades are parsed, mapped and stored by the daily job's code. The import only adds: a trade whose key is already stored is left as it is and counted as already present. No terminal step.

## What main holds today

- The daily job reaches a report through `FlexSource.Fetch` (`internal/jobs/optionledger/job.go:293`). `parseReport` (`internal/sources/flex/parse.go:119`, called at `client.go:162`) is unexported and takes the whole body as `[]byte`; `maxBody` is 50 MiB (`client.go:24`); `prelude` (`parse.go:17`) refuses a DTD or an entity. `RawSHA256` is the SHA-256 of the raw body (`client.go:166`). Only `<Trade>` rows of level `EXECUTION` are kept; other levels are counted and dropped (`parse.go:12` and the comment at `:115-118`). The raw XML is not kept (`option-ledger.md` Decision 5).
- `mapStatement` (`internal/jobs/optionledger/map.go:19`) is unexported; it maps an account number to its alias through `OptionLedgerConfig.Accounts` and reports an unmapped account as `ok` false.
- `RecordFlexFetch(ctx, rec, rows)` (`internal/store/api.go:673`) is one transaction, with the revision rule per row (a new key is New, an equal hash is Unchanged, a changed hash is written as the next revision, Revised). The table `optionledger.flex_fetch` has the primary key `fetch_id` only (`migrations/0022_optionledger.sql:11-30`), and `fetchID` is `Login/QueryID/StartedAt` with no alias (`internal/store/optionledger/store.go:263-265`); `insertFetchSQL` is `ON CONFLICT DO NOTHING` and a count other than 1 is an error that rolls the transaction back (`store.go:119`). So two records with the same login, query id and start instant cannot both be written.
- The view `rd.flex_fetch_r1` shows every record, failed ones too, and `ot_ro_pages` and `ot_w_optionledger` can read it (`0022:72` and `:77`). The only read of it in Go is `LatestOkFetch` (status ok only; `store.OptionLedgerReader` has `FlexTradeRows`, `LatestOkFetch`, `StoredAliases`). `FlexTradeRows(alias)` returns the highest revision of every transaction of the alias. The job's window reads `LatestOkFetch(login, queryID)` (`job.go:267`); a record under another login leaves the window untouched.
- `internal/store/api.go` has 800 lines and is not in `docs/design/ratchet.json`; a line added there needs a ceiling, which an agent never writes. So the new read below is declared in a package of its own.
- `cmd/ot/optionledger.go:162` `flexConfig` builds `Accounts` (account number to alias) from `flex.toml` and the env file; the number never leaves it. This design does not read `flex.toml` itself.
- `ImportOptionTrader` (`importfile.go:47`) and `cmd/ot/ledgerimport.go` are the older file import, for option-trader's JSON Lines; they stay.
- `ot serve` writes through one pattern today: `--owner-edit`, a loopback `--addr` (`serve.go:46` and `:52-53`), `OT_FORM_KEY` (at least 32 bytes) and the settings DSN (`newOwnerEdit`, `settings.go:53`, through `ownerEditSecrets`, `env.go:70`), a hidden `expires` and `mac` token on the page, `guardOwnerPost` (`settings.go:245`: Host, 64 KiB cap, `settingsform.Check`: remote address, Host, Origin, content type `application/x-www-form-urlencoded`, 64 KiB, MAC, expiry), and a store opened per post (`underlyings_post.go:116-120`). The guard refuses a multipart body, so it cannot carry an upload as it is.
- The page parts of a page are: a schema `contracts/schemas/page.<name>.vN.json`, a generated type `web/src/generated/<name>.vN.ts`, a builder in `internal/pages/<name>/`, a route in `cmd/ot`, a view in `web/src/pages/<name>/` (`docs/design/underlyings.md`, entries 3, 4, 8 and 11, are the model). `rules/frontend-stack.md` does not exist on main; the rule is `docs/adr/0034-frontend-stack.md`, which the UI lines below cite.
- The pages that read the stored trades are `positions_page`, `risk_page`, `household_page`, `option_result_page`, `home_options`, `option_tracking_page` and `report_blocks` (`FlexTradeRows`). The Flex sections are written by the job (`sections.go`, `flexsection`) and no page or compute package on main reads them.

## Decisions

1. **One parse entry, shared.** `sources.ParseFlexFunc` (`func(r io.Reader) (FlexReport, error)`) is declared in `internal/sources/api.go`; its implementation is `flex.Parse` in `internal/sources/flex`, a new file that calls the same `parseReport` and `prelude`. `cmd/ot` is the composition root and imports `internal/sources/flex` and `internal/jobs/optionledger` directly in its new files; `cmd/ot/wire.go` (797 lines) is not touched and the deps struct gets no field. `flex.Parse` reads at most `maxBody` bytes after decompression; a body that starts with the gzip magic `1f 8b` is decompressed first (this is what makes option-trader's raw archive, one gzip per report, importable by upload: a `.gz` costs a few lines here, so it is in). `RawSHA256` is the hash of the decompressed bytes, so a file and its gzip have one hash, and it equals the daily job's hash of the same body. `FetchedAt` is left zero; the caller sets it. A body over the limit is `too_large`, a non-report is `not_report`: the existing kinds, no text carries a value. `client.go` is not changed.
2. **One mapping, add-only storage.** `optionledger.ImportFlexReport` (a new file in `internal/jobs/optionledger`) takes the `FlexReport`, the `Accounts` map, a `store.OptionLedgerWriter`, a `store.OptionLedgerReader`, the file's hash text and ordinal, and `now`. It maps every statement with `mapStatement`, groups the rows by alias, and for each alias (sorted) reads `FlexTradeRows(alias)`. A row whose key (level of detail, transaction id) is stored, or repeats an earlier row of the same report, is already present and is not passed on; the rest is written by one `RecordFlexFetch`, so each of those rows is a key that was absent and is New. The import never writes a revision: whatever the field set or hash of the stored row, a stored key is left as it is. There is no date rule: the 2025-12-01 boundary is not checked, the days of the file only fill the record's first and last day (an unreadable period leaves them empty, it does not refuse the file). The record is `Login: "import"`, `QueryID: "file:" + first 12 hex of the hash + ":" + alias`, `Status` ok, `ParserVersion` the job's `parserVersion`, `Statements` the statements of that alias, `RowsParsed` the mapped rows of the alias, `RowsNew` and `RowsUnchanged` the New and the already present (`RowsRevised` 0). `StartedAt` is `now` plus one microsecond per file ordinal and one nanosecond per alias index. Together with the query id this makes `fetch_id` differ for every alias of a file, for every file of a post (two files with the same bytes included) and for every later import. No account number is in the key: only the alias, which the store already holds. Unmapped statements are counted into the record of the first alias in sorted order, or into one record `file:<hash>:-` without rows when no alias matched. A file that finds no trade row and no unmapped statement is also one record `file:<hash>:-` (status `nothing_found` on the page).
3. **A failure is a record.** An import is not a runner job and has no job result, so its only trace is `flex_fetch`. A refused or failed file writes one record with `Status` failed, `ErrorKind` and `ErrorCode` a fixed code (`not_report`, `too_large`, `too_many_files`, `bad_gzip`, `bad_row`, `store_error`; never a value) and the query id `file:<hash>:-` (`file:-:-` when no hash was computed). A store error on a record is written best-effort: if even that write fails, the page of that response shows `failed` / `store_error` and nothing remains to reload, which the page text says (Doubt 2). The page after a reload reads the newest 20 records of the login `import` from the view (Decision 8, entry 3) and shows each by the same five statuses, so a failure is visible without the POST response.
4. **Idempotence stated exactly.** A re-import of the same file writes no trade row: every key is already present. It appends one `flex_fetch` record per alias (the fetch table is a log, as for every daily run). No file name is stored (the record has no column for it): the recent list shows the time, the alias, the status and the counts.
5. **Account mapping is the job's.** The `Accounts` map comes from one function extracted out of `flexConfig` (`flexAccounts`) and used by both; so the alias-pattern issue, when it lands, changes the map for both. The daughter's alias maps when `flex.toml` lists it; before that her statement is counted as unmapped and the page says "account not configured by the alias rule". An account number never reaches the page, a log line, an error text or a query id.
6. **Who writes (second opinion).** The upload route is the first path by which `ot serve` holds the option-ledger writer: the role `ot_w_optionledger` (INSERT and SELECT only, no UPDATE or DELETE), opened per post from the env file and closed after it, never kept open. The post route exists where the two settings posts exist: `rd.edit` is set, which `newOwnerEdit` does only with `--owner-edit`, a loopback address, `OT_FORM_KEY` and `OT_PG_DSN_SETTINGS` (`ownerEditSecrets` knows nothing of the ledger line; `env.go` and `settings.go` are not changed). The ledger line is checked by the new route's own code: the GET builder shows `form` null with one gap line when `OT_PG_DSN_OPTION_LEDGER` is unset or `flex.toml` does not load, and the post answers the same refusal without opening a store. The GET reads the recent records with the read-only pages line `OT_PG_DSN_PAGES` (role `ot_ro_pages`), not with the writer. No new `.env` line is needed on the owner's machine, which has all of these lines. No new library: `mime/multipart`, `compress/gzip` and `encoding/xml` are the standard library.
7. **Guard for an upload.** `settingsapi` gains a second interface `UploadChecker` with `CheckUpload(r FormRequest, maxBody int64, now time.Time) error`: the checks of `Check` (remote, Host, Origin, token, expiry) with the content type `multipart/form-data` and the cap given instead of 64 KiB. `settingsform` implements it on the same value; `cmd/ot` reaches it by a type assertion on `rd.edit.form`, so no existing fake of `SettingsForm` changes. The token is the one of the page (`Token(now)`), which is also a valid settings token (Adversarial, token point).
8. **The read of past imports.** `optionledger.ImportLog` (a new `internal/store/optionledger/api.go`, declarations only) has `RecentFetches(ctx context.Context, login string, limit int) ([]store.FlexFetchRecord, error)`, newest first by finish time, from `rd.flex_fetch_r1`; the store type of `internal/store/optionledger` implements it in a new file and `cmd/ot` reaches it by a type assertion on the opened store, as for `UploadChecker`. No migration, grant or view changes (the view and both grants exist). The store row of `docs/design/modules.md` does not list this file as an interface file; adding it is an edit of an owner-only file, named in Doubt 4.
9. **Limits.** All three are chosen by the designer, not measured on a real 365-day export (Doubt 6); each is enforced in one place and refuses with one fixed text, and the other files of the post go on.

| limit | value | why | enforced in | refusal |
|---|---|---|---|---|
| one file, after decompression | 50 MiB | the live fetch's own bound (`maxBody`), so the import accepts what the job accepts; the parser holds a file and an upper-case copy in memory | `flex.Parse` (entry 1), `LimitReader` of `maxBody+1` behind the gzip reader | the file is `failed`, reason `too_large`: "file too large: at most 50 MiB after decompression" |
| one post | 64 MiB | files are parsed one at a time, so memory is one file and its copy, not the post; 64 MiB is above one file and below eight | `cmd/ot` before the guard (declared length over the cap or unknown: 413) and `http.MaxBytesReader` while reading (413); `CheckUpload` refuses the same lengths | 413, no page: "upload too large: at most 64 MiB" |
| files per post | 8 | eight years of 365-day files; more is a second post | `cmd/ot`, the ninth part is not read | one entry `(more files)` `failed`, reason `too_many_files`: "too many files: at most 8 per upload" |

10. **The page.** `page.flex_import.v1`: `title`, `sentence`, `scope_note`, `form` (null when the route is off; otherwise `post_path` `/owner-flex-import`, `expires`, `mac`, `max_files`, `max_file_text`), `gaps`, `files` and `recent`. Per file in `files`: `name` (the base name, at most 80 characters, shown only in the POST response), `status` (one of `stored`, `present`, `nothing_found`, `unconfigured`, `failed`), `reason` (the fixed codes of Decision 3 for `failed`, empty otherwise), `status_text`, `period_first`, `period_last`, `aliases` (alias, statements, rows_parsed, rows_new, rows_present, all integers) and `unmapped_statements`. The file status is the first that applies: `failed`; `stored` when any alias has rows_new above 0; `present` when any alias has rows_parsed above 0; `unconfigured` when only unmapped statements were found; else `nothing_found`. Per record in `recent`: `at_text`, `alias`, `status` and `reason` by the same rule from the record (`failed` status; `RowsNew` above 0; `RowsUnchanged` above 0; unmapped above 0; else nothing), the counts and `unmapped_statements`. `scope_note` is one fixed line: "Only trades of level EXECUTION are read, from the same Flex query that option_ledger uses for trades (the query named in its trade queries); cash, positions, NAV and the other sections of the file are not imported." Every sentence is built in Go; the view computes nothing. A GET shows the form, an empty `files` and the `recent` list; the POST answers with the page carrying the results (200, no redirect: the results are in `recent` after a reload).
11. **No CLI.** `ot flex import FILE...` would be a second front for the same calls; it costs a command file, a test and a second opener path. It is under `## Left out`.

## Modules

No new row. First matching glob of `docs/design/modules.md`.

| module | paths touched | change |
|---|---|---|
| sources (interface entry 1) | `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/flex/parse_entry.go`, `internal/sources/flex/parse_entry_test.go` | `ParseFlexFunc` and `flex.Parse` |
| compute (interface entry 2) | `internal/compute/settingsapi/api.go`, `internal/compute/settingsapi/api_test.go`, `internal/compute/settingsform/upload.go`, `internal/compute/settingsform/upload_test.go`, `docs/contracts/compute.md` | `UploadChecker` and its implementation |
| store (interface entry 3) | `internal/store/optionledger/api.go`, `api_test.go`, `recent.go`, `recent_test.go`, `import_key_pg_test.go`, `docs/contracts/store.md` | `ImportLog` and the real-key test |
| jobs | `internal/jobs/optionledger/importflex.go`, `internal/jobs/optionledger/importflex_test.go` | `ImportFlexReport` |
| contracts (interface entry 5) | `contracts/schemas/page.flex_import.v1.json`, `web/src/generated/flex_import.v1.ts`, `contracts/fixtures/**`, `internal/contracts/flex_import_v1_test.go`, `docs/contracts/page-schemas.md` | the page schema and its type |
| pages | `internal/pages/flex_import/**`, `docs/contracts/pages.md` | the builder |
| ot | `cmd/ot/flex_import_site.go`, `cmd/ot/flex_import_post.go`, their tests, `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/flex_import_page.md` | extraction, route, post |
| web | `web/src/pages/flex_import/view.tsx`, `web/src/types/flex_import.ts`, `web/test/pages/flex_import/view.test.tsx` | the view |

## Interface

- Entry 1: `sources.ParseFlexFunc` in `internal/sources/api.go`, declarations only; the implementation is named `flex.Parse`. `api_test.go` tests it from both sides (a fake used by a caller; a caller of the real function over a body, a gzip of it, and a body over the limit).
- Entry 2: `settingsapi.UploadChecker`, declarations only; `api_test.go` has a fake and a caller, and checks that the real form value satisfies it.
- Entry 3: `optionledger.ImportLog` in `internal/store/optionledger/api.go`, declarations only; `api_test.go` has a fake used by a caller and a caller of the real store through the interface.
- Entry 5: `page.flex_import.v1` and the generated type `FlexImport`.
- Jobs: `optionledger.ImportFlexReport(ctx context.Context, w store.OptionLedgerWriter, rd store.OptionLedgerReader, accounts map[string]string, rep sources.FlexReport, file FlexFileRef, now time.Time) (FlexImportSummary, error)` and `optionledger.RecordImportFailure(ctx context.Context, w store.OptionLedgerWriter, file FlexFileRef, code string, now time.Time) error`; `FlexFileRef` (hash text, ordinal) and `FlexImportSummary` (per alias counts, `Unmapped`, `First`, `Last`) are types of the package, not of `internal/jobs/api.go` (only `cmd/ot` uses them).

"Landed" means: the entry's pull request is merged to main and `git grep` of its declaration on origin/main finds it; an entry that uses it is cut only after.

## Issues

Eight entries. 1, 2, 3 and 5 are interface entries and land before the entries that use them. Each lists every file it touches and builds alone on main plus the entries it is ordered after; no two entries share a file. Acceptance states results checkable with git or by reading code; test runs and pull request text are under `## Checked by CI`.

### 1 flex-parse-entry (interface, sources)

Module: sources. Size: M.
Files: `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/flex/parse_entry.go`, `internal/sources/flex/parse_entry_test.go`
Design: docs/design/flex-xml-import.md#Decisions (1)
Contracts: unchanged
Order: first.
Goal: a caller outside the package parses a Flex body, plain or gzip, with the parser of the client.
Acceptance:
1. `git grep -h -c "type ParseFlexFunc func" internal/sources/api.go` prints 1; `git rev-parse --short=12 HEAD:internal/sources/flex/client.go` prints de78d626db08 (unchanged) and `git grep -h -c "parseReport(" internal/sources/flex/parse_entry.go` prints 1 (the entry calls the one parser, it does not copy it); `git grep -h -c "^func Parse(" internal/sources/flex/parse_entry.go` prints 1.
2. `parse_entry.go` reads through `io.LimitReader` of `maxBody+1` after the gzip reader, so the limit is on decompressed bytes, and returns the `too_large` kind above it (`git grep -n "LimitReader" internal/sources/flex/parse_entry.go` prints a line after the line that makes the gzip reader); a body starting with bytes 1f 8b is decompressed, any other is passed on.
3. `RawSHA256` is computed over the decompressed bytes with `sha256.Sum256`, as `client.go:166` does; `FetchedAt` is not set in the file (`git grep -h -c "FetchedAt" internal/sources/flex/parse_entry.go` prints 0).
4. `parse_entry_test.go` has a case for each of: plain body, the same body gzipped (equal `RawSHA256`), a gzip that is not valid, a body over the limit, a body with a DTD (refused by `prelude`), and a body that is not a report.
5. `wc -l` of each new file prints under 600; each new test calls `t.Parallel()` and never sleeps.

### 2 upload-guard (interface, compute)

Module: compute. Size: S.
Files: `internal/compute/settingsapi/api.go`, `internal/compute/settingsapi/api_test.go`, `internal/compute/settingsform/upload.go`, `internal/compute/settingsform/upload_test.go`, `docs/contracts/compute.md`
Design: docs/design/flex-xml-import.md#Decisions (7)
Contracts: unchanged
Order: first.
Goal: the guard of the settings form can check a multipart upload under a cap the caller names.
Acceptance:
1. `git grep -h -c "type UploadChecker interface" internal/compute/settingsapi/api.go` prints 1; the `SettingsForm` interface is unchanged (`git grep -h -c "CheckUpload" internal/compute/settingsapi/api.go` prints 1, in `UploadChecker` only).
2. `git rev-parse --short=12 HEAD:internal/compute/settingsform/guard.go` prints 07ac65d12acf (the existing `Check` is unchanged); `upload.go` calls the host, token and signing helpers of `guard.go` and `token.go` and does not copy them (`git grep -h -c "loopbackName\|parseToken\|f.sign" internal/compute/settingsform/upload.go` prints at least 3).
3. `upload.go` refuses a content type other than `multipart/form-data` and a size below 0 or above the argument; `upload_test.go` has a case for each of: remote address, Host, Origin, wrong content type, size over the cap, bad token, expired token, token ahead, and a valid upload.
4. `git grep -h -c "UploadChecker" docs/contracts/compute.md` prints at least 1.

### 3 import-log (interface, store)

Module: store. Size: M.
Files: `internal/store/optionledger/api.go`, `internal/store/optionledger/api_test.go`, `internal/store/optionledger/recent.go`, `internal/store/optionledger/recent_test.go`, `internal/store/optionledger/import_key_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/flex-xml-import.md#Decisions (2, 3, 8)
Contracts: unchanged
Order: first.
Goal: past imports, failed ones too, can be read, and the fetch key of an import record is proved on the real table.
Acceptance:
1. `git grep -h -c "type ImportLog interface" internal/store/optionledger/api.go` prints 1; `git rev-parse --short=12 HEAD:internal/store/api.go` prints 542bda2b86f9 and `wc -l < internal/store/api.go` prints 800 (no line added there); `git ls-files migrations | wc -l` is the same as on main at 1196872127c1 (no migration).
2. `recent.go` reads only `rd.flex_fetch_r1` (`git grep -h -c "rd.flex_fetch_r1" internal/store/optionledger/recent.go` prints 1 and `git grep -h -c "optionledger.flex_fetch" internal/store/optionledger/recent.go` prints 0) with a `WHERE login = $1` and an `ORDER BY finished_at DESC` and a `LIMIT`; `git rev-parse --short=12 HEAD:internal/store/optionledger/store.go` prints 86ff6d360341 (the writer and its key are unchanged).
3. `import_key_pg_test.go` carries the build tag `pg` (its first line is `//go:build pg`) and, on the real table, has a case for each of: two records of Login `import` with the query ids `file:<hash>:alias1` and `file:<hash>:alias2` and the same `StartedAt` are both stored; two records with the same query id and the start instants one microsecond apart are both stored; two records with the same login, query id and start instant make `RecordFlexFetch` return an error and store neither the record nor its rows (the collision of `fetch_id` the key avoids); a failed record with no rows is stored and `RecentFetches` returns it, newest first, with its error kind and code.
4. `recent_test.go` (no tag) has a case that `limit` caps the list, that a login with no record gives an empty slice, and that the fake used through `ImportLog` gives what the caller passed; each test calls `t.Parallel()` and `t.TempDir()` where a file is written, and never sleeps.
5. `git grep -h -c "ImportLog" docs/contracts/store.md` prints at least 1.

### 4 import-flex-report

Module: jobs. Size: M.
Files: `internal/jobs/optionledger/importflex.go`, `internal/jobs/optionledger/importflex_test.go`
Design: docs/design/flex-xml-import.md#Decisions (2, 3, 4)
Contracts: unchanged
Order: first (it takes `sources.FlexReport`, which is on main).
Goal: one report is mapped with the job's mapping and its new keys are added per alias, counted; a failure is a record.
Acceptance:
1. `git rev-parse --short=12 HEAD:internal/jobs/optionledger/map.go` prints b80eeed2759a (`mapStatement` is unchanged) and `git grep -h -c "mapStatement(" internal/jobs/optionledger/importflex.go` prints 1; `git grep -h -c "RecordFlexFetch(" internal/jobs/optionledger/importflex.go` prints 2 (one in the loop over aliases, one in `RecordImportFailure`).
2. `git rev-parse --short=12 HEAD:internal/store/api.go` prints 542bda2b86f9 and `git rev-parse --short=12 HEAD:internal/jobs/api.go` prints f34381afe1d5 (no store or jobs interface change).
3. The record carries `Login: "import"`, a `QueryID` built from `"file:"` and the alias or `-`, and the job's `parserVersion` (`git grep -h -c 'Login: *"import"' internal/jobs/optionledger/importflex.go` prints 1 or 2, and `git grep -h -c '"file:"' internal/jobs/optionledger/importflex.go` prints at least 1); `git grep -h -c "LatestOkFetch\|ImportFlexTradeRows" internal/jobs/optionledger/importflex.go` prints 0; the rows handed to `RecordFlexFetch` are the ones whose key `FlexTradeRows` did not return (`git grep -h -c "FlexTradeRows(" internal/jobs/optionledger/importflex.go` prints 1).
4. `importflex_test.go` has a case for each of: two aliases in one file give two records with different `fetch_id` inputs (query id and start instant) and per-alias counts that add up; the same report twice gives New 0 the second time and rows_present equal to the first time's New; an upload over stored rows with a different field set (another attribute or another value under the same key) writes no row, gives Revised 0 and counts the rows as already present; a key repeated inside one report is counted once as New and once as already present; an unmapped statement is counted and no row of it is written (a fake writer records every row it is given; no row, record or query id holds an `accountId`, an `acctAlias` or an account number); no alias mapped gives one record `file:<hash>:-` without rows and the count; a report with no trade and no unmapped statement gives one such record with all counts 0; a period that cannot be read leaves the record's days empty and writes the rows; a writer error returns `store_error` and `RecordImportFailure` writes a failed record with a fixed code and no value of the file.
5. `wc -l` of each new file prints under 600; each test calls `t.Parallel()`, uses a fake writer, a fake reader and a fake clock, and never sleeps.

### 5 flex-import-page-schema (interface, contracts)

Module: contracts (the generated type under `web/src/generated/**` is a path of contracts). Size: M.
Files: `contracts/schemas/page.flex_import.v1.json`, `web/src/generated/flex_import.v1.ts`, `contracts/fixtures/page.flex_import.v1.golden.json`, `contracts/fixtures/page.flex_import.v1.result.json`, `contracts/fixtures/page.flex_import.v1.disabled.json`, `contracts/fixtures/page.flex_import.v1.bad-version.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/flex_import_v1_test.go`, `docs/contracts/page-schemas.md`
Design: docs/design/flex-xml-import.md#Decisions (10)
Contracts: page.flex_import v1 (new)
Order: first.
Goal: the schema, the type `FlexImport` and four fixtures exist and agree.
Acceptance:
1. The schema has `"$id": "https://optdesk.invalid/schemas/page.flex_import.v1.json"`, `"const": "flex_import"` for `schema`, `"const": 1` for `schema_version`, `additionalProperties: false` on every object, the fields of Decision 10, `status` (in `files` and in `recent`) an enum of exactly `stored`, `present`, `nothing_found`, `unconfigured` and `failed`, and `reason` an enum of exactly `not_report`, `too_large`, `too_many_files`, `bad_gzip`, `bad_row`, `store_error` and the empty text; the counts are `integer` with `minimum` 0.
2. The title is `FlexImport`; the generated file is the output of `node scripts/generate-types.mjs page.flex_import.v1` run from `web/`: line 1 is the banner and `git grep -h -c "export interface FlexImport " web/src/generated/flex_import.v1.ts` prints 1; no `web/src/generated/index.ts` exists (`git ls-files web/src/generated/index.ts` prints no line) and `git rev-parse --short=12 HEAD:web/scripts/generate-types.mjs` prints 3551fad4871b.
3. The fixtures: `golden` has the form, no file and one `recent` item; `result` has one `stored` file with two aliases and an unmapped count, one `present` file, one `nothing_found` file, one `unconfigured` file and one `failed` file with `not_report`, and a `recent` list of two items; `disabled` has `form` null and one gap; `bad-version` is the golden with `schema_version` 2. Invented aliases only; `grep -c "@"` of each fixture prints 0. `MANIFEST.synthetic-v1.json` gains the four paths and loses none.
4. `TestFlexImportV1Fixtures` validates golden, result and disabled, shows bad-version fails at `schema_version`, and inline mutations (unknown status, unknown reason, negative count, extra top-level field) each fail.
5. `docs/contracts/page-schemas.md` has a row `| page.flex_import.v1 | 1 |`; `git rev-parse --short=12 HEAD:contracts/schemas/carryover.sha256` is the same as on main at 1196872127c1.

### 6 flex-import-builder

Module: pages. Size: S.
Files: `internal/pages/flex_import/builder.go`, `internal/pages/flex_import/page.go`, `internal/pages/flex_import/builder_test.go`, `internal/pages/flex_import/page_test.go`, `docs/contracts/pages.md`
Design: docs/design/flex-xml-import.md#Decisions (10)
Contracts: unchanged
Order: after 5.
Goal: `Build` turns an input of results and recent records into a valid `page.flex_import.v1`, every sentence in Go.
Acceptance:
1. `git grep -h -c 'Schema: *"page.flex_import.v1"' internal/pages/flex_import/page.go` prints 1; `Input` holds the form (token fields, nil for off), the gaps, the file results and the recent items as plain values; the builder imports no store row type and no `internal/jobs` (`git grep -h -c "internal/jobs\|internal/store/" internal/pages/flex_import/builder.go` prints 0).
2. The file name is cut to its base name and at most 80 runes in the builder (`git grep -h -c "filepath.Base" internal/pages/flex_import/builder.go` prints 1); the sentence of a status and of a reason is chosen from a closed table keyed by status or reason, none built from input text; `scope_note` is one constant that holds the words `EXECUTION`, `option_ledger` and `not imported`.
3. `builder_test.go` builds the golden, result and disabled fixtures of entry 5 from inputs and compares the JSON byte for byte, and has a case for the order of the file status (`failed` before `stored` before `present` before `unconfigured` before `nothing_found`); `page_test.go` has a case that `Build` of the registered `Page()` is the read-only page; an unknown status or reason is an error.
4. `wc -l` of each new file prints under 600; each test calls `t.Parallel()`.

### 7 ot-flex-import-route

Module: ot. Size: L.
Files: `cmd/ot/flex_import_site.go`, `cmd/ot/flex_import_post.go`, `cmd/ot/flex_import_site_test.go`, `cmd/ot/flex_import_post_test.go`, `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/flex_import_page.md`
Design: docs/design/flex-xml-import.md#Decisions (2, 3, 5, 6, 7, 9)
Contracts: unchanged
Order: after 1, 2, 3, 4, 6.
Second opinion: a new write path of `ot serve` under the option-ledger role, with a multipart body. Questions: (a) the post route exists only where `rd.edit` is set (`--owner-edit`, loopback, key and settings DSN) and writes only when the ledger line is set, and a failed `CheckUpload` writes nothing; (b) the writer is opened per post and closed after it, the recent list is read with the read-only pages line; (c) no account number, token, DSN or file content reaches a log line, an error text, a query id or the page; (d) the body, file count and decompressed size are capped as Decision 9 says.
Goal: `GET /flex-import` shows the form and the recent imports, and `POST /owner-flex-import` imports the files and answers with the page of results.
Acceptance:
1. `git rev-parse --short=12 HEAD:cmd/ot/wire.go` prints f399107f1f80 (the deps struct is not touched; the file has 797 lines on main), `git rev-parse --short=12 HEAD:cmd/ot/ledgerimport.go` prints ba5f49154fec, `git rev-parse --short=12 HEAD:cmd/ot/env.go` prints 8f16c2a1cfe8 and `git rev-parse --short=12 HEAD:cmd/ot/settings.go` prints 9fd77749add2.
2. `git grep -h -c "flexAccounts(" cmd/ot/optionledger.go` prints 2 (the definition and the call in `flexConfig`) and `git grep -h -c "flexAccounts(" cmd/ot/flex_import_post.go` prints 1; `git grep -h -c "flex.toml" cmd/ot/flex_import_post.go` prints no line (the file is read through `loadFlexFile` of deps and `flexFileName`).
3. `site.go` routes the path `owner-flex-import` only when `rd.edit != nil`, as the two routes beside it; any method but POST on it is 405 with `Allow: POST`; `flex_import_site.go` reads `store.DSNEnvOptionLedger` itself (`git grep -h -c "DSNEnvOptionLedger" cmd/ot/flex_import_site.go` prints at least 1) and opens the recent list with `store.DSNEnvPages` (`git grep -h -c "DSNEnvPages" cmd/ot/flex_import_site.go` prints at least 1).
4. In `flex_import_post.go` the check of the declared length against 64 MiB is on an earlier line than the call of `CheckUpload`, and that call on an earlier line than the first read of a part (`MultipartReader` or `NextPart`); the body is read under `http.MaxBytesReader` of 64 MiB; the ninth part is not read; the writer is opened with `openOptionLedgerStore` and the line `store.DSNEnvOptionLedger` per post and closed with `defer`; a failed guard answers 403 with a fixed text and a body over the cap 413, with the texts of the table of Decision 9.
5. Each file gets a hash from `flex.Parse` and an ordinal (`FlexFileRef`) before `ImportFlexReport`; a file refused or failed gets `RecordImportFailure`; a refused file never stops the others, and the writer is not opened when no file was parsed; `git grep -n "a.log\." cmd/ot/flex_import_post.go` shows only calls holding component, status and a file index (no name, alias, hash or number).
6. The tests of `flex_import_site_test.go` and `flex_import_post_test.go` have a case for each of: route off without `--owner-edit` (404), read-only page when a line is unset, a forged token (403, fake writer not called), a plain XML and the same gzipped (one hash), two files with one refused (the refused one is `failed` and its record is in the fake writer), the same bytes twice in one post (both stored as records, no collision), an unmapped account (`unconfigured`, no row stored), a report of no trades (`nothing_found`), the same file twice (second post: `present`, New 0), an upload over stored rows with another field set (Revised 0), an over-cap declared length and an over-cap stream (413), a file over the decompressed limit (`too_large`), nine files (`too_many_files`), a GET that lists the recent records including a failed one, and the page of the POST validated against the schema.
7. `check_serve.go`, `check_blocks.go` and `check_blocks.json` list the page as `underlyings` is listed (`git grep -h -c flex_import cmd/ot/check_serve.go` prints 1 and `git grep -h -c flex_import cmd/ot/check_blocks.json` prints 1); `wc -l` of each new file prints under 600 and of `site.go` under 800.

### 8 web-flex-import-view

Module: web. Size: M.
Files: `web/src/pages/flex_import/view.tsx`, `web/src/types/flex_import.ts`, `web/test/pages/flex_import/view.test.tsx`
Design: docs/design/flex-xml-import.md#Decisions (10)
Contracts: unchanged
Order: after 5.
Goal: a payload of `page.flex_import.v1` shows the upload form, the result tables and the recent imports, every string read from the payload.
UI parts (frontend-stack, ADR 0034):
- 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the page frame.
- 按 frontend-stack：用 shadcn `Input` (type file, `multiple`, `accept=".xml,.gz"`), `Label` and `Button` (already copied, no new package) 做 the upload as a plain `<form method="post" enctype="multipart/form-data">` with hidden `expires` and `mac`, no `fetch`.
- 按 frontend-stack：用 tanstack table (`web/src/components/data-table/data-table.tsx`) 做 the per-alias table of each file (alias, statements, rows parsed, new, already present) and the table of recent imports (time, alias, status, new, already present).
- 按 frontend-stack：用 shadcn `Badge` 做 the five statuses and `Alert` 做 the gaps, the failure reason, the unmapped count and `scope_note`.
- 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not used.
Acceptance:
1. `web/src/types/flex_import.ts` has `SUPPORTED_VERSIONS = [1]`, `export type Payload = FlexImport` from `@/generated/flex_import.v1` and `validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)`.
2. `view.tsx` exports `registration = { schema: 'flex_import', page: page<Payload>(validate, FlexImportView) }`.
3. The view shows `title`, `sentence`, `scope_note`, each file's name, `status_text`, reason and alias table, the unmapped count, the gaps and the recent table, the form only when `form` is not null, and formats nothing (`git grep -n "toFixed\|Intl\.\|toLocale" web/src/pages/flex_import/view.tsx` prints no line); the file input has `multiple` (`git grep -h -c "multiple" web/src/pages/flex_import/view.tsx` prints 1).
4. `view.test.tsx` reads the fixtures of entry 5 with `readFileSync`, checks each shown string is read from the fixture, that a `failed` file and a `nothing_found` file show their own status text, that the `disabled` payload renders no `<form>`, and that `validate` refuses `bad-version` with `unsupported_version`; it opens no port and never sleeps.

## Rollout

Entries 1 to 6 need no owner step. After entry 7 merges `ot serve` deploys itself with the other code merges; it runs with `--owner-edit` as for `/settings` (all lines exist). The owner then opens `/flex-import`. No migration, no new role, no new `.env` line. The owner's own work: export the XML files from IBKR with the Flex query that option_ledger uses for trades (365 days at most each) and choose them on the page.

## Checked by CI

`go test` of `internal/sources/...`, `internal/compute/settingsapi`, `internal/compute/settingsform`, `internal/store/optionledger` (the tag `pg` runs only in GitHub CI), `internal/jobs/optionledger`, `internal/contracts`, `internal/pages/flex_import`, `cmd/ot`; `tsc`, `eslint`, `vitest`, the build and the `type drift` step of the web workflow; `go run ./scripts/arch --check`; the ratchet; the `Contracts:` line check, which reads the body of the pull request of entry 5 (`Contracts: page.flex_import v1 (new)`).

## Left out

- **`ot flex import FILE...` on the command line.** Decision 11: a second front for the same calls, and not asked for after the owner's word. If wanted, it is one small command file that calls the same functions and prints the page's numbers.
- **Follow-up design `job-console`** (every job runnable alone from the web, idempotent; owner ask optdesk-2881): a button that runs one job for one day (what `ot backfill --day D --job J`, `cmd/ot/backfill.go`, does today) under the service env, with the result on the page. A separate audit is running for it; this design takes nothing from it and adds no runner call.
- **Flex sections, and what is empty because of it.** Cash, dividends, fees, open positions and NAV of the imported period are not stored, because they come from the sections of the report, which are parsed and dropped (the daily job stores them through `flexsection`). No page or compute package on main reads the sections (checked by `git grep` of `flexsection` and `FlexSection` outside store, sources and jobs: only `cmd/ot/optionledger.go`), so no page shows a gap today; a later reader would find none before the first day the daily job stored. The pages that read trades (`positions_page`, `risk_page`, `household_page`, `option_result_page`, `home_options`, `option_tracking_page`, `report_blocks`) see the imported trades like the job's. The page says so in `scope_note` and names the query to export with. If the old sections are wanted, a later entry adds one `RecordSections` call after `ImportFlexReport`.
- **Rows of other levels** (cancellations, corrections, closing and assignment lines that are not `EXECUTION` trades): `parseReport` keeps only `EXECUTION`-level `Trade` rows and counts the rest (`parse.go:12` and the comment at `:115-118`); the import does the same as the job. The page does not show the count of dropped rows.
- **A date rule.** The import does not check the 2025-12-01 boundary and never revises: a trade the daily job stored is left as it is. The consequence: a row stored by the import with another field set than the job's can later get a revision 2 from the daily job, when its window reaches that day (`history_from` is 2025-12-01 by default, so for a pre-2025-12-01 export this does not happen).
- **Bulk import of option-trader's raw archive** (`<raw root>/<sha[:2]>/<sha>.gz`, many files): one `.gz` per upload works (Decision 1); a directory walk, or more than 8 files per post, is not built.
- **Keeping the raw XML or the file name**, a page of covered days, and a delete or undo: the store is append-only by design (`option-ledger.md` Decision 5); the record has no column for a name.

## Adversarial

The reads are kept in the designer's scratchpad, outside the repository: `designs/flex-xml-import-adversarial.md` (the fresh sub-agent's six answers, revision 1) and the advisor's review `flex-xml-import-review-r1.md` (round 1); their answers are written here, which is what the file holds. The six answers of the fresh read:
- *Gap against the goal (history coverage, the daughter's account).* Not adopted: a page of covered days is a history page, left out by name; the daughter's mapping depends on the registered alias-pattern issue and is stated as a dependency in Decision 5.
- *Biggest unnamed risk (the import makes `rowsForAll` true).* Adopted as Doubt 7: checked in `window.go:24` and `job.go:253-262`; with no stored fetch the job still starts at `history_from`, so only an alias with no rows after the first run is affected. Named, not changed.
- *A simpler way (`ImportFlexTradeRows`, or the CLI first).* Not adopted: `ImportFlexTradeRows` makes a changed row a Conflict and tags rows `ot_import`, writes no `flex_fetch` record for the page to read, and gives no per-alias counts; the design now uses `RecordFlexFetch` on the keys that are absent, which gives the same add-only result with the counts and the record. The CLI is ruled out by the owner's word and is in `## Left out`.
- *Lock-in (page schema and reason enum, `UploadChecker`, `ImportLog`, the `import` login records and their query-id shape, hash over decompressed bytes).* Adopted as stated: all are in Decisions 1, 2, 7, 8 and 10; the schema is versioned v1.
- *Claim most likely false (the route is gated by both lines).* Adopted: true, `newOwnerEdit` knows only the key and the settings DSN. Decision 6 and entry 7 say the post route follows `rd.edit` and the ledger line is checked by the route's own code; `env.go` and `settings.go` stay unchanged (blob ids in entry 7).
- *Likeliest slip (entry 7 size, files outside its list, `wire.go`, fragile grep order).* Adopted for the files: the gate needs no `env.go` or `settings.go`, `wire.go` and `store/api.go` stay untouched with their blob ids. The order-of-lines items are kept, because a review without test runs has no other check of "guard before read".
Round 1 of the review:
- *B1, the fetch key (one record per alias with one query id and one start instant is refused by `fetch_id`).* Adopted: true on main (`store.go:263-265`, `:310-312`, `0022:11-30`). The query id carries the hash prefix and the alias, `StartedAt` carries the file ordinal and alias index (Decision 2), and entry 3 item 3 tests the real key under the tag `pg`.
- *B2, failure and the "nothing found" case visible.* Adopted: five statuses, a failed record, a `nothing_found` record, `ImportLog` and the `recent` list (Decisions 3, 8, 10; entries 3, 4, 6, 7).
- *B3, the 2025-12-01 boundary and a different field set (every overlap would be Revised).* Adopted as the owner-style rule the coordinator gave: no date rule, add-only, a stored key is already present, Revised is 0 by construction; tested in entry 4 item 4 and entry 7 item 6; the consequence is in `## Left out`.
- *N1, sections and the consequence for pages; N5, other levels.* Adopted in `## Left out` and in `scope_note`.
- *N2, N3, the token and the write path.* Adopted as this point: the page token is also a valid `/owner-settings` and `/owner-underlyings` token for 15 minutes, and the reverse, because `Check` binds nothing to a route. What a holder gains is the ability to add ledger rows (before, settings only). Reach is local: the remote address must be loopback (`guard.go`), the Host must be a loopback name with the listen port (DNS rebinding is refused), `serve` refuses `--owner-edit` on another address, a cross-origin form post sends the hostile page's Origin and is refused, and the route adds rows and never changes or deletes one (the role cannot). A token scoped to the route would change the signing in `settingsform`; not adopted now, it is Doubt 3, and the cost of being wrong is the add-only insert of rows the owner's own exports could hold anyway.
- *N4, limits.* Adopted: Decision 9 states each limit, its reason, where it is enforced and the refusal text; nesting depth is not capped and memory is bounded by the file limit (50 MiB and a copy).
- *N6, the name of the implementation and how `cmd/ot` reaches it.* Adopted: `flex.Parse`, a direct import in the composition root (Decision 1).
- *N7, cites.* Adopted: `client.go:162` and `:166`, `underlyings_post.go:116-120`, corrected.
- *N8, acceptance forms.* Adopted: counts use `-h -c`, an empty result is "prints no line", the pull request text and diff-shape items moved to results or to `## Checked by CI`.
- *N9, the cut.* Adopted: entry 4 is ordered first (it needs nothing of entry 1); the store read is its own entry; no new library is stated in Decision 6.
- *N10, the adversarial file is not checkable.* Adopted: its answers are written in this section, which is what a reviewer can read.
- *N11, atomicity (a re-import repairs a half write).* Adopted and made true: a file is not one transaction (one per alias). If the second alias fails, the first stays stored and a failed record for the file is written; importing the file again adds the aliases that failed (their keys are absent) and counts the stored ones as already present, which works because the key differs per record and the import never revises.

## Doubts (least sure)

1. The date format of `fromDate` and `toDate` in the owner's own export [unverified]: the fixtures use `yyyy-MM-dd`; IBKR can also send `yyyyMMdd`. Both are read; any other leaves the record's days empty, which loses only the period shown.
2. If the write of a failed record itself fails (the store is down), nothing remains to reload for that file; the POST response shows it and the page says so. Per-alias writes are not atomic across aliases (Adversarial, N11).
3. The upload token is also a valid settings token (Adversarial, N2, N3).
4. Two owner-only edits are named: the store row of `docs/design/modules.md` gains `internal/store/optionledger/api.go` as an interface file (Decision 8), and entry 7 is the first route of `ot serve` that holds `ot_w_optionledger` and the first multipart body on the site (a second-opinion case). Neither is done in this pull request.
5. A re-import appends a `flex_fetch` record per alias (Decision 4); "changes nothing" holds for trade rows only.
6. The limits (64 MiB per post, 8 files, 50 MiB each decompressed) are chosen, not measured on a real 365-day export.
7. An upload before the daily job's first run, or of part of the history, stores rows for an alias, which makes `rowsForAll` true in `planWindows` (`window.go:24`; `job.go:253-262`) and so can end the daily job's re-fetch of an alias that had no rows. With no stored `LatestOkFetch` the job still starts at `history_from`; the effect is only for an alias that had no rows after the first run. Accepted and named, not changed.
8. A trade the daily job stores between the import's read of `FlexTradeRows` and its `RecordFlexFetch` (seconds) could be Revised by the write; the two cover different days for a pre-2025-12-01 export, so it is unlikely, and the record would count it as Revised.
