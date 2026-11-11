# Flex XML import: the owner uploads downloaded IBKR Flex XML files on a web page and they land in the option ledger like the daily job's rows

Author: the optdesk designer (draft)
Owner approved: (pending)
Date: 2026-11-12
Revision: 1

Source: owner ask optdesk-2875 (2026-11-11): the pre-2025-12-01 trade history will be exported from IBKR as Flex Query XML (the query of the option_ledger job, a custom date range, at most 365 days per file); same parsing, same account mapping, same append-only storage as the daily job; a re-import changes nothing; an unmapped account is counted, not stored; the daughter's account arrives as a second statement of the main login and maps once `flex.toml` lists its alias (after the registered alias-pattern issue). Owner, 2026-11-11 ~00:05Z: he will not use a terminal ("we are a product; I click on the web page"), so the import is a page of `ot serve`. Coordinator, same time: the run-one-job-for-one-day button is not in this design (`## Left out`).
Status: draft. Read on origin/main at 5a0e4235beda. "[unverified]" marks a claim not checked in code, docs or a probe.

Goal. On `/flex-import` of `ot serve --owner-edit` the owner chooses one or more `.xml` or `.gz` files, presses one button, and the page shows, per file, either the reason it was refused or, per alias, the statements, rows parsed, new, revised and unchanged, and the count of unmapped statements. The rows are those the daily job would have stored for the same report. No terminal step.

## What main holds today

- The daily job reaches a report through `FlexSource.Fetch` (`internal/jobs/optionledger/job.go:266`). `parseReport` (`internal/sources/flex/parse.go:119`, called from `client.go:162`) is unexported; `maxBody` is 50 MiB (`client.go:24`); `prelude` (`parse.go:17`) refuses a DTD or an entity. `RawSHA256` is the SHA-256 of the raw body (`client.go:163`). The raw XML is not kept (`option-ledger.md` Decision 5).
- `mapStatement` (`internal/jobs/optionledger/map.go:19`) is unexported; it maps an account number to its alias through `OptionLedgerConfig.Accounts`, drops `accountId` and `acctAlias`, hashes the rest, and reports an unmapped account as `ok` false.
- Rows are stored by `RecordFlexFetch(ctx, rec, rows)` (`internal/store/api.go:673`): one transaction, revision rule per row (New, Revised, Unchanged by `AttributesSHA256`), insert-only. The role is `ot_w_optionledger`, the line `OT_PG_DSN_OPTION_LEDGER`. The job's window reads `LatestOkFetch(login, queryID)` (`job.go:240`); a record under another login leaves the window untouched.
- `cmd/ot/optionledger.go:162` `flexConfig` builds `Accounts` (account number to alias) from `flex.toml` and the env file; the number never leaves it. This design does not read `flex.toml` itself.
- `ImportOptionTrader` (`importfile.go:47`) and `cmd/ot/ledgerimport.go` are the older file import, for option-trader's JSON Lines; they stay.
- `ot serve` writes through one pattern today: `--owner-edit`, a loopback `--addr`, `OT_FORM_KEY` (at least 32 bytes) and a writer DSN, a hidden `expires` and `mac` token on the page, `guardOwnerPost` (`cmd/ot/settings.go:245`: Host, 64 KiB cap, `settingsform.Check`: remote address, Host, Origin, content type `application/x-www-form-urlencoded`, 64 KiB, MAC, expiry), and a store opened per post with the DSN of the line (`underlyings_post.go:115`). The guard refuses a multipart body, so it cannot carry an upload as it is.
- A page is: a schema `contracts/schemas/page.<name>.vN.json`, a generated type `web/src/generated/<name>.vN.ts`, a builder in `internal/pages/<name>/`, a route in `cmd/ot`, a view in `web/src/pages/<name>/` (`docs/design/underlyings.md`, entries 3, 4, 8 and 11, are the model). `rules/frontend-stack.md` does not exist on main; the rule is `docs/adr/0034-frontend-stack.md`, which the UI lines below cite.

## Decisions

