---
type: design
title: "data check 2: the daily check also sees candidates written by an older rule version, earnings checks that fail or collapse, and bars whose open or close lies outside their own high and low"
status: accepted
date: "2026-11-27"
need: [none]
none_reason: "follow-up of docs/design/data-check.md (Left out); plan item P1; owner ask 2026-11-26 (data check) stands; no need record exists"
---

# data check 2: the daily check also sees candidates written by an older rule version, earnings checks that fail or collapse, and bars whose open or close lies outside their own high and low

Author: the optdesk designer
Owner approved: (pending)
Date: 2026-11-27
Plan: P1

Source: `docs/design/data-check.md` (landed; its `## Left out`: the rule-version item, held contracts and earnings coverage, row-level value checks) and the dataset sweep `state/optdesk-dataset-sweep-2026-11-26.md` (rows 9, 10, 36, 37). Every fact below was read on origin/main after `git fetch` on 2026-11-27 (first written on `63788f8d`, revised after main moved); the counts in `## Data read` come from read-only queries on the dev database on 2026-11-27, nothing was written. Repair stays out (data-check Left out, first item): the first design only reports and this one does too.

Owner-visible result: the block "数据检查" on `/data-health` can show three more kinds of finding, each in Chinese with a short reason beside it. (1) "候选的规则版本落后" (candidates written by an older version of the put rule: the stored rows were made by code that has since changed, so they must be recomputed). (2) "财报日检查失败或突然查不到" (an earnings check returned errors, or a market that had dates the day before has none). (3) "日线开收价超出最高最低价" (a daily bar whose open or close lies outside its own high and low, which cannot be true). A name that the provider simply has no date for is NOT a finding: it stays "absent" as today, with no invented reason. Nothing is repaired, deleted or overwritten.

摘要：数据检查第一版留下三件事，这里做完。一，候选行里存着“规则版本”，但改了规则代码这个版本从没变过（现在还是 put_v1），所以“候选是用旧规则算的”看不出来；改为版本随规则变，检查把存的版本和当前代码的版本对一下，不同就报。二，财报日：只有“检查本身出错”和“昨天有日期今天一个都没有”才报，提供方没有日期不报（那是正常的，不编理由）。三，日线：开盘、收盘必须落在最高最低之间，这是恒等式，不需要给价格区间定数字。价格带、隐含波动率带仍然不做，因为没有有出处的数字。页面只多三行文字，前端不算任何东西。

## Frame

