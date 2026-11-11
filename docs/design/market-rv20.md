# Market RV20: the 20-day realised volatility of the indices

Revision: 2
Date: 2026-11-11
Status: draft (not yet a pull request). Read on origin/main at f0e54021.

Source: owner 2026-11-11, dispatcher window, verbatim: "4. a, 日本后面再说". Read: 4a is rv20 for SPX, NDX and RUT only; N225 is later (decided, not pending). The drop of `iv_rv20` is the dispatcher's proposal, not the owner's word (Left out).

## Goal

The cell `rv20` of `market_page` (`internal/pages/market_page/cells.go`, rows of `indexSpecs`) and of `home_market`
(`internal/pages/home_market/member.go`, `buildRow`) is Gap `not_built` today. This design stores one reading per index,
`market.rv20|underlying|SPX` (and NDX, RUT), from closes main already holds, in the way `market.hv30_own` is stored
(`computefigures` step, `compute.Figures.HistoricalVol`, `readings.reading`), and the two pages show it from the stored
reading and compute nothing. Revision 1 also stored `market.iv_rv20`; that is dropped (Left out): nothing produces
`market.iv_ibkr` for an index, so the difference could not be filled. N225 is later by the owner's word and stays a Gap (Left out).

## What main holds today

- Closes: `bars_refresh` stores daily `trades` bars of SPX, NDX, RUT and N225 (`internal/jobs/barsrefresh/job.toml`
  `[[index]]`) in `obs.price_bar`. The instrument of each index is made with `ResolveInstrument` keyed by
  `ibkr_conid` (`barsrefresh/run.go:86`) with the spec `Type "index"`, `Name` = the symbol (`barsrefresh/job.go:24`,
  `:109`, `:116-118`), and the rows carry that id as `InstrumentID` with basis `trades` (`run.go:181`). So an index close
  is stored under the id of the `master.instrument` row of type `index` and name `SPX`, `NDX`, `RUT`.
- Subjects of `compute_figures` come from `listSubjects` (`internal/jobs/computefigures/subjects.go`): underlyings with
  `option_eod` rows on the day. Option rows exist only for stocks (`internal/jobs/optioneod/watch.go:88-100`), so SPX,
  NDX and RUT are never subjects; `market.hv30_own` for them is not produced either. That is why Revision 1's plan
  yielded no key for the indices.
- There is no reader from a name to an instrument id: `store.InstrumentNameReader.InstrumentNames` is id to name
  (`internal/store/api.go:142-146`). The view `rd.instrument_r1` (`migrations/0010_instrument_names_view.sql`) holds
  `instrument_id, instrument_type, name, currency` and is readable by the page role that `computefigures` already uses
  for names (`cmd/ot/runonce.go:475-481`, `openInstrumentName`).
- `internal/store/api.go` has 800 lines and is not in `docs/design/ratchet.json`; one added line makes it a file over 800
  that needs an owner-only ceiling. The reader therefore is not added there (Decision 4).