1. **One parse entry, shared.** `sources.ParseFlexFunc` (`func(r io.Reader) (FlexReport, error)`) is declared in `internal/sources/api.go` and implemented in `internal/sources/flex` by a new file that calls the same `parseReport` and `prelude`. It reads at most `maxBody` bytes after decompression; a body that starts with the gzip magic `1f 8b` is decompressed first (this is what makes option-trader's raw archive, one gzip per report, importable by upload: a `.gz` costs a few lines here, so it is in). `RawSHA256` is the hash of the decompressed bytes, so a file and its gzip have one hash, and it equals the daily job's hash of the same body. `FetchedAt` is left zero; the caller sets it. A body over the limit is `too_large`, a non-report is `not_report`: the existing kinds, no text carries a value. `client.go` is not changed.
2. **One mapping, one store path.** `optionledger.ImportFlexReport` (a new file in `internal/jobs/optionledger`) takes the `FlexReport`, the `Accounts` map, the writer and `now`. It maps every statement with `mapStatement`, groups the rows by alias and writes one `RecordFlexFetch` per alias with that alias's rows: Login `import`, QueryID `file`, `FirstDay` and `LastDay` the earliest `FromDate` and latest `ToDate` of the file's statements (`yyyy-MM-dd` as in the fixtures of `parse_test`; `yyyyMMdd` is also read; anything else refuses the file), `ParserVersion` the job's `parserVersion`, `Statements` the statements of that alias. The unmapped statements are counted into the record of the first alias in sorted order, or into one record without rows when no alias matched. A refused file writes nothing. The owner's per-alias table needs one call per alias, because `FlexInsert` is per call; the cost is that a store failure between two aliases leaves the first written, and the same file imported again finishes it (Doubt 2). Sections (`flexsection`) are not imported (`## Left out`).
3. **Idempotence stated exactly.** A re-import of the same file writes no trade row: every row is Unchanged by the revision rule of Decision 5. It does append one `flex_fetch` record per alias, as every daily run does (the fetch table is a log). The page shows every row as unchanged.
4. **Account mapping is the job's.** The `Accounts` map comes from one function extracted out of `flexConfig` (`flexAccounts`) and used by both; so the alias-pattern issue, when it lands, changes the map for both. The daughter's alias maps when `flex.toml` lists it; before that her statement is counted as unmapped. An account number never reaches the page, a log line or an error text.
5. **Who writes (second opinion).** The upload route is the first path by which `ot serve` holds the option-ledger writer: the role `ot_w_optionledger` (INSERT and SELECT only, no UPDATE or DELETE), opened per post from the env file and closed after it, never kept open. The post route exists where the two settings posts exist: `rd.edit` is set, which `newOwnerEdit` does only with `--owner-edit`, a loopback address, `OT_FORM_KEY` and `OT_PG_DSN_SETTINGS` (`env.go:70`; `ownerEditSecrets` knows nothing of the ledger line, and `env.go` and `settings.go` are not changed). The ledger line is checked by the new route's own code: the GET builder reads the env file and shows `form` null with one gap line when `OT_PG_DSN_OPTION_LEDGER` is unset or `flex.toml` does not load, and the post answers the same refusal without opening a store. No new `.env` line is needed on the owner's machine, which has all three lines.
6. **Guard for an upload.** `settingsapi` gains a second interface `UploadChecker` with `CheckUpload(r FormRequest, maxBody int64, now time.Time) error`: the checks of `Check` (remote, Host, Origin, token, expiry) with the content type `multipart/form-data` and the cap given instead of 64 KiB. `settingsform` implements it on the same value; `cmd/ot` reaches it by a type assertion on `rd.edit.form`, so no existing fake of `SettingsForm` changes. The token is the one of the page (`Token(now)`), which is also a valid settings token (Doubt 3).
7. **Limits.** The post is read under `http.MaxBytesReader` of 64 MiB, at most 8 files, each at most `maxBody` after decompression; the multipart reader streams, files are parsed one at a time and not kept. A file over a limit is refused with its reason and the others go on. A request over the cap is 413 with a fixed text and no page.
8. **The page.** `page.flex_import.v1`: `title`, `sentence`, `form` (null when the route is off; otherwise `post_path` `/owner-flex-import`, `expires`, `mac`, `max_files`, `max_file_text`), `gaps`, and `files`: per file `name` (the base name, at most 80 characters), `status` (`imported` or `refused`), `reason` (a fixed code from a closed enum: `not_report`, `too_large`, `bad_gzip`, `bad_period`, `bad_row`, `store_error`; empty when imported), `period_first`, `period_last`, `aliases` (alias, statements, rows_parsed, rows_new, rows_revised, rows_unchanged, all integers) and `unmapped_statements`. Every sentence is built in Go; the view computes nothing. A GET shows the form and an empty `files`; the POST answers with the page carrying the results (200, no redirect: the result is not stored and the import is idempotent).
9. **No CLI.** `ot flex import FILE...` would be a second front for the same two calls; it costs a command file, a test and a second opener path. It is under `## Left out`.

## Modules

No new row. First matching glob of `docs/design/modules.md`.

| module | paths touched | change |
|---|---|---|
| sources (interface entry 1) | `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/flex/parse_entry.go`, `internal/sources/flex/parse_entry_test.go` | `ParseFlexFunc` and its implementation |
| compute (interface entry 2) | `internal/compute/settingsapi/api.go`, `internal/compute/settingsapi/api_test.go`, `internal/compute/settingsform/upload.go`, `internal/compute/settingsform/upload_test.go`, `docs/contracts/compute.md` | `UploadChecker` and its implementation |
| jobs | `internal/jobs/optionledger/importflex.go`, `internal/jobs/optionledger/importflex_test.go` | `ImportFlexReport` |
| contracts (interface entry 4) | `contracts/schemas/page.flex_import.v1.json`, `web/src/generated/flex_import.v1.ts`, `contracts/fixtures/**`, `internal/contracts/flex_import_v1_test.go`, `docs/contracts/page-schemas.md` | the page schema and its type |
| pages | `internal/pages/flex_import/**`, `docs/contracts/pages.md` | the builder |
| ot | `cmd/ot/flex_import_site.go`, `cmd/ot/flex_import_post.go`, their tests, `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/flex_import_page.md` | extraction, route, post |
| web | `web/src/pages/flex_import/view.tsx`, `web/src/types/flex_import.ts`, `web/test/pages/flex_import/view.test.tsx` | the view |

## Interface

- Entry 1: `sources.ParseFlexFunc` in `internal/sources/api.go`, declarations only; `api_test.go` tests it from both sides (a fake used by a caller; a caller of the real function over a body, a gzip of it, and a body over the limit).
- Entry 2: `settingsapi.UploadChecker`, declarations only; `api_test.go` has a fake and a caller, and checks that the real form value satisfies it.
- Entry 4: `page.flex_import.v1` and the generated type `FlexImport`.
- Jobs: `optionledger.ImportFlexReport(ctx context.Context, w store.OptionLedgerWriter, accounts map[string]string, rep sources.FlexReport, now time.Time) (FlexImportSummary, error)`; `FlexImportSummary` (per alias counts, `Unmapped`, `First`, `Last`) is a type of the package, not of `internal/jobs/api.go` (only `cmd/ot` uses it).

"Landed" means: the entry's pull request is merged to main and `git grep` of its declaration on origin/main finds it; an entry that uses it is cut only after.

## Issues

Seven entries. 1, 2 and 4 are interface entries and land before the entries that use them. Each lists every file it touches and builds alone on main plus the entries it is ordered after; no two entries share a file. Acceptance states results checkable with git or by reading code; test runs are under `## Checked by CI`.

### 1 flex-parse-entry (interface, sources)

Module: sources. Size: M.
Files: `internal/sources/api.go`, `internal/sources/api_test.go`, `internal/sources/flex/parse_entry.go`, `internal/sources/flex/parse_entry_test.go`
Design: docs/design/flex-xml-import.md#Decisions (1)
Contracts: unchanged
Order: first.
Goal: a caller outside the package parses a Flex body, plain or gzip, with the parser of the client.
Acceptance:
1. `git grep -h -c "type ParseFlexFunc func" internal/sources/api.go` prints 1; `git rev-parse --short=12 HEAD:internal/sources/flex/client.go` prints de78d626db08 (unchanged) and `git grep -c "parseReport(" internal/sources/flex/parse_entry.go` prints 1 (the entry calls the one parser, it does not copy it).
2. `parse_entry.go` reads through `io.LimitReader` of `maxBody+1` after the gzip reader, so the limit is on decompressed bytes, and returns the `too_large` kind above it (`git grep -n "LimitReader" internal/sources/flex/parse_entry.go` prints a line after the line that makes the gzip reader); a body starting with bytes 1f 8b is decompressed, any other is passed on.
3. `RawSHA256` is computed over the decompressed bytes with `sha256.Sum256` as `client.go:163` does; `FetchedAt` is not set in the file (`git grep -c "FetchedAt" internal/sources/flex/parse_entry.go` prints 0).
4. `parse_entry_test.go` has a case for each of: plain body, the same body gzipped (equal `RawSHA256`), a gzip that is not valid, a body over the limit, a body with a DTD (refused by `prelude`), and a body that is not a report.
5. `wc -l` of each new file prints under 600; each new test calls `t.Parallel()` and never sleeps.

### 2 upload-guard (interface, compute)

Module: compute. Size: S.
Files: `internal/compute/settingsapi/api.go`, `internal/compute/settingsapi/api_test.go`, `internal/compute/settingsform/upload.go`, `internal/compute/settingsform/upload_test.go`, `docs/contracts/compute.md`
Design: docs/design/flex-xml-import.md#Decisions (6)
Contracts: unchanged
Order: first.
Goal: the guard of the settings form can check a multipart upload under a cap the caller names.
Acceptance:
1. `git grep -h -c "type UploadChecker interface" internal/compute/settingsapi/api.go` prints 1; the `SettingsForm` interface is unchanged (`git grep -c "CheckUpload" internal/compute/settingsapi/api.go` prints 1, in `UploadChecker` only).
2. `git rev-parse --short=12 HEAD:internal/compute/settingsform/guard.go` prints 07ac65d12acf (the existing `Check` is unchanged); `upload.go` calls the host, token and signing helpers of `guard.go` and `token.go` and does not copy them (`git grep -c "loopbackName\|parseToken\|f.sign" internal/compute/settingsform/upload.go` prints at least 3).
3. `upload.go` refuses a content type other than `multipart/form-data` and a size below 0 or above the argument; `upload_test.go` has a case for each of: remote address, Host, Origin, wrong content type, size over the cap, bad token, expired token, token ahead, and a valid upload.
4. `git grep -c "UploadChecker" docs/contracts/compute.md` prints at least 1.

### 3 import-flex-report

Module: jobs. Size: M.
Files: `internal/jobs/optionledger/importflex.go`, `internal/jobs/optionledger/importflex_test.go`
Design: docs/design/flex-xml-import.md#Decisions (2, 3)
Contracts: unchanged
Order: after 1.
Goal: one report is mapped and stored per alias with the job's mapping and revision rule, and counted.
Acceptance:
1. `git rev-parse --short=12 HEAD:internal/jobs/optionledger/map.go` prints b80eeed2759a (`mapStatement` is unchanged) and `git grep -c "mapStatement(" internal/jobs/optionledger/importflex.go` prints 1; `git grep -c "RecordFlexFetch(" internal/jobs/optionledger/importflex.go` prints 1 (one call site, inside the loop over aliases).
2. `git rev-parse --short=12 HEAD:internal/store/api.go` prints 542bda2b86f9 and `git rev-parse --short=12 HEAD:internal/jobs/api.go` prints f34381afe1d5 (no store or jobs interface change).
3. The record carries `Login: "import"`, `QueryID: "file"` and the job's `parserVersion` (`git grep -c 'Login: *"import"' internal/jobs/optionledger/importflex.go` prints 1); `git grep -c "LatestOkFetch" internal/jobs/optionledger/importflex.go` prints 0.
4. `importflex_test.go` has a case for each of: two aliases in one file give two records and per-alias counts that add up; the same report twice gives New 0 and Revised 0 the second time and an Unchanged equal to the first time's New; an unmapped statement is counted and no row of it is written (a fake writer records every row it is given; no row holds an `accountId`, an `acctAlias` or an account number); no alias mapped gives one record without rows and the count; a period in neither accepted form refuses the file and the fake writer is not called; a writer error is returned as a refusal whose text holds no row content or account number.
5. `wc -l` of each new file prints under 600; each test calls `t.Parallel()`, uses a fake writer and a fake clock, and never sleeps.

### 4 flex-import-page-schema (interface, contracts)

Module: contracts (the generated type under `web/src/generated/**` is a path of contracts). Size: M.
Files: `contracts/schemas/page.flex_import.v1.json`, `web/src/generated/flex_import.v1.ts`, `contracts/fixtures/page.flex_import.v1.golden.json`, `contracts/fixtures/page.flex_import.v1.result.json`, `contracts/fixtures/page.flex_import.v1.disabled.json`, `contracts/fixtures/page.flex_import.v1.bad-version.json`, `contracts/fixtures/MANIFEST.synthetic-v1.json`, `internal/contracts/flex_import_v1_test.go`, `docs/contracts/page-schemas.md`
Design: docs/design/flex-xml-import.md#Decisions (8)
Contracts: page.flex_import v1 (new)
Order: first.
Goal: the schema, the type `FlexImport` and four fixtures exist and agree.
Acceptance:
1. The schema has `"$id": "https://optdesk.invalid/schemas/page.flex_import.v1.json"`, `"const": "flex_import"` for `schema`, `"const": 1` for `schema_version`, `additionalProperties: false` on every object, the fields of Decision 8, `status` an enum of exactly `imported` and `refused`, and `reason` an enum of exactly `not_report`, `too_large`, `bad_gzip`, `bad_period`, `bad_row`, `store_error` and the empty text; the counts are `integer` with `minimum` 0.
2. The title is `FlexImport`; the generated file is the output of `node scripts/generate-types.mjs page.flex_import.v1` run from `web/`: line 1 is the banner and `git grep -c "export interface FlexImport " web/src/generated/flex_import.v1.ts` prints 1; no `index.ts` is made and `web/scripts/generate-types.mjs` is not in the diff.
3. The fixtures: `golden` has the form and no file; `result` has one imported file with two aliases and an unmapped count, one refused file with `not_report`, and one imported file whose rows are all unchanged; `disabled` has `form` null and one gap; `bad-version` is the golden with `schema_version` 2. Invented aliases only; `grep -c "@"` of each prints 0. `MANIFEST.synthetic-v1.json` gains the four paths and loses none.
4. `TestFlexImportV1Fixtures` validates golden, result and disabled, shows bad-version fails at `schema_version`, and inline mutations (unknown reason, negative count, extra top-level field) each fail.
5. `docs/contracts/page-schemas.md` has a row `| page.flex_import.v1 | 1 |`; `contracts/schemas/carryover.sha256` is unchanged. The body carries `Contracts: page.flex_import v1 (new)`.

### 5 flex-import-builder

Module: pages. Size: S.
Files: `internal/pages/flex_import/builder.go`, `internal/pages/flex_import/page.go`, `internal/pages/flex_import/builder_test.go`, `internal/pages/flex_import/page_test.go`, `docs/contracts/pages.md`
Design: docs/design/flex-xml-import.md#Decisions (8)
Contracts: unchanged
Order: after 4.
Goal: `Build` turns an input of results into a valid `page.flex_import.v1`, every sentence in Go.
Acceptance:
1. `git grep -c 'Schema: *"page.flex_import.v1"' internal/pages/flex_import/page.go` prints 1; `Input` holds the form (token fields, nil for off), the gaps and the file results as plain values; the builder imports no store row type and no `internal/jobs` (`git grep -c "internal/jobs\|internal/store/" internal/pages/flex_import/builder.go` prints 0).
2. The file name is cut to its base name and at most 80 runes in the builder (`git grep -c "filepath.Base" internal/pages/flex_import/builder.go` prints 1); the sentence of a refused file is chosen from a closed table keyed by reason, none built from input text.
3. `builder_test.go` builds the golden, result and disabled fixtures of entry 4 from inputs and compares the JSON byte for byte; `page_test.go` has a case that `Build` of the registered `Page()` is the read-only page; an unknown reason is an error.
4. `wc -l` of each new file prints under 600; each test calls `t.Parallel()`.

### 6 ot-flex-import-route

Module: ot. Size: L.
Files: `cmd/ot/flex_import_site.go`, `cmd/ot/flex_import_post.go`, `cmd/ot/flex_import_site_test.go`, `cmd/ot/flex_import_post_test.go`, `cmd/ot/optionledger.go`, `cmd/ot/optionledger_test.go`, `cmd/ot/site.go`, `cmd/ot/check_serve.go`, `cmd/ot/check_blocks.go`, `cmd/ot/check_blocks.json`, `docs/contracts/ot/pages/flex_import_page.md`
Design: docs/design/flex-xml-import.md#Decisions (2, 4, 5, 6, 7)
Contracts: unchanged
Order: after 1, 2, 3, 5.
Second opinion: a new write path of `ot serve` under the option-ledger role, with a multipart body. Questions: (a) the post route exists only where `rd.edit` is set (`--owner-edit`, loopback, key and settings DSN) and writes only when the ledger line is set, and a failed `CheckUpload` writes nothing; (b) the writer is opened per post and closed after it; (c) no account number, token, DSN or file content reaches a log line, an error text or the page; (d) the body, file count and decompressed size are capped as Decision 7 says.
Goal: `GET /flex-import` shows the form and `POST /owner-flex-import` imports the files and answers with the page of results.
Acceptance:
1. `git rev-parse --short=12 HEAD:cmd/ot/wire.go` prints f399107f1f80 (the deps struct is not touched; the file has 797 lines on main) and `git rev-parse --short=12 HEAD:cmd/ot/ledgerimport.go` prints ba5f49154fec.
2. `git grep -c "flexAccounts(" cmd/ot/optionledger.go` prints 2 (the definition and the call in `flexConfig`) and `git grep -c "flexAccounts(" cmd/ot/flex_import_post.go` prints 1; `git grep -c "flex.toml" cmd/ot/flex_import_post.go` prints 0 (the file is read through `loadFlexFile` of deps and `flexFileName`).
3. `site.go` routes the path `owner-flex-import` only when `rd.edit != nil`, as the two routes beside it; any method but POST on it is 405 with `Allow: POST`; `git rev-parse --short=12 HEAD:cmd/ot/env.go` prints 8f16c2a1cfe8 and `git rev-parse --short=12 HEAD:cmd/ot/settings.go` prints 9fd77749add2 (unchanged); `flex_import_site.go` reads `store.DSNEnvOptionLedger` itself (`git grep -c DSNEnvOptionLedger cmd/ot/flex_import_site.go` prints at least 1).
4. In `flex_import_post.go` the call of `CheckUpload` is on an earlier line than the first read of a part (`MultipartReader` or `NextPart`); the body is read under `http.MaxBytesReader` of 64 MiB; more than 8 files is refused; the writer is opened with `openOptionLedgerStore` and the line `store.DSNEnvOptionLedger` per post and closed with `defer`; a failed guard answers 403 with a fixed text and a body over the cap 413.
5. A refused file never stops the others, and the writer is not opened when every file was refused before the store; `git grep -n "a.log\." cmd/ot/flex_import_post.go` shows only calls holding component, status and a file index (no name, alias or number).
6. The tests of `flex_import_site_test.go` and `flex_import_post_test.go` have a case for each of: route off without `--owner-edit` (404), read-only page when a line is unset, a forged token (403, fake writer not called), a plain XML and the same gzipped (one hash), two files with one refused, an unmapped account counted and not stored, the same file twice (second post: New 0, Revised 0), an over-cap body (413), a file over the decompressed limit refused, and the page of the POST validated against the schema.
7. `check_serve.go`, `check_blocks.go` and `check_blocks.json` list the page as `underlyings` is listed (`git grep -c flex_import cmd/ot/check_serve.go cmd/ot/check_blocks.json` prints 1 on each); `wc -l` of each new file prints under 600 and of `site.go` under 800.

### 7 web-flex-import-view

Module: web. Size: M.
Files: `web/src/pages/flex_import/view.tsx`, `web/src/types/flex_import.ts`, `web/test/pages/flex_import/view.test.tsx`
Design: docs/design/flex-xml-import.md#Decisions (8)
Contracts: unchanged
Order: after 4.
Goal: a payload of `page.flex_import.v1` shows the upload form and the result tables, every string read from the payload.
UI parts (frontend-stack, ADR 0034):
- 按 frontend-stack：用 shadcn `Card`, `CardHeader`, `CardTitle` 做 the page frame.
- 按 frontend-stack：用 shadcn `Input` (type file, `multiple`, `accept=".xml,.gz"`), `Label` and `Button` (already copied, no new package) 做 the upload as a plain `<form method="post" enctype="multipart/form-data">` with hidden `expires` and `mac`, no `fetch`.
- 按 frontend-stack：用 tanstack table (`web/src/components/data-table/data-table.tsx`) 做 the per-alias table of each imported file with the columns alias, statements, rows parsed, new, revised and unchanged.
- 按 frontend-stack：用 shadcn `Badge` 做 the file status and `Alert` 做 the gaps, the refusal reason and the unmapped count.
- 按 frontend-stack：用 vitest and testing-library 做 the tests; zustand is not used.
Acceptance:
1. `web/src/types/flex_import.ts` has `SUPPORTED_VERSIONS = [1]`, `export type Payload = FlexImport` from `@/generated/flex_import.v1` and `validate = placeholderValidate<Payload>(SUPPORTED_VERSIONS)`.
2. `view.tsx` exports `registration = { schema: 'flex_import', page: page<Payload>(validate, FlexImportView) }`; no file of `web/src/shell/` is in the diff.
3. The view shows `title`, `sentence`, each file's name, status text and reason sentence, the alias table, the unmapped count and the gaps, the form only when `form` is not null, and formats nothing (`git grep -n "toFixed\|Intl\.\|toLocale" web/src/pages/flex_import/view.tsx` prints nothing); the file input has `multiple` (`git grep -c "multiple" web/src/pages/flex_import/view.tsx` prints 1).
4. `view.test.tsx` reads the fixtures of entry 4 with `readFileSync`, checks each shown string is read from the fixture, that the `disabled` payload renders no `<form>`, and that `validate` refuses `bad-version` with `unsupported_version`; it opens no port and never sleeps.

## Rollout

Entries 1 to 5 need no owner step. After entry 6 merges `ot serve` deploys itself with the other code merges; it runs with `--owner-edit` as for `/settings` (both `.env` lines exist). The owner then opens `/flex-import`. No migration, no new role, no new `.env` line. The owner's own work: export the XML files from IBKR (365 days at most each) and choose them on the page.

## Checked by CI

`go test` of `internal/sources/...`, `internal/compute/settingsapi`, `internal/compute/settingsform`, `internal/jobs/optionledger`, `internal/contracts`, `internal/pages/flex_import`, `cmd/ot`; `tsc`, `eslint`, `vitest`, the build and the `type drift` step of the web workflow; `go run ./scripts/arch --check`; the ratchet; the `Contracts:` line check.

## Left out

- **`ot flex import FILE...` on the command line.** Decision 9: a second front for the same calls, and not asked for after the owner's word. If wanted, it is one small command file that calls the same two functions and prints the page's numbers.
- **Follow-up design `job-console`** (every job runnable alone from the web, idempotent; owner ask optdesk-2881): a button that runs one job for one day (what `ot backfill --day D --job J`, `cmd/ot/backfill.go`, does today) under the service env, with the result on the page. A separate audit is running for it; this design takes nothing from it and adds no runner call.
- **Flex sections** (cash, positions and the other sections the daily job records through `flexsection`): not imported; the ask is the trade history. A statement's sections are parsed and dropped. If the old exports are wanted for sections, a later entry adds one `RecordSections` call after `ImportFlexReport`.
- **Bulk import of option-trader's raw archive** (`<raw root>/<sha[:2]>/<sha>.gz`, many files): one `.gz` per upload works (Decision 1); a directory walk, or more than 8 files per post, is not built.
- **Keeping the raw XML**, a history page of past imports, and a delete or undo: the store is append-only by design (`option-ledger.md` Decision 5).

## Adversarial

The read is saved at `designs/flex-xml-import-adversarial.md` in the designer's scratchpad (outside the repository). Answers:
- *Gap against the goal (history coverage, the daughter's account).* Not adopted: a page of covered days is a history page, left out by name; the daughter's mapping depends on the registered alias-pattern issue and is stated as a dependency in Decision 4.
- *Biggest unnamed risk (the import makes `rowsForAll` true).* Adopted as Doubt 6: checked in `window.go:34`; with no stored fetch the job still starts at `history_from`, so only an alias with no rows after the first run is affected. Named, not changed.
- *A simpler way (`ImportFlexTradeRows`, or the CLI first).* Not adopted: `ImportFlexTradeRows` makes a changed row a Conflict and tags rows `ot_import`, while the ask is the daily job's revision rule (New, Revised, Unchanged) and per-alias counts; the CLI is ruled out by the owner's word and is in `## Left out`.
- *Lock-in (page schema and reason enum, `UploadChecker`, the `import` login records, hash over decompressed bytes).* Adopted as stated: all four are in Decisions 1, 3, 6 and 8 and Doubt 4; the schema is versioned v1, the hash choice is Decision 1.
- *Claim most likely false (the route is gated by both lines).* Adopted: true, `newOwnerEdit` knows only the key and the settings DSN. Decision 5 and entry 6 now say the post route follows `rd.edit` and the ledger line is checked by the route's own code, `env.go` and `settings.go` stay unchanged (blob ids in entry 6).
- *Likeliest slip (entry 6 size, files outside its list, `wire.go`, fragile grep order).* Adopted for the files: the gate no longer needs `env.go` or `settings.go`, and `wire.go` and `store/api.go` stay untouched with their blob ids; the order-of-lines items are kept, because a review without test runs has no other check of "guard before read".

## Doubts (least sure)

1. The date format of `fromDate` and `toDate` in the owner's own export [unverified]: the fixtures use `yyyy-MM-dd`; IBKR can also send `yyyyMMdd`. Both are read; any other refuses the file with `bad_period`.
2. One `RecordFlexFetch` per alias gives the owner's per-alias table but is not atomic across aliases; a failure in between is finished by importing the file again.
3. The upload token is also a valid settings token for 15 minutes (the same key and the same value signed). Both routes are owner-only on loopback, so no actor is added; a scoped token would change `settingsform`'s signing.
4. A second-opinion case under the rules: the first route of `ot serve` that holds `ot_w_optionledger`, and the first multipart body on the site.
5. A re-import appends a `flex_fetch` record per alias (Decision 3); "changes nothing" holds for trade rows only.
6. An upload before the daily job's first run, or of part of the history, stores rows for an alias, which makes `rowsForAll` true in `planWindows` (`window.go:34`, `job.go` stored check) and so can end the daily job's re-fetch of an alias that had no rows. With no stored `LatestOkFetch` the job still starts at `history_from`; the effect is only for an alias that had no rows after the first run. Accepted and named, not changed.
7. The limits (64 MiB per post, 8 files, 50 MiB each decompressed) are the designer's numbers, not measured on a real 365-day export.