- The check (`internal/jobs/datacheck`, job `data_check`) has six rules and a probe (`finding.Probe`, implemented in `internal/store/findingpg`). A rule is one function of an `Input`; a rule id is a fixed string and a new rule adds one (`internal/store/finding/api.go`). The page builder words each rule from a table in `internal/pages/data_health_page/datacheck.go` and `zh_texts.go`; a rule id the page does not know shows alone with the generic reason `zhRuleWhyOther`. So a rule added without a page entry would show as a bare id; entry 5 is therefore not optional.
- `rule_version` is a column of every candidate row (`migrations/0018_candidates.sql`, `rd.option_candidate_r1`). The job writes it from one constant, `ruleVersion = "put_v1"` (`internal/jobs/optioncandidates/convert.go:22`). The put rule has changed since the constant was written (for one, `d3d8e42f` "a put with its strike above the buy price passes the gate", and `4db2f595` the spot fallback) and the constant did not move. Sweep rows 36 and 37 were exactly this case: rows computed before a rule change still stored as if current. The existing rule `candidates_older_than_settings` sees only a change of stored settings (`SettingsChangedAt`), not a change of code.
- `internal/jobs/optioncandidates` may not import another jobs package, and `internal/jobs/datacheck` may not import `optioncandidates` (`docs/design/modules.md`, row jobs: "importing another jobs package"). The expected value therefore comes through `cmd/ot`, as `Expected` does today (`cmd/ot/wire_datacheck.go`, `datacheck.Stores.Expected`).
- Earnings: `obs.earnings_check` has one row per (issuer, observed_on) with `status` in `listed`, `absent`, `unsupported`, `error` and an `error_code` (`migrations/0046_fundamentals_keys.sql`); the view `rd.earnings_date_r1` carries `observed_on`, `market`, `status`, `error_code` per issuer and day. The sweep (rows 9, 10) found 4 of 35 US names and 1 of 7 JP names listed each day, no `error_code`. Whether the other names have no date at the provider or our matching drops them was not verified, and `docs/design/earnings-fallback.md` added a second source for exactly those. So the count of `absent` names is not a defect signal and no number of "expected listed" names exists. What is a signal without any expectation: an `error` status, and a market whose `listed` count falls from above zero to zero between two checked days.
- Bars: `obs.price_bar` stores prices as text with a constraint that each is a positive number (`migrations/0021_bars.sql`); nothing says `low <= open <= high` or `low <= close <= high`. That is an identity of what a bar is, not a band; a bar that breaks it is wrong data stored as good. `rd.price_bar_r1` is the view of the newest adjust epoch. The role `ot_w_datacheck` reads ten views (`migrations/0073_data_check.sql`; its pg test lists eight of them), and neither `rd.price_bar_r1` nor `rd.earnings_date_r1` is among them, so one migration of grants is needed.
- `put_v1` outside the job: `git grep -n put_v1` over `contracts/`, `migrations/`, `web/` and `pyside/` finds nothing, and no schema enum, SQL view or contract pins the text; the only non-test hits are `internal/jobs/optioncandidates/convert.go` (the constant this design moves) and `internal/strategy/shortput/shortput.go` (`RuleVersion` of `strategy_result`, a different dataset, not changed). So the bump is not a contracts change and needs no entry beyond entry 3; every other hit is a test literal of `strategy_result` rows.
- Quotes with `bid > ask` are already counted by the base-data views (`migrations/0057_base_fact_option.sql`, `0062_base_fact_option_open.sql`); this design does not repeat them.

## Data read

Read-only queries on the dev database (role `optdesk_dev_ro`), 2026-11-27, newest adjust epoch (`rd.price_bar_r1`):

- `bar_ohlc_shape` (open or close outside high/low, or low > high). Basis trades: ibkr USD 5908 rows, 0 bad; ibkr JPY 1530 rows, 0 bad; yfinance USD 427170 rows, 0 bad; yfinance JPY 46913 rows, 1 bad (instrument 2412, session day 2002-04-01, open 1.94 under low 2.04: a provider quirk of an old bar). Basis adjusted_last: 0 bad except the same 2002 bar. So the identity is clean on live data and the rule would raise nothing today; the one bad bar lies outside any check window. Both `ibkr` and `yfinance` rows exist (the source check of `0021_bars.sql` was widened later), so the rule does not name a source and judges every source alike.
- Adjusted versus raw: for the 474075 days that have both bases from yfinance, `close` differs in 398868 and `high` differs in the same 398868, so an adjusted series adjusts the whole bar, not the close alone. The rule still judges basis `trades` only (as Decision 5), the safe choice.
- `earnings_coverage`, from `obs.earnings_check` (14 checked days per market, 2026-11-09 to 2026-11-27): no market-day has `listed` falling to 0 (US lowest 4, JP lowest 1 once checks answer). US Thanksgiving 2026-11-26 has no check row for either market, so a holiday gives no row and rule case (b) never compares across it. Case (a) would have fired for JP on 2026-11-09, 10, 11 and 12 (8 names, 8 errors, 0 listed each day), which was a real failure of the JP source and is a true positive. The counts also show `listed` jumping from 4 to 29 (US) and 1 to 7 (JP) on 2026-11-27 after the earnings-fallback source arrived; that is a rise, not a finding. Result: no tolerance or calendar change is needed, because the rule compares only days that have check rows; severity stays `warn`.
- One consequence for the page: on the first run `earnings_coverage` shows nothing for the latest window (no error rows after 2026-11-12) unless the window reaches back to those four JP days.

## Owner rules and how this design meets them