- `Figures.HistoricalVol(id, closes, asOf)` takes N and TradingDays from the rule: sample stdev (n - 1) of the last N
  log returns times sqrt(TradingDays), needs N+1 closes above zero, `max_age_days` check. `runHv30`
  (`computefigures/hv.go`) reads the winner `trades` rows of a subject `[day - 70 calendar days, day]` and stores the
  figure with `hvParams` (window = the rule's N); it works for any target that carries a rule and an instrument id.
- `GLOSSARY.md` lines 88 and 441-443 name `market.rv20_n225` and `market.iv_rv20_n225` (Python `readings/statistics.py`).
  No such code exists.

## Decisions

1. Inputs: the winner `trades` closes of `obs.price_bar` of the index instrument (never `adjusted_last`, never IBKR's own
   figure, never FRED `SP500`), read exactly as `runHv30` reads them.
2. Definition (ADR 0031, glossary): sample stdev (ddof 1) of the last 20 log returns of consecutive stored closes times
   sqrt(252). Rule `market.rv20` v1: convention `rv20_v1`, unit `decimal`, scale 6, `n` 20, `trading_days` 252,
   `max_age_days` 4, implemented by the existing `HistoricalVol`: no new compute code and no change of
   `compute.Figures`.
3. A valid reading needs 21 closes above zero and a newest close at most 4 calendar days before the as-of day. Fewer
   closes: `missing` `short_history`; none: `no_input`; stale: `stale_input`; a close not above zero: `bad_input`.
   Nothing is filled or extrapolated, never stored as zero.
4. Subjects and the name-to-id mapping. The subjects of `market.rv20` are the names of a manifest list
   `rv20_indices = ["SPX", "NDX", "RUT"]` (the `[[index]]` rows of `bars_refresh` whose closes are the US indices;
   N225 is left out). They do not come from `listSubjects`. The id of a name is read by a new reader declared by the
   job package, `computefigures.IndexIDReader` (`IndexIDs(ctx, names []string) ([]store.InstrumentName, error)`: the
   instruments of type `index` whose name is in names, ordered by id), carried by a new optional field
   `ComputeStores.IndexIDs`. The smallest correct mapping is this lookup by `(type = index, name)` because the stored
   bars are keyed by exactly that row (see evidence above); it needs no `con_id` in the manifest (which is 0 there, and
   resolved per run by the broker), no broker, no write role and no migration. It is declared in the job package and
   not in `internal/store/api.go` because that file cannot take a line (800 lines, no ceiling). The implementation is
   `internal/store/instrument` (`OpenIndexIDs`, a read of `rd.instrument_r1` with the page role), wired in `cmd/ot`.
   A name with no instrument row makes its key fail `Retryable` ("no index instrument named X"); a name with two
   rows makes it fail `Permanent` ("ambiguous"); neither guesses an id. With `PriceBars` or `IndexIDs` nil the keys are still
   planned (Decision 5) and each fails `Retryable` naming the cause ("no price bar reader", "no index reader"): a run never
   goes green with nothing stored. This covers the case where `cmd/ot/wire.go:555-575` leaves the bar reader off.
5. Plan and Run: `Plan` adds one key `market.rv20|underlying|<name>` per listed name of the manifest, sorted with the
   rest, whether or not the readers are set (so a missing reader is a failed key, not a missing key). `Run`'s
   routing switch (`job.go:245-253`) tests the rv20 rule name before the `kindUnderlying` case (those keys are not in
   `under`) and sends them to a new list `rvs`; without that the name falls to `runIvStats`. `rvs` run after HV30 and
   use `runHv30` unchanged (target = rule + id), so the stored reading, its inputs (`obs.price_bar`, id|trades) and
   parameters (`data_day`, `observations`, `window`) are those of `hv30_own`.
6. Pages: `rv20` is the stored value times 100 in `vol_points`, one decimal, half-even, through the existing
   `shiftRound(v, 2, 1)` (`0.162060` shows `16.2`). A row with no reading is Gap `no_data`; a reading not `ok` is Gap
   `missing_input`; N225 keeps `not_built`. The pages read the newest reading of the last `readingDays` as their `iv`
   cell does (`readNewestKind`, kind `underlying`). A `rv20` Gap does not enter the page's `failed` counter
   (`cells.go:254` counts the percentile only, as it does for `iv`). `iv_minus_rv20` stays Gap `not_built` on every row
   with its existing text ("implied minus realised: not built yet", `ivRvNotBuilt`, `diffNotBuilt`), unchanged: it says
   "not built" and no reason, because a stored `market.iv_ibkr` for an index can appear (`ivhistory` keys by any watch
   symbol). The page header strings that say "the realised columns are not built yet" are changed to name only the
   implied-minus-realised column. Both pages compute nothing, guarded by a purity test (issues 6 and 7). The schemas already type the cells `FigureOrGap`; the web changes nothing.
7. Register: `market.rv20` gets a `[[figure]]` entry with all 15 keys, `pages = []` (as `hv30_own`), `implemented_by`
   `internal/compute/figures.(*calc).HistoricalVol`. The agreement lines (`agreed_on`, `agreed_in`) come from the owner's
   approval of this design, carried in the issue text.
8. Glossary: the entries on lines 88, 441, 442 (`market.iv_n225`) and 443 keep `rv20_n225`, `iv_rv20_n225`, `market.iv_n225` and subject `JP`
   and are marked "not built"; one added sentence says that `market.rv20` is the id of the stored 20-day realised
   volatility of the SPX, NDX and RUT subjects (source: the stored `trades` closes) and that the N225 reading is later.
   Nothing is withdrawn.

## Modules

`docs/design/modules.md` is not changed and no package is added. Paths by the first row that matches: `compute`
(rules), `store` (`internal/store/instrument` is under `internal/store/**`), `jobs`, `ot` (`cmd/**`), `pages`, `docs`.

Judged cuts that span rows (each is one pull request, the precedent being `hv30-rule` in `market-bars.md` and
`compute-rules` in `compute-figures.md`):

- Issue 3 `rv20-rules` spans `compute` and `docs`: `TestRulesMatchRegister` needs the rule row and the register entry
  `docs/conventions/register.toml` (row `docs`) in one pull request.
- Issue 1 `index-ids-seam` and issue 4 `rv20-step` each edit `docs/contracts/jobs/compute_figures.md`, which is a `jobs`
  path, so they stay single-module.
- Issue 2 `index-ids-store` edits `docs/contracts/store.md`, a `store` path.
- Issue 5 `index-ids-wire` is `ot` only; it needs no file of another module.

Build order (each entry compiles against main plus the entries it is ordered after):

- Parallel, no Order line: issue 1 (jobs seam) and issue 3 (rules and register) share no file and no symbol.
- Chain: 2 after 1 (the store method has the signature of the seam); 4 after 1 and 3 (it uses `IndexIDReader` and
  `findRules` fails with "no rule" until issue 3's row exists); 5 after 1 and 2 (it uses `instrument.OpenIndexIDs` of
  issue 2); 6 after 3 and 4 (`ids_test.go` checks `figRv20` against the rules table and the manifest); 7 after 4 and 6;
  8 after 3. Issues 2 and 4 are parallel to each other, as are 5 and 6.
- Issue 1 alone is the interface entry: it is the only new declaration other modules compile against without being able
  to build it themselves (`IndexIDReader`). `instrument.OpenIndexIDs` is a concrete constructor, not a seam: issue 5 uses
  it only after issue 2, and the rule row is data read by id string, so neither is an interface entry.

## Interface

Issue 1 is the only interface entry. What lands in a later entry is used only by entries ordered after it (see Build order in Modules).

- Jobs (issue 1, interface): `computefigures.IndexIDReader` and `ComputeStores.IndexIDs`
  (`internal/jobs/computefigures/types.go`), optional like `PriceBars`; contract text in
  `docs/contracts/jobs/compute_figures.md`; tested from both sides: a fake reader in the job's tests and a
  compile-time assertion in `cmd/ot` that the store implementation satisfies it (issue 5, ordered after 1 and 2).
- Store (issue 2, used by issue 5 only): `instrument.OpenIndexIDs(ctx, dsn) (*instrument.IndexIDs, error)`, methods `IndexIDs` and `Close`;
  not an edit of `internal/store/api.go`.
- Rule id `market.rv20` (issue 3), read by `jobs` and `pages` by id string through `compute.Rules`.
- Manifest keys of `computefigures` (`job.toml`, issue 4; read by issue 6's `ids_test.go`, ordered after 4): `rv20 = "market.rv20"`, `rv20_indices = ["SPX","NDX","RUT"]`.
- Reading key (dataset `readings.reading`): `market.rv20|underlying|SPX` (same for NDX, RUT); parameters `data_day`,
  `observations`, `window`.
- `compute.Figures`, `internal/compute/api.go` and `internal/store/api.go` are not edited.

## Issues

Build order: every entry compiles and its own tests pass against main plus the entries it is ordered after, and
nothing else. Which run in parallel and which chain, and why, is in `## Modules` (Build order).

### 1 index-ids-seam (interface)

Module: jobs. Size: S.
Files: `internal/jobs/computefigures/types.go`, `internal/jobs/computefigures/index_ids_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/market-rv20.md#decisions (4)
Order: first.
Goal: declare `IndexIDReader` and the optional field `ComputeStores.IndexIDs` as in Decision 4, a fake reader in a new
test file, the contract text; behaviour of the job unchanged.
Acceptance:
1. `internal/jobs/computefigures/types.go` declares `type IndexIDReader interface` with the method `IndexIDs(ctx context.Context, names []string) ([]store.InstrumentName, error)` and the field `IndexIDs IndexIDReader` in `ComputeStores` (`git grep -n "IndexIDs(ctx context.Context, names \[\]string) (\[\]store.InstrumentName, error)\|IndexIDs  *IndexIDReader" -- internal/jobs/computefigures/types.go` prints 2 lines); the field `PriceBars` is still declared there (`git grep -c "PriceBars  *bar.PriceBarReader" -- internal/jobs/computefigures/types.go` prints 1).
2. `docs/contracts/jobs/compute_figures.md` names the optional field `ComputeStores.IndexIDs` and the reader `IndexIDReader` as a seam with no user yet (`git grep -n "IndexIDReader" -- docs/contracts/jobs/compute_figures.md` prints a line that contains "optional"); what a nil value does to a key is written by issue 4, which builds the key.
3. `internal/jobs/computefigures/index_ids_test.go` has a fake type with the method `IndexIDs(` that is assigned to `ComputeStores{IndexIDs: ...}` (`git grep -n "IndexIDs:" -- internal/jobs/computefigures/index_ids_test.go` prints a line); `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/jobs/computefigures -race -count=1`, `go run ./scripts/arch --check`.

### 2 index-ids-store

Module: store. Size: S.
Files: `internal/store/instrument/index_ids.go`, `internal/store/instrument/index_ids_test.go`, `internal/store/instrument/index_ids_pg_test.go`, `docs/contracts/store.md`
Design: docs/design/market-rv20.md#decisions (4)
Order: after 1 (the method has the signature of `IndexIDReader`; the store test imports nothing of jobs, so the order is for the reviewer's contract check).
Goal: `OpenIndexIDs` (page role DSN line, no migration) and `IndexIDs`: rows of `rd.instrument_r1` with
`instrument_type = 'index'` and name in the list, ordered by id.
Acceptance:
1. The query reads the view and only the type index: `git grep -c "rd.instrument_r1" -- internal/store/instrument/index_ids.go` is at least 1, `git grep -c "instrument_type = 'index'" -- internal/store/instrument/index_ids.go` prints 1, `git grep -c "master\." -- internal/store/instrument/index_ids.go` prints 0.
2. An empty list makes no database call: `internal/store/instrument/index_ids_test.go` has a test with a database seam that fails the test when called, given `nil` names (`git grep -c "no database call\|empty" -- internal/store/instrument/index_ids_test.go` is at least 1).
3. More than 500 names is a `Permanent` error: the test passes 501 names and asserts the error is `Permanent` (`git grep -n "501" -- internal/store/instrument/index_ids_test.go` and `git grep -c "Permanent\|permanent" -- internal/store/instrument/index_ids_test.go` are each at least 1).
4. A name of empty text is a `Permanent` error: the test passes `[]string{"SPX", ""}` and asserts `Permanent` (`git grep -n '""' -- internal/store/instrument/index_ids_test.go` shows that call).
5. `internal/store/api.go` and `migrations/` are untouched and the pg test is tagged: `git diff --name-only BASE` lists exactly the Files line; `head -1 internal/store/instrument/index_ids_pg_test.go` is `//go:build pg`; `git grep -c "IndexIDs" -- docs/contracts/store.md` is at least 1.
Checked by CI: `go test ./internal/store/instrument -race -count=1` (the `pg` test runs only in GitHub CI).

### 3 rv20-rules

Module: compute, plus docs (judged two-module entry, Modules above). Size: S.
Files: `internal/compute/rules/rules.json`, `internal/compute/rules/rules_test.go`, `docs/conventions/register.toml`
Design: docs/design/market-rv20.md#decisions (2, 3, 7)
Order: first (shares no file with 1 and 2; it lands the rule row that issue 4's `findRules` (`job.go:145`, "no rule") needs and the id that issues 6, 7 and 8 read).
Goal: rule `market.rv20` with the numbers of Decision 2 and the hash the hash test computes for `HistoricalVol`, and its
`[[figure]]` entry (15 keys). The old entries are unchanged.
Acceptance:
1. `git grep -c '"id":"market.rv20".*"n":20.*"trading_days":252.*"max_age_days":4' -- internal/compute/rules/rules.json` prints 1; `git grep -c "iv_rv20" -- internal/compute/rules/rules.json docs/conventions/register.toml` prints 0 for both files.
2. `git grep -c 'id = "market.rv20"' -- docs/conventions/register.toml` prints 1; the new entry is not pending (`git grep -n -A16 'id = "market.rv20"' -- docs/conventions/register.toml | grep -c pending-recompute` prints 0) and the entry of `market.hv30_own` is still there (`git grep -c 'id = "market.hv30_own"' -- docs/conventions/register.toml` prints 1).
3. `internal/compute/rules/rules_test.go` names the rule in a test of its numbers (`git grep -n "market.rv20" -- internal/compute/rules/rules_test.go` prints a line) and the hash test still exists (`git grep -c "func TestHashesMatchSource" -- internal/compute/rules/rules_test.go` prints 1); `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/compute/rules -race -count=1` (`TestHashesMatchSource`, `TestRulesMatchRegister`, `TestNoPendingForGoIds`, `TestRegisterEntriesComplete`).

### 4 rv20-step

Module: jobs. Size: M.
Files: `internal/jobs/computefigures/rv.go`, `internal/jobs/computefigures/rv_test.go`, `internal/jobs/computefigures/job.go`, `internal/jobs/computefigures/job.toml`, `internal/jobs/computefigures/job_test.go`, `internal/jobs/computefigures/hv_test.go`, `internal/jobs/computefigures/ease_test.go`, `internal/jobs/computefigures/valuation_test.go`, `docs/contracts/jobs/compute_figures.md`
Design: docs/design/market-rv20.md#decisions (1 to 5)
Order: after 1 and 3.
Goal: `Plan` adds one key per name of `rv20_indices`; `Run` resolves each name to its instrument id through
`IndexIDs`, routes the key by the rv20 rule name (before the `kindUnderlying` case, never to `runIvStats`) and stores it
through `runHv30` with the rule `market.rv20`. A missing reader, a missing or an ambiguous index instrument fails the
key as Decisions 4 and 5 say. Because `Plan` now always plans three more keys, and a key whose reader is absent fails, every same-package test that runs all planned keys and requires no error is given the readers it needs or the new expectation: `job_test.go`, `ease_test.go` (`runAll` with `res.Err == nil` at lines 120-121 and 203), `valuation_test.go` (`runAll` with `r.Err != nil` at lines 133 and 161) and `hv_test.go` (`TestNilPriceBarsSkipsStep`, line 155, is rewritten to assert the failed rv20 keys). General rule of this design: a same-package test file that uses a changed identifier is in Files.
Acceptance:
1. The stored value is the hand-worked one: `rv_test.go` feeds 21 closes alternating 100 and 101 and requires `0.162060` (`ln(1.01) x sqrt(20/19) x sqrt(252)`, 6 places) with state `ok` (`git grep -c "0.162060" -- internal/jobs/computefigures/rv_test.go` is at least 1).
2. Decision 3, short history: a test with 20 closes requires state `missing` and reason `short_history`; none requires `no_input`; a newest close 5 days old requires `stale_input` (`git grep -n "short_history" -- internal/jobs/computefigures/rv_test.go`, the same for `no_input` and the same for `stale_input` each print a line).
3. Decision 3, a close not above zero: a test with one close `0` or `-1` requires state `missing`, reason `bad_input` and no stored value, never `0` (`git grep -n "bad_input" -- internal/jobs/computefigures/rv_test.go` prints at least 1 line, and the same test asserts the reading's `Value` is nil).
4. Decision 1, basis: `rv_test.go` stores `adjusted_last` rows with other closes beside the `trades` rows and requires the `trades` value (`git grep -c "adjusted_last" -- internal/jobs/computefigures/rv_test.go` is at least 1); `git grep -c "adjusted_last\|historical_vol\|market.iv30_own\|Iv30" -- internal/jobs/computefigures/rv.go` prints 0.
5. Decision 5, planning and routing: `Plan` yields exactly the keys `market.rv20|underlying|SPX`, `NDX`, `RUT` beyond the old ones and no other rv20 key (a test whose expected list names the three); the rv20 name is tested before `case known && kindUnderlying:` in `job.go` (`git grep -n "j.m.Rv20\|case known && kindUnderlying" -- internal/jobs/computefigures/job.go` lists a `j.m.Rv20` line above that case line) and a test shows the key is not stored by `runIvStats` (no `ivp` or `ivr` reading for the name).
6. Decision 4, failure and never green with nothing stored: with `PriceBars` nil, with `IndexIDs` nil, with an unknown name and with a name of two rows, each key fails (`JobResult` error, `Usable` 0). Four tests in `rv_test.go`, one each, assert the error text: `TestRv20NoBarReaderFails` ("no price bar reader"), `TestRv20NoIndexReaderFails` ("no index reader"), `TestRv20UnknownIndexFailsRetryable` ("no index instrument named", Retryable) and `TestRv20AmbiguousIndexFailsPermanent` ("ambiguous", Permanent); `git grep -c "^func TestRv20NoBarReaderFails\|^func TestRv20NoIndexReaderFails\|^func TestRv20UnknownIndexFailsRetryable\|^func TestRv20AmbiguousIndexFailsPermanent" -- internal/jobs/computefigures/rv_test.go` prints 4. The existing `TestNilPriceBarsSkipsStep` (`hv_test.go:155`) is rewritten to assert the failed rv20 keys (`git grep -n "no price bar reader" -- internal/jobs/computefigures/hv_test.go` prints a line) while still asserting no hv30 key is planned or stored.
7. `git grep -c 'rv20 = "market.rv20"\|rv20_indices' -- internal/jobs/computefigures/job.toml` prints 2; `git grep -n "rv20" -- docs/contracts/jobs/compute_figures.md` prints a line that says a missing reader fails the key; `wc -l < internal/jobs/computefigures/rv.go` is under 600; `git diff --name-only BASE` lists only the Files line.
Checked by CI: `go test ./internal/jobs/... -race -count=1`, `go run ./scripts/arch --check`.

### 5 index-ids-wire

Module: ot. Size: S.
Files: `cmd/ot/runonce.go`, `cmd/ot/wire.go`, `cmd/ot/index_ids_wire_test.go`
Design: docs/design/market-rv20.md#decisions (4)
Order: after 1 and 2 (the default `instrument.OpenIndexIDs` and the assertion `var _ computefigures.IndexIDReader = (*instrument.IndexIDs)(nil)` name the symbols of issue 2 and the interface of issue 1).
Goal: a `deps` field `openIndexIDs` (default `instrument.OpenIndexIDs`) and, in `openComputeStores`, `cs.IndexIDs` set
from it with the pages line, closed with the others; a nil `openIndexIDs`, or an opener that returns an error, leaves `IndexIDs` nil with a logged warning and does not fail `openComputeStores` (the keys then fail, Decision 4; this keeps every `cmd/ot` test that opens the compute stores with a fake `openInstrumentName` and the default deps from dialling a database). `cmd/ot/wire_computefigures_test.go` (the `rec.dsns` counts at lines 171 and 193) is not changed: `rec.dsns` records only the bar opener, and `openIndexIDs` is a separate field. the step-off warning names `market.rv20` beside `market.hv30_own`.
Acceptance:
1. `git grep -c "openIndexIDs: *instrument.OpenIndexIDs" -- cmd/ot/wire.go` prints 1 (the default) and `git grep -c "openIndexIDs  *func" -- cmd/ot/wire.go` prints 1 (the field) and `git grep -c "cs.IndexIDs" -- cmd/ot/runonce.go` prints 1; `git grep -n "market.rv20" -- cmd/ot/wire.go` shows the warning line near `HV step left out`.
2. `cmd/ot/index_ids_wire_test.go` contains `var _ computefigures.IndexIDReader = (*instrument.IndexIDs)(nil)` (`git grep -c "computefigures.IndexIDReader = (\*instrument.IndexIDs)(nil)" -- cmd/ot/index_ids_wire_test.go` prints 1) and tests of `openComputeStores`: `TestIndexIDsSetWhenOpenerSet` (non-nil with the fake opener), `TestIndexIDsNilWhenNoOpener` (nil with no opener) and `TestIndexIDsNilWithWarningWhenOpenerErrors` (the opener returns an error: `IndexIDs` is nil, a warning naming `market.rv20` is logged, and `openComputeStores` returns no error); `git grep -c "^func TestIndexIDsSetWhenOpenerSet\|^func TestIndexIDsNilWhenNoOpener\|^func TestIndexIDsNilWithWarningWhenOpenerErrors" -- cmd/ot/index_ids_wire_test.go` prints 3.
3. Size: `wc -l < cmd/ot/wire.go` is at most 800 and `wc -l < cmd/ot/runonce.go` is at most 600; `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./cmd/ot -race -count=1`, `go run ./scripts/arch --check`.

### 6 market-page-rv20

Module: pages. Size: M.
Files: `internal/pages/market_page/cells.go`, `internal/pages/market_page/cells_test.go`, `internal/pages/market_page/builder_test.go`, `internal/pages/market_page/ids.go`, `internal/pages/market_page/ids_test.go`, `internal/pages/market_page/strings.go`, `internal/pages/market_page/purity_test.go`, `docs/contracts/pages.md`
Design: docs/design/market-rv20.md#decisions (6)
Order: after 3 and 4 (`ids_test.go` checks each id against the rules table and the `compute_figures` manifest).
Goal: `buildIndices` fills `rv20` of SPX, NDX and RUT from the stored reading (`readNewestKind`, kind `underlying`), Gaps by
Decision 6; N225 `rv20` and every `iv_minus_rv20` stay `not_built` with their existing text; the header strings of
`strings.go` are corrected; `data_times` is unchanged. `builder_test.go`'s reading fake (which today demands kind `market`
for every name but `iv_ibkr`) and `TestIvRvCellsAreNotBuilt` and `pageGaps` are updated to the new truth; `purity_test.go`
is new (the package has none) and guards that the page computes nothing.
Acceptance:
1. The value: `cells_test.go` has a table that maps stored values to texts and includes `0.162060` to `16.2` (`git grep -c "16.2" -- internal/pages/market_page/cells_test.go` is at least 1).
2. Half-even rounding to one decimal beyond that case: the same table has the exact ties `0.162500` to `16.2`, `0.163500` to `16.4` and `0.000500` to `0.0` (`git grep -c "0.162500\|0.163500\|0.000500" -- internal/pages/market_page/cells_test.go` is at least 3).
3. Gaps: `TestRv20CellNoRowIsNoData`, `TestRv20CellNotOkIsMissingInput` and `TestN225RvStaysNotBuilt` exist in `cells_test.go` and assert the reasons `no_data`, `missing_input` and `not_built` (`git grep -c "^func TestRv20CellNoRowIsNoData\|^func TestRv20CellNotOkIsMissingInput\|^func TestN225RvStaysNotBuilt" -- internal/pages/market_page/cells_test.go` prints 3).
4. The failed counter is unchanged: `TestRv20GapsLeaveFailedCountUnchanged` in `cells_test.go` builds a page where all three `rv20` cells are Gaps and asserts the run's `failed` count and status equal those of the same page with the cells filled (`git grep -c "^func TestRv20GapsLeaveFailedCountUnchanged" -- internal/pages/market_page/cells_test.go` prints 1).
5. `iv_minus_rv20` keeps its text: the string is still in `strings.go` (`git grep -c "implied minus realised: not built yet" -- internal/pages/market_page/strings.go` prints 1) and `cells_test.go` asserts it on rows 0 to 3 (`git grep -c "implied minus realised: not built yet" -- internal/pages/market_page/cells_test.go` prints at least 1).
6. The header: `git grep -c "realised columns are not built yet" -- internal/pages/market_page/strings.go` prints 0 and the new header text appears in a test (`git grep -n "implied-minus-realised\|implied minus realised" -- internal/pages/market_page/cells_test.go`).
7. Purity: `purity_test.go` (modelled on `internal/pages/home_market/purity_test.go`) fails on an import of `internal/store/bar`, `internal/jobs`, `internal/compute/figures`, `database/sql` or `os` and on the text `math.Sqrt` or `math.Log` in a non-test file (`git grep -c "math.Sqrt\|math.Log\|internal/store/bar" -- internal/pages/market_page/purity_test.go` is at least 3).
8. Ids and files: `git grep -c 'figRv20 *= "market.rv20"' -- internal/pages/market_page/ids.go` prints 1; `git grep -c "rv20NotBuilt" -- internal/pages/market_page/cells.go` is at most 1 (N225 only); `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/pages/... -race -count=1`, `go run ./scripts/arch --check`.

### 7 home-market-rv20

Module: pages. Size: S.
Files: `internal/pages/home_market/member.go`, `internal/pages/home_market/member_test.go`, `internal/pages/home_market/strings.go`, `internal/pages/home_market/purity_test.go`, `internal/pages/home_market/testdata/ok.golden.json`
Design: docs/design/market-rv20.md#decisions (6)
Order: after 4 and 6 (same cell rules as issue 6, Decision 6; `docs/contracts/pages.md` is edited by 6 only; the rule row of issue 3 is before 4).
Goal: `buildRow` fills `RV20` from `c.Readings` (kind `underlying`) with the cell rules of issue 6; `IVMinusRV20` stays
`not_built` with its text. `member_test.go`'s `readings` fake errors on any name but `iv_ibkr` ("unexpected reading"),
so it is extended to answer `market.rv20`; the `ok` golden carries the filled cell. No other golden or test builds this
member: `internal/pages/home_page/testdata/*.json` are static fixtures that no builder produces, and the permissive
readers in `cmd/ot/wire_pages_test.go:51` and `cmd/ot/page_test.go:338,358` return no rows (so `no_data`) and assert
nothing on `rv20` (read on origin/main f0e54021).
Acceptance:
1. The value and rounding: `member_test.go` maps `0.162060` to `16.2` and the ties `0.162500` to `16.2`, `0.163500` to `16.4` (`git grep -c "0.162060\|0.162500\|0.163500" -- internal/pages/home_market/member_test.go` is at least 3).
2. Gaps: `TestRowRv20NoRowIsNoData` and `TestRowRv20NotOkIsMissingInput` exist in `member_test.go` and assert `no_data` and `missing_input` for `RV20` (`git grep -c "^func TestRowRv20NoRowIsNoData\|^func TestRowRv20NotOkIsMissingInput" -- internal/pages/home_market/member_test.go` prints 2).
3. `iv_minus_rv20` keeps `not_built` and its text: `git grep -c "diffNotBuilt *=" -- internal/pages/home_market/strings.go` prints 1 and `member.go` still uses it for `IVMinusRV20` (`git grep -n "IVMinusRV20: *mkGap" -- internal/pages/home_market/member.go` prints a line).
4. Purity: `purity_test.go` is extended to fail on the text `math.Sqrt` or `math.Log` in a non-test file (`git grep -c "math.Sqrt\|math.Log" -- internal/pages/home_market/purity_test.go` is at least 2) and still has its import checks.
5. The golden: `git grep -c '"rv20"' -- internal/pages/home_market/testdata/ok.golden.json` is at least 1 and the value in it is a `value` cell, not a `gap` (read the golden near `"rv20"`); `git diff --name-only BASE` lists exactly the Files line.
Checked by CI: `go test ./internal/pages/home_market -race -count=1`.

### 8 glossary-rv20

Module: docs. Size: XS.
Files: `docs/GLOSSARY.md`
Design: docs/design/market-rv20.md#decisions (8)
Order: after 3.
Goal: Decision 8: the four entries (lines 88, 441, 442 and 443) keep their names and are marked not built; one sentence names `market.rv20`.
Acceptance:
1. Nothing is withdrawn: `git grep -n "rv20_n225\|iv_rv20_n225\|market.iv_n225" -- docs/GLOSSARY.md | grep -vc "not built"` prints 0 and `git grep -n "rv20_n225" -- docs/GLOSSARY.md | grep -c "withdrawn"` prints 0; the lines still name subject `JP` (`git grep -n "rv20_n225" -- docs/GLOSSARY.md | grep -c '`JP`'` is at least 1).
2. The id is named: `git grep -c "market.rv20[^_]" -- docs/GLOSSARY.md` is at least 1 and that line says N225 is later (`git grep -n "market.rv20[^_]" -- docs/GLOSSARY.md | grep -c "later"` is at least 1); `git diff --name-only BASE` lists exactly the Files line.

## Left out

- `market.iv_rv20`, `Figures.Difference` and the cell `iv_minus_rv20` (the dispatcher's proposal, for the owner to
  accept or reverse; the owner's "4. a" names rv20 only): the stored
  `market.iv_ibkr` of an index has no producer (`iv_history` keys come from watch entries by contract symbol,
  `internal/jobs/ivhistory/job.go:70-85`; nothing in the repo puts SPX, NDX or RUT there), so the `iv` cell of the
  indices may well be a Gap (the owner's watch list is not in the repository, unverified). The `iv_minus_rv20` cells stay `not_built` with the existing text (Decision 6). It returns with a design for an index IV producer.
- N225 (and TOPX): later, by the owner's word ("日本后面再说"). No option rows, no stored IV, and its bars exchange
  `OSE.JPN` is unverified. The row stays `not_built`; the glossary entries `rv20_n225`, `iv_rv20_n225` and subject `JP`
  stay, marked not built (Decision 8).
- Rejected ways to map an index name to its instrument id (Decision 4): (a) `con_id` in the manifest of
  `compute_figures`: the `[[index]]` rows of `bars_refresh` carry `con_id = 0` and resolve it per run through the broker,
  so a second copy would be unverified owner data that drifts from the one that stores the bars; (b) a reader in
  `internal/store/api.go`: the file has 800 lines and no ceiling, one added line needs an owner-only ratchet entry;
  (c) resolving through the broker (`ResolveContract`) or `ResolveInstrument`: `compute_figures` has no broker and no
  write role, and `ResolveInstrument` creates a row when none exists; (d) taking the subjects from `listSubjects`:
  the indices never have option rows (Revision 1's blocking fault).
- `market.hv30_own` for the indices: the same mapping would give it, not asked for.
- Exchange strings of the `[[index]]` rows (NDX `NASDAQ`, RUT `RUSSELL`) are the owner's proposals, unverified live
  (`barsrefresh/job.toml` line 1); a wrong one gives no bars and, after this design, a `Retryable` failed key
  "no index instrument named X" that names it, not a silent gap.
- The `PriceBars` reader is wired only when broker history is up at start (`cmd/ot/wire.go:555-575`); otherwise the step is
  left out, the existing warning "HV step left out" is logged (issue 5 adds the rv20 name to it) and, by Decision 4, each
  rv20 key fails "no price bar reader" instead of vanishing.
- Missing-day detection in the stored closes (a gap makes one return span two days), as for HV30.
- `market.iv30_own` as an implied side, RV20 of single stocks, a percentile or chart of RV20: no consumer.
- Page-cell paths in the register `pages` list: `[]`, as `market.hv30_own`.
- Schema changes, web changes, `compute.Figures` changes, `internal/store/api.go` changes: none.

## Adversarial

Source: the adversarial review `market-rv20.adversarial.md` (a file of the design session, not committed).

1. Gap against the goal (blocking): adopted. Subjects no longer come from `listSubjects`; they are the manifest list
   `rv20_indices` mapped to ids by `IndexIDReader` (Decision 4). `iv_ibkr` has no index producer: not adopted as a
   thing to build; `iv_rv20` is dropped (the dispatcher's proposal) and the cell stays `not_built` with its existing text (Left out).
2. Biggest unnamed risk (id equality, exchange strings, `PriceBars` wiring): adopted. Id equality is now shown by the
   writer's own code (`run.go:86`, `job.go:109,116-118`, `run.go:181`) and the lookup keys on the same row; the
   unverified exchanges and the broker-up condition are named in Left out and surface as a named failed key or the
   existing warning.
3. Simpler way (rv20 only): adopted by the dispatcher's decision; issues for `Difference` and `iv_rv20` are gone, as is
   the 840-line squeeze of `internal/compute/api.go` (not edited).
4. Lock-in: adopted in part. The interface method that was hard to undo is gone. What remains is the rule id
   `market.rv20` v1 with its agreement lines, the manifest keys and the reading key `underlying|SPX`; all cheap to change
   except the rule id, which is the glossary's name.
5. Claim most likely false (subjects like `hv30_own`, `cmd/ot` needs no change, `iv` cell already shows `iv_ibkr`):
   adopted. Decision 5 and the Modules section are rewritten; `cmd/ot` needs a wiring issue (5); the `iv` cell claim is
   removed and the Gap is recorded in Left out.
6. Likeliest slip: adopted point by point. Routing in `Run`'s switch is Decision 5 and acceptance 5 of issue 4;
   `TestIvRvCellsAreNotBuilt` and `builder_test.go` (`pageGaps`) are in the Files of issue 6 with the `failed` counter
   answered in Decision 6; fakes of `compute.Figures` no longer need a new method; the cross-module issue sets are
   listed as judged cuts in Modules; acceptance items are results checked by `git grep` or by reading a named test, with no diff-shape or count-only items,
   "matches the design" lines are removed, and test runs are under "Checked by CI". The ratchet point is moot for `compute/api.go` and is the
   reason `IndexIDReader` is not in `store/api.go` (800 lines).