1. Bugs first. Each of the three rules answers a wrong value or silent stale data that the sweep found or that the identity says cannot happen; no feature is added. The migration (entry 2) is grants only.
2. The outcome is visible to the owner on a page. Entry 5 gives each rule a Chinese label, a one-sentence reason with the term glossed beside it ("规则版本 = 算候选时用的那套规则的编号"), and the counts. The `data_check` block already exists; no new page.
3. No made-up data: gaps stay gaps with a reason. An `absent` earnings name is not turned into a finding or a reason. A rule that cannot decide returns an error and is stored as `unknown`. A bar that breaks the identity is reported with its counts only; the job does not say which price is right.
4. Every job runnable on its own and idempotent. No new job. `data_check` is unchanged in its shape: a rerun is a new run, findings are keyed by (rule, dataset, day, subject) and a repeated finding is not new (data-check Decision 8).
5. The front end renders data and computes nothing. Entry 5 is `internal/pages`; `web/` is not touched because the block prints the strings and flags the builder gives it. Self-check: no arithmetic and no wording table in `web/`.
6. "Our code is no hard limit" / dev mode: no time guard is added. The earnings rule compares two stored days, it does not look at the clock.

## Decisions

1. **The rule version is a fact of the job, exported, and compared by value.** `optioncandidates` exports `RuleVersion` (the constant, renamed from `ruleVersion`) and moves it from `put_v1` to `put_v2` in the same entry: the put rule has changed since `put_v1` was set, so every candidate row stored before this entry is, correctly, from an older version. After deploy the rule reports the newest run of each kind as old until the next candidate run replaces it; that is the true state, not noise. `cmd/ot` hands the exported value to the check as `Stores.RuleVersions` (dataset id to version).
2. **A version bump cannot be forgotten silently.** A test in `optioncandidates` runs the conversion on fixed inputs and compares a digest of the output rows, written with `RuleVersion` in a table of `version: digest`. A change of the rule's output changes the digest and fails the test with the text "bump RuleVersion and add the new digest"; the old row stays in the table. Own implementation, because no library hashes a function's behaviour; it is thirty lines of test code and no run-time dependency. It cannot see a rule change that leaves the fixed inputs' output the same, which is said in Left out.
3. **`candidate_rule_version`: the newest candidate run of each kind in the window is compared with the expected version of its dataset.** Different: one `warn` finding (not `error`: rows are stored as computed by a known older rule, not corrupt), dataset `candidates.option_candidate`, day of the run, evidence `rows`. The version texts are not in the evidence (a `Count` holds a number only); the page reason says "存的版本与当前代码的不同" and names no version. No expected version in `Stores` for the dataset: the rule returns the error `no_expected_version`, stored as `unknown`.
4. **`earnings_coverage`: only two cases.** Per market (US checked on the XNYS window, JP on the XTKS window) and window day with at least one check row: (a) any row with status `error`: one `warn` finding, dataset `obs.earnings_date`, evidence `names`, `listed`, `errors`; (b) no `error` and `listed` is 0 while the previous window day with check rows of that market had `listed` above 0: one `warn` finding, evidence `names`, `listed` 0, `listed_before`. A market with no check row on a window day gives no finding here (a missing day is `capture_gap` or `plan_absent`). `absent` and `unsupported` rows are never a finding. Severity `warn`, never `error`: nothing says a stored date is wrong.
5. **`bar_ohlc_shape`: a bar is wrong when low > high, or open or close is below low or above high.** Over `rd.price_bar_r1`, basis `trades` only (an adjusted series may differ by rounding of adjustment and is not judged), for the window days of XNYS and XTKS by the day's `session_day`. The probe answers per day the rows and the rows that break the identity (`bad`); a day with `bad` above 0 gives one `error` finding, dataset `obs.price_bar`, evidence `rows`, `bad`. No band, no number is invented: the rule has no parameter.
6. **No `Subject`.** As in data-check, a finding is per dataset and day (`Subject` empty); which instrument is wrong is a question for the page of base data, not for this block (Left out).
7. **One migration, grants only.** `ot_w_datacheck` gains SELECT on `rd.price_bar_r1` and `rd.earnings_date_r1` and nothing else; Down revokes the same two. Number 0075 is unclaimed: `migrations/` ends at `0074_gateway_state.sql`, no other design or open pull request names 0075. `migrations/` is not an owner-only path of the project declaration (modules.md lists only `.github/`, `contracts/proto/`, `deploy/`, `pyside/src/optdesk/ibkr/transport.py` and `docs/design/ratchet.json`), so the gate merges it without the owner. A migration holds the auto-deploy; the window runs `ot setup pg`, `migrate`, `setup pg` after the merge (owner rule 2026-11-09).
9. **How `candidate_rule_version` clears.** After the deploy and the migrate step the window force re-runs `option_candidates` (console re-run with `-force`; dev mode allows the overwrite) for the newest session day, then runs `data_check` once. The rule compares only the newest candidate run of each kind, so one forced re-run of the newest day clears the finding; older days stay `put_v1` and are not judged. No code or page action clears it, and the finding is not hidden by the check.
8. **Interface change is declarations only and one entry.** `finding.api.go` gains three rule ids, two `Probe` methods with their two answer types, one field on `CandidateRun` (`RuleVersion string`) and the new count names in the `Count` doc. No existing name changes meaning; every fake `Probe` in a test of another package embeds the interface (`cmd/ot/wire_datacheck_test.go` `fakeFindingProbe`, `internal/jobs/datacheck` `fakeProbe`, `gapProbe`, `wrongProbe`), so the two new methods need no edit there; the only implementer, `pgProbe` in `internal/store/findingpg`, does not compile without them, which fixes the cut below.

## Modules

`docs/design/modules.md`: no row changes and no dependency edge is added. First row that matches decides: store (`internal/store/**`, `migrations/**`, `docs/contracts/store/**`: entries 1, 2), jobs (`internal/jobs/**`, `docs/contracts/jobs/**`: entries 3, 4), pages (`internal/pages/**`: entry 5), ot (`cmd/**`, `docs/contracts/ot/**`: entry 6). `internal/store/finding` is already in the entry cell of row store. The cut: the interface and its only implementer are both in module store and must compile together, so entry 1 carries `api.go` and the whole of `findingpg` (the methods and the `rule_version` read); entry 2 is then the migration and the `pg` tests. No fake outside store needs an edit. Import edges used, all existing: `datacheck` imports `finding`; `findingpg` implements `finding`; `data_health_page` imports `finding`; `cmd/ot` imports `datacheck` and `optioncandidates`. `datacheck` does not import `optioncandidates`.

## Interface

One shared interface changes: `internal/store/finding/api.go` (entry 1), used by `findingpg` (store), `datacheck` (jobs) and `data_health_page` (pages). Declarations added:

```go
const (
	RuleCandidateRuleVersion = "candidate_rule_version"
	RuleEarningsCoverage     = "earnings_coverage"
	RuleBarOHLCShape         = "bar_ohlc_shape"
)

// CandidateRun gains: RuleVersion string // the rule_version of the newest run's rows, "" when rows disagree
// Probe gains:
//   EarningsChecks(ctx context.Context, from, to contracts.Day) ([]EarningsCheck, error)
//   BarShapes(ctx context.Context, from, to contracts.Day) ([]BarShape, error)

// EarningsCheck is the answer of EarningsChecks for one market ("US" or "JP") and day: the check rows, those
// with status listed, and those with status error.
type EarningsCheck struct {
	Market               string
	Day                  contracts.Day
	Names, Listed, Errors int
}

// BarShape is the answer of BarShapes for one day: the trades bars and those that break low <= open, close <= high.
type BarShape struct {
	Day       contracts.Day
	Rows, Bad int
}
```

The `Count` doc adds the names `names`, `listed`, `errors`, `listed_before`, `bad`. Nothing else in `finding` changes. No file under `contracts/schemas`, `contracts/datasets` or `contracts/proto` changes (the page schema carries a rule id and strings, not a list of rules).

## Issues

Build order (the cut: entry 1 holds the interface and its only implementer, both module store, so it compiles alone; entry 2 is the migration; no module other than store is touched before entry 3): a draft names an entry as `Design: docs/design/data-check-2.md#Entry <n>`. Every entry compiles and its tests pass against main. Entry 2 is the only migration, holds the auto-deploy and is merged by the gate (`migrations/` is not owner-only). Order: 1 first; 2 after 1; 3 first (independent of 1); 4 after 1 and 3; 5 after 1; 6 after 2, 3 and 4. Test runs are in `## Checked by CI`. Each entry is one module and lists only paths of that module. The design's own pull request carries `Contracts: unchanged`.

### Entry 1 data-check-2-store-api (interface and its implementer)

Module: store. Size: M.
Order: first.
Files: `internal/store/finding/api.go`, `internal/store/finding/api_test.go`, `docs/contracts/store/finding.md`, `internal/store/findingpg/probe.go`, `internal/store/findingpg/probe_more.go` (new), `internal/store/findingpg/probe_more_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check-2.md#Interface (the interface and its one implementer `pgProbe` land together so the store module compiles)
Acceptance:
1. `api.go` declares what the Interface section lists (the three rule ids, `CandidateRun.RuleVersion`, the two `Probe` methods, `EarningsCheck`, `BarShape`, and the new count names in the `Count` doc) with docs and no function body; the file stays declarations only and no other non-test `.go` file is added to the package.
2. `grep -c 'candidate_rule_version\|earnings_coverage\|bar_ohlc_shape' internal/store/finding/api.go` prints at least 3; the six earlier rule ids are unchanged (`git diff BASE -- internal/store/finding/api.go` shows no removed or edited line of them).
3. `api_test.go` gains a table that the nine rule ids are distinct and non-empty and that a compile-time assertion `var _ finding.Probe = fakeProbe{}` holds with the two new methods (the contract seen from the user side); `docs/contracts/store/finding.md` lists the three ids, the two methods and the two answer types.
4. `probe_more.go` implements `EarningsChecks`: one answer per (market, day) from `rd.earnings_date_r1` with `Names` the rows, `Listed` those with status `listed`, `Errors` those with status `error`; days with no rows give no answer. `probe.go` selects `rule_version` in the candidate query and sets `CandidateRun.RuleVersion` to the one value when every row of the newest run has the same, to the empty text otherwise.
5. `BarShapes` reads `rd.price_bar_r1` with basis `trades` and a `session_day` in the inclusive window, and returns per day `Rows` and `Bad`, where a bar is bad when `low > high`, `open < low`, `open > high`, `close < low` or `close > high`, comparing as numbers (the columns are text; the table constraint guarantees each parses).
6. `var _ finding.Probe = (*pgProbe)(nil)` still compiles, and `go build ./...` and `go vet ./...` pass with no other file edited: the fakes of other packages (`fakeFindingProbe` in `cmd/ot`, `fakeProbe`, `gapProbe` and `wrongProbe` in `internal/jobs/datacheck`) embed `finding.Probe` and need no change.
7. `probe_more_test.go` (new) checks the SQL text of the two new queries is read-only (no INSERT, UPDATE, DELETE) as the existing probe test does.

### Entry 2 data-check-2-probe

Module: store. Size: S.
Order: after 1.
Files: `migrations/0075_data_check_inputs.sql` (new), `internal/store/findingpg/probe_more_pg_test.go` (new), `internal/store/setup/data_check_pg_test.go`
Contracts: unchanged
Design: docs/design/data-check-2.md#Decisions (5, 7)
Acceptance:
1. `migrations/0075_data_check_inputs.sql` (new) has one Up statement `GRANT SELECT ON rd.price_bar_r1, rd.earnings_date_r1 TO ot_w_datacheck;` and a Down that revokes the same two, and nothing else; `git diff --name-only BASE -- migrations` prints this one file.
2. `probe_more_pg_test.go` (new, build tag `pg`, GitHub CI only) inserts, in a temporary schema of the test, three US check rows (listed, absent, error) and gets `{Names 3, Listed 1, Errors 1}`, and bars `{O 10 H 12 L 9 C 11}`, `{O 13 H 12 L 9 C 11}`, `{O 10 H 12 L 9 C 8}` and gets `{Rows 3, Bad 2}`, and one adjusted_last bar that is not counted.
3. `data_check_pg_test.go` is edited so that the list of input views read by `ot_w_datacheck` has ten entries, the two new ones included; the role still has no INSERT, UPDATE or DELETE on any `rd` view or `obs` table (the test's existing negative part is unchanged).

### Entry 3 data-check-2-rule-version

Module: jobs. Size: S.
Order: first.
Files: `internal/jobs/optioncandidates/convert.go`, `internal/jobs/optioncandidates/version.go` (new), `internal/jobs/optioncandidates/version_test.go` (new), `internal/jobs/optioncandidates/testdata/rule_digest_input.json` (new), `docs/contracts/jobs/option_candidates.md`
Contracts: unchanged
Design: docs/design/data-check-2.md#Decisions (1, 2)
Acceptance:
1. `convert.go` no longer declares `ruleVersion`; `version.go` declares the exported `const RuleVersion = "put_v2"` with a doc that says it changes whenever a rule of the put gates or tiers changes, and `convert.go` writes `RuleVersion` in the `rule_version` field of every row (the line that held `ruleVersion`). `grep -rn 'ruleVersion' internal/jobs/optioncandidates` prints nothing.
2. `version_test.go` (new) has `TestRuleVersionDigest` (new): it converts the rows of `testdata/rule_digest_input.json` (new) (a fixed input set of at least 8 underlying and chain rows covering a passing put, a blocked put, a put above the buy price, and a name with no quote) and compares the sha256 of the JSON of the output rows with the entry of `ruleDigests` for `RuleVersion`; the map has the one row `put_v2`. A version with no row in the map, or a digest that differs, fails with a message that contains "bump RuleVersion and add the new digest".
3. `TestRuleVersionDigest` (new) is the only test that reads the digest; the existing tests of the package pass unchanged (none of them asserts the text `put_v1` for a candidate row: `grep -rn 'put_v1' internal/jobs/optioncandidates` prints only `shadow_test.go`, whose value comes from the strategy package and is not changed).
4. `docs/contracts/jobs/option_candidates.md` says that `rule_version` is `RuleVersion`, that the data check compares it with the stored value, and that a change of the rule's output bumps it and adds a digest row.

### Entry 4 data-check-2-rules

Module: jobs. Size: M.
Order: after 1 and 3.
Files: `internal/jobs/datacheck/rule.go`, `internal/jobs/datacheck/job.go`, `internal/jobs/datacheck/rules.go`, `internal/jobs/datacheck/rules_more.go` (new), `internal/jobs/datacheck/rules_more_test.go` (new), `docs/contracts/jobs/data_check.md`
Contracts: unchanged
Design: docs/design/data-check-2.md#Decisions (3, 4, 5, 6)
Acceptance:
1. `Stores` and `Input` gain the field `RuleVersions map[string]string` (dataset id to expected version); `New` accepts it nil (the rule then returns `no_expected_version`); `rules_more.go` has `ruleCandidateVersion`, `ruleEarningsCoverage` and `ruleBarShape`, and `defaultRules()` returns nine rules, the six of data-check first and these three after them in that order.
2. `ruleCandidateVersion`: with expected `{candidates.option_candidate: put_v2}` and newest runs `{Day D, Kind put, Rows 40, RuleVersion put_v1}` it returns one `warn` finding, dataset `candidates.option_candidate`, day D, evidence `rows` 40; with `RuleVersion put_v2` none; with `RuleVersion ""` (rows disagree) one finding; with no expected entry it returns the error `no_expected_version`. `TestCandidateRuleVersion` (new) is a table of these rows.
3. `ruleEarningsCoverage`: `{US, D, Names 35, Listed 4, Errors 0}` gives none; `{US, D, Names 35, Listed 0, Errors 35}` gives one `warn` finding, dataset `obs.earnings_date`, evidence `names` 35, `listed` 0, `errors` 35; `{US, D1, Names 35, Listed 4}` followed by `{US, D2, Names 35, Listed 0, Errors 0}` on the next XNYS window day gives one finding for D2 with evidence `names` 35, `listed` 0, `listed_before` 4; `{JP, D, Names 7, Listed 1}` gives none; a market with `Listed 0` on every window day and `Errors 0` gives none (no earlier day with dates). `absent` rows are not an input of the rule. `TestEarningsCoverage` (new) covers these, and a US holiday absent from the XNYS window is not compared as a "previous day".
4. `ruleBarShape`: `{Day D, Rows 5000, Bad 0}` gives none; `{Day D, Rows 5000, Bad 3}` gives one `error` finding, dataset `obs.price_bar`, evidence `rows` 5000, `bad` 3; a day outside both the XNYS and XTKS windows gives none. `TestBarShape` (new) covers these.
5. A probe method that returns an error makes the rule return that error (the job stores it as `unknown`, as for the six); `TestMoreRulesReturnProbeError` (new) covers each of the three. `TestJobRunsNineRules` (new) shows a run with a fake probe stores `RulesTotal` 9.
6. `docs/contracts/jobs/data_check.md` lists the three rules with their severity and evidence names.

### Entry 5 data-check-2-page

Module: pages. Size: S.
Order: after 1.
Files: `internal/pages/data_health_page/datacheck.go`, `internal/pages/data_health_page/zh_texts.go`, `internal/pages/data_health_page/datacheck_more_test.go` (new)
Contracts: unchanged
Design: docs/design/data-check-2.md (Frame, first bullet; Owner rules 2, 5)
Acceptance:
1. `ruleLabel` returns for the three new ids a Chinese label plus the id in brackets: "候选的规则版本落后 (candidate_rule_version)", "财报日检查失败或突然查不到 (earnings_coverage)", "日线开收价超出最高最低价 (bar_ohlc_shape)"; `ruleWhy` returns a one-sentence reason for each that glosses its term in the same sentence: the first says "规则版本是算候选时那套规则的编号，存的编号和现在代码里的不同，说明这些候选是旧规则算的，需要重算"; the second says "检查出错，或昨天有日期今天一个都没有；提供方没有日期不算问题"; the third says "一根日线的开盘或收盘价必须落在它自己的最高价和最低价之间，不满足就是存错了".
2. `evidenceText` words `names`, `listed`, `errors`, `listed_before` and `bad` as Chinese sentence parts with each count named ("检查 35 个，查到 0 个，出错 35 个"; "昨天查到 4 个，今天 0 个"; "共 5000 根，不合格 3 根"), still with no rule that the web knows; `grep -rn 'candidate_rule_version\|earnings_coverage\|bar_ohlc_shape' web/src` prints nothing.
3. `datacheck_more_test.go` (new) builds the block from findings of the three rules with a fake `Reader` and shows each row has the Chinese label, the reason and the evidence sentence, none shows the generic `zhRuleWhyOther`, and the existing six rows are unchanged in text and order; the JSON validates against `contracts/schemas/page.data_health_page.v1.json` unchanged.

### Entry 6 data-check-2-wiring

Module: ot. Size: S.
Order: after 2, 3 and 4.
Files: `cmd/ot/wire_datacheck.go`, `cmd/ot/wire_datacheck_test.go`, `docs/contracts/ot/jobs/data_check.md`
Contracts: unchanged
Design: docs/design/data-check-2.md#Decisions (1)
Acceptance:
1. `wire_datacheck.go` fills `datacheck.Stores.RuleVersions` with `{"candidates.option_candidate": optioncandidates.RuleVersion}`; it adds no other logic and no other import of a job package.
2. `TestDataCheckRuleVersionsWired` (new) opens the job through the existing opener seam and shows the `Stores` it received holds that one entry equal to `optioncandidates.RuleVersion`.
3. The existing tests of the file pass unchanged, and `go run ./scripts/arch` reports no new import edge outside `docs/design/modules.md`.
4. `docs/contracts/ot/jobs/data_check.md` says where the expected version comes from.

## Checked by CI

`go test ./...` (entries 1 to 6; the `pg` tests of entry 2 run only in GitHub CI), `go run ./scripts/arch` and go-arch-lint, the migration check with PostgreSQL 17 and TimescaleDB, `go run ./scripts/precheck`. No web change, so `ci-web` is not triggered by this design. After deploy the window runs `ot setup pg`, `migrate`, `setup pg`, runs `data_check` once from `/job-console` for the latest session day, renders `/data-health` and checks the block with a screenshot (owner rule of 2026-11-14); that check is not an acceptance item. Expected on that first run: `candidate_rule_version` shows, because the stored candidate rows are `put_v1`; the window then force re-runs `option_candidates` for the newest session day (Decision 9) and runs `data_check` again, and it clears.

## Left out

- **Repair.** Same as data-check: report only. A finding with a repair button is a later design.
- **A rule change that leaves the digest inputs' output the same.** Entry 3's digest sees only what the fixed inputs exercise; a change that touches a case the inputs do not cover can still go without a bump. More inputs make the test stronger; the owner-visible fix for such a case is a recompute by hand.
- **Row-level value bands** (price outside a band, IV outside a band, spread too wide). Still no stated band with a source; "numbers sourced or trial" (owner 2026-11-13) applies. This design adds only an identity that needs no number. A band comes with its own entry once a source is named.
- **Per-instrument findings** (`Subject`). The block says which day and dataset, not which instrument; naming the instrument needs a page design for the block's rows.
- **Earnings: whether `absent` is right.** Whether the 31 US names with no date are not announced or are dropped by our matching is not verified (sweep row 9); no rule judges it. If the owner or a probe settles it, it becomes one more rule.
- **Held contracts with no end-of-day quote** (sweep rows 17, 18). Needs positions as input; not in this design.
- **Wrong candidates from wrong inputs (row 37), stale readings other than the four named (row 33), rows that vanish (row 23).** Unchanged from data-check.
- **Quiet hours, a check on a day XNYS is closed, a silent first run.** Unchanged from data-check (Left out, Doubts 1 and 4).
- **Strategy results (`strategy_result`) carry their own `rule_version`** (`internal/strategy/shortput.RuleVersion`, still `put_v1`). Not compared here; it is a different dataset with a different writer.

## Doubts

1. A provider may in rare cases deliver an open or close outside high and low (live data: 0 of 7438 ibkr trades bars, 1 of 474083 yfinance trades bars, see `## Data read`). If a live run shows such bars every day, `bar_ohlc_shape` reports a real source behaviour, not a defect of our storage; the rule is then narrowed in a follow-up by the evidence, not by a guessed tolerance.
2. After entry 3 deploys, every day's candidates remain `put_v1` until the window's forced re-run (Decision 9); the finding sits on the page until then. The Telegram message of the first run lists it as new (data-check Decision 8).
3. `earnings_coverage` case (b) can be right for a legitimate reason (the provider's list of near-term reporters really emptied). The reason text says so; it is a `warn`, not an `error`.
4. `rd.earnings_date_r1` is the check table `obs.earnings_check` left joined to the date row (migration 0046) and has one row per (issuer, observed_on); the table's counts were read from the dev database (`## Data read`).

## Adversarial

Self-written by the designer, revised on current origin/main; no sub-agent or advisor has read it yet.

- **Gap versus the goal.** The goal of P1 is every stored dataset repairable and visible on a page. This design adds three visibility rules and no repair, and no rule for datasets outside the sweep's rows (dividends, FX rates, macro events, fundamentals). Visible on a page: yes for the three; repairable: no, by the first design's decision. The gap is real and stated in Left out.
- **Biggest unnamed risk.** The `put_v2` bump makes a finding appear on day one that is "true" but looks like a failure to the owner. The Chinese reason says it needs a recompute, but the page has no recompute button for candidates; the owner's way out is the existing job console. If the owner reads it as broken, the design has failed its owner-visible goal even though every test passes.
- **Simpler way.** Skip the digest test and bump by hand. Rejected: the sweep already showed a hand bump is forgotten (`put_v1` through two rule changes). A simpler way for the earnings rule would be to alert on `error` only; kept both cases because the JP/US collapse to zero is the one silent failure we have seen shape of (a key or matching break) and it needs no expectation.
- **Lock-in.** The count names and three rule ids are permanent strings (a rule id never changes). The digest table `version: digest` is a convention in one package; replacing it later is local. `Stores.RuleVersions` is a map keyed by dataset id, open to the next dataset that carries a version.
- **Claim most likely false.** "A bar with open or close outside its high and low cannot be true" (Frame, Decision 5). It is true of a bar as defined, but IBKR can deliver such bars from late prints; Doubt 1 says what happens then. The earlier claim that a fake of another package breaks from the new `Probe` methods was checked by grep and is false: all four fakes embed the interface.
- **Likeliest slip.** Entry 3's digest input set is too small, so the test passes while a rule changes (named in Left out), or the implementer picks the digest by running the code once and the test then protects nothing about intent. The acceptance names the four cases the inputs must cover; a reviewer should read the input file, not only the test.
