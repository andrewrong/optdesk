# Macro page wiring: `ot page` and `ot serve` can build `macro_page`

Author: the optdesk dispatcher (revision 2, after the adversarial review and the advisor)
Source: the `macro_page` lines of `## Left out` of `docs/design/pages-registry.md` and of issue 109.
Read on origin/main at 95c5f1b (the one sha every fact below is read at; PRs 113, 117, 125 and 128 are merged there).

Facts on 95c5f1b. `internal/pages/macro_page` exists: `NewPage(c Config) pages.Page` returns `Meta{Name: "macro_page",
Schema: "page.macro_page.v1"}`, and `Config` is `RunID`, `BuildID`, `Scope` (each must match `^[A-Za-z0-9_-]{1,64}$`),
`Zone *time.Location` (nil is an error) and `Indicators`. `cmd/ot/wire.go` has 14 `deps` fields (`readEnv` is the last), among them
`pageList []pages.Page` holding `macro_rates.Page()`; `findPage(list, name)` in `cmd/ot/page.go` lists only the names of the list
it is given; `runPage` and `runServe` take `--name` (default `macro_rates`). The text of `modules.md` and the generated files already
allow the import (PR 117): the implementation list after `one constructor each` names `internal/pages/macro_page`, `arch.toml` and
`.go-arch-lint.yml` hold the component `pages/macro_page`, and `parseImpl` of `scripts/arch/main.go` reads that list without a count.
`internal/profile` holds `Name` and `LogFields`; `Load` reads `OT_HOME` and `OT_PROFILE` only. `cmd/ot/day.go` has `marketZone`, the
zone of the exchange, used only for the default `--day`. `docs/contracts/ot.md` has the table of environment names (`OT_HOME`,
`OT_PROFILE`, `OT_PG_ADMIN_DSN`, ...) and the profile rules. `checks.yml` runs `go run ./scripts/arch --check` without a paths filter.
In `cmd/ot` tests, `testApp` (`main_test.go`) builds its app through `newApp` and so through `profile.Load`, with a real-profile
`OT_HOME`; it sets only `deps.newClock` (a `fakeClock`). The tests of `page` and `serve` and of `runonce` build their app with it.

Goal of the design: `ot page --name macro_page` and `ot serve --name macro_page` build the macro page in the owner's zone, with a
fixed run id, scope and one indicator, and `macro_rates` stays the default and keeps working.

## Zone

The zone is a field of the profile, not a key of `.env` (`.env` holds secrets only; every other value is a setting, with a default
in code until the owner-settings store exists; and the owner's zone is written in no file of the repository).

- `profile.Profile` gains `Zone *time.Location`. The demo profile gives `time.UTC` and does not read `OT_ZONE`.
- The real profile reads `OT_ZONE` from the process environment (the runner's launchd environment, like `OT_PG_ADMIN_DSN`; never
  `.env`, never a flag), loads it with `time.LoadLocation`, and `Load` returns an error when it is unset, empty, `Local`, or not a
  name `time.LoadLocation` accepts. The error names the key and carries no value: `profile: OT_ZONE is not set`,
  `profile: OT_ZONE is not a valid zone name`. So the real binary refuses to start, for every command, without a valid `OT_ZONE`.
- This is an interface change of the module `repo` (`internal/profile/api.go`): one interface entry, and it lands before the
  issue that uses the field.
- No zone literal other than `UTC` appears in any file the issues touch. `marketZone` stays for the default day only and is not
  reused for the page.

## Modules

Landed in #117: `docs/design/modules.md` changed in that design's own diff, and `arch.toml` and `.go-arch-lint.yml` were regenerated in the same
diff (source and generated output travel together, AGENTS.md "Interfaces"). The proposed text of `modules.md` is the file
`modules.proposed.md`; three places differ from the text before #117:

1. The implementation list of the row `ot` gains `internal/pages/macro_page`, after `internal/pages/macro_rates`: twelve packages.
2. The marker words `these eleven` become `these implementation packages`, so the next package is one more name and no word of the
   text is a count.
3. In the `ot` row of the second table, "the eleven packages named above" becomes "the implementation packages named above".

No row, path, entry, tests or depends cell changes. Generated output: `arch.toml` gains the block `[component."pages/macro_page"]` and
a longer `ot` line; `.go-arch-lint.yml` gains `pages/macro_page` in the `ot` list and the generator moves its existing component to
table order. Neither file is edited by hand, and no claim is made about how many lines change.

### Order

Each merge is green: `checks.yml` runs `go run ./scripts/arch --check` on every pull request, so the generated files must be in the
same diff as the text. The parser no longer counts (issue 115, merged: it reads the list after the phrase `one constructor each`).

1. Landed in #117, this design's pull request: `docs/design/modules.md`, `arch.toml`, `.go-arch-lint.yml` and this file. Its head needs the design
   verdict and the normal review. The design review is started by the crew daemon, not by hand.
2. `ot-test-zone-env` (ot), landed in #128: every test of `cmd/ot` that builds a real profile sets `OT_ZONE=UTC` in its environment map. Green on the
   old `Load`, which ignores the variable. It must land before step 3, because step 3 makes `Load` refuse a real profile without
   the key, and without this step the tests of `cmd/ot` turn red on the merge of step 3.
3. `profile-zone` (repo, an interface change): the field and the rule above. Green after step 2. It is cut after step 1 (its `Design:` line
   names this file) and can be built in parallel with step 2; only its merge waits.
4. `ot-macro-page` (ot): the wiring. Needs steps 1 and 3 merged (the generated files allow the import, the field exists).

Step 4 may be built in parallel with step 3 against a fake profile, but it merges last. `Depends on:` in the issues names real issue
numbers only once they exist; until then the order above is the statement of it. The issue `ot-test-zone-env` is an addition to the
order that was asked for: it is the only way to keep step 3 green without letting an issue of module `repo` edit `cmd/ot`.

### What was run, and what was not

Run in a scratch worktree of origin/main, never the real checkout: the count-free parser of issue 115 with the old text: `go test ./scripts/arch/`
passes and `go run ./scripts/arch --check` prints `arch: ok`; with `modules.proposed.md` and the old generated files the check exits
1 (as it must); after `go run ./scripts/arch` it exits 0, `go test ./scripts/arch/` passes, and `go build ./...` and `go vet ./cmd/ot/`
pass. The regenerated files are `arch.toml.proposed` and `go-arch-lint.yml.proposed` in the scratchpad. Not run: `go-arch-lint`
itself (not installed here), the golangci-lint job, `checks.yml`, and any of the three issues' own tests (no issue is written yet).

## Interface

Interface change: `internal/profile/api.go` (module repo), issue `profile-zone`.

```go
// Profile is the profile name, the owner's zone and the whitelist of log fields.
type Profile struct {
	Name string
	// Zone is the owner's zone: time.UTC in the demo profile, the zone named by OT_ZONE in the real one. Never nil.
	Zone *time.Location
	LogFields []string
}
```

`LoadFunc` reads `OT_HOME`, `OT_PROFILE` and, for the real profile only, `OT_ZONE`. No other interface file changes
(`internal/pages/api.go` and `internal/pages/macro_page` stay as they are).

`macro_page.Config` is built only in `cmd/ot/wire.go`, because `ot` may import an implementation package only there. Other files see
a `deps` field `macroPage` of type `func(zone *time.Location, buildID string) pages.Page`, which `newDeps` assigns to a function of
`wire.go` that fills `Config` and calls `macro_page.NewPage`.

| value | source |
|---|---|
| `Zone` | `a.profile.Zone` |
| `RunID` | fixed `ot-page`: a label inside the page JSON only; `page` and `serve` write no ledger row (`ops` is not imported by them). If a later design stores it in `ops.capture_run` or any ledger row, it must then be unique per run, with `ot-page` as the scope or job name beside it |
| `BuildID` | `version` of `cmd/ot/version.go` with every character outside `[A-Za-z0-9_-]` replaced by `-`, cut to 64 characters, `dev` when empty |
| `Scope` | fixed `macro` |
| `Indicators` | one row: series `DGS10`, `Name` `10年期美债收益率`, `SourceText` `FRED`, no `DetailHref`. A literal, because the series list of the fred_series manifest (`internal/jobs/fredseries/job.toml`) is not reachable from `cmd/ot`: `parseManifest` is unexported and the package exports `New` only. The literal carries the comment "single source moves to the job manifest in a later design" |

Behaviour of the name:

| `--name` | what happens |
|---|---|
| `macro_rates` | as after issue 109 |
| `macro_page` | `a.deps.macroPage(a.profile.Zone, buildIDOf(version))`; a nil zone is an error that names `OT_ZONE` |
| anything else | an error built by `pageNamed` itself: `unknown page "x"; known pages: macro_rates, macro_page` (the names of `a.deps.pageList` in order, then `macro_page`); `findPage` is not changed |

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff, with an
offline grep or a named test. Every issue edits the files of one module only (`docs/design/modules.md`).

### ot-test-zone-env

The tests of `cmd/ot` that build a real profile set `OT_ZONE=UTC`
  Landed in #128 (commit 95c5f1b).
  Goal: the tests of `cmd/ot` stay green when `profile.Load` starts to refuse a real profile without `OT_ZONE`.
  Files: cmd/ot/main_test.go, cmd/ot/migrate_test.go, cmd/ot/setup_test.go, cmd/ot/calendar_test.go, cmd/ot/env_test.go
  Design: docs/design/macro-page-wiring.md#order
  Read at the base (offline: `grep -c '"OT_HOME"' cmd/ot/*_test.go`): these five files are the only ones that write an environment
  map or a getenv function with `OT_HOME`. The other test files (`day`, `page`, `serve`, `runonce`, `wire`) build their app through
  `testApp` of `main_test.go` and carry no `OT_HOME` of their own.
  Acceptance (all met in #128):
  1. Every environment map or getenv function of these five files that builds a real profile also gives `OT_ZONE` the value `UTC`
     (a helper may carry the pair once). Check: in each file, every line that holds `"OT_HOME"` also holds `"OT_ZONE"`, or the
     function around it holds `"OT_ZONE"` (`envApp` of `env_test.go` does so with a `k == "OT_ZONE"` branch); the counts of the two
     strings are not compared.
  2. `TestDemoProfile` still passes unchanged: the demo profile reads only `OT_HOME` and `OT_PROFILE`.
  3. `go test ./cmd/ot/` passes; no file outside the five is in the diff, and no file of `cmd/ot` that is not a test is touched.
  4. Every test added or changed calls `t.Parallel()`, uses `t.TempDir()` for any file, and none sleeps.
  Checked by CI: ci-go.
  Depends on: none (merged work only).
  Size: S

### profile-zone (interface)

The profile carries the owner's zone
  Goal: `profile.Profile` has `Zone *time.Location`; demo gives UTC; the real profile reads `OT_ZONE` and refuses a missing or invalid one, naming the key.
  Files: internal/profile/api.go, internal/profile/api_test.go, internal/profile/profile.go, internal/profile/profile_test.go
  Design: docs/design/macro-page-wiring.md#interface
  Acceptance:
  1. `api.go` declares the field `Zone *time.Location` with the comment of the Interface section, and the comment of `LoadFunc` names `OT_ZONE`; declarations only.
  2. `TestLoadZone` (in `profile_test.go`, table driven, a recording `getenv`): demo (no `OT_HOME`) gives `time.UTC` and `getenv` is never asked for `OT_ZONE`; real with `OT_ZONE=UTC` gives a zone whose name is `UTC`; real with the key unset, empty, `Local` and `no/such/zone` each gives an error whose text contains `OT_ZONE` and does not contain the value; `OT_PROFILE=real` without `OT_HOME` and the bad `OT_PROFILE` cases keep their present errors.
  3. `api_test.go` tests the contract from both sides: a fake `LoadFunc` that fills `Zone`, and a caller that reads it.
  4. No zone literal is added: `git diff origin/main -- '*.go' | grep -E '^\+.*"[A-Z][a-z]+/[A-Za-z_]+"'` prints nothing, and the only zone names in the added lines are `UTC`, `Local` and `no/such/zone`.
  5. Every test added or changed calls `t.Parallel()` and none sleeps; no file outside the four listed is in the diff (`git diff --stat` shows `cmd/` and `docs/` absent).
  Checked by CI: ci-go; the design verdict on the pull request (an interface change); the `cmd/ot` tests are green on the base only after `ot-test-zone-env` is merged.
  Depends on: ot-test-zone-env (the issue number is written when it exists). Order: after this design is merged; before `ot-macro-page`.
  Size: S

### ot-macro-page

`ot page` and `ot serve` build the page `macro_page` when `--name macro_page` is given
  Goal: the list of pages in `cmd/ot` has a second name, `macro_page`, built in the zone of the profile.
  Files: cmd/ot/wire.go, cmd/ot/pagelist.go, cmd/ot/pagelist_test.go, cmd/ot/page.go, cmd/ot/serve.go, cmd/ot/wire_test.go, cmd/ot/page_test.go, cmd/ot/serve_test.go, cmd/ot/testdata/macro_page_demo.golden.json, docs/contracts/ot.md
  Design: docs/design/macro-page-wiring.md#interface
  Acceptance:
  1. `wire.go` imports `internal/pages/macro_page` and no other file of `cmd/ot` does (offline: `grep -l 'internal/pages/macro_page' cmd/ot/*.go` lists `wire.go` only). `deps` gains the field `macroPage` of the type of the Interface section, so it has 15 fields; `newDeps` assigns it a function that builds `macro_page.Config{RunID: "ot-page", BuildID: buildID, Scope: "macro", Zone: zone, Indicators: <the DGS10 row>}` and returns `macro_page.NewPage(cfg)`; the literal carries the manifest note of the Interface table. `TestNewDepsFilled` counts 15 (it counts 14 on main) and keeps its `IsNil` loop.
  2. `pagelist.go` (new) declares `pageNamed(a *app, name string) (pages.Page, error)`: a name of `a.deps.pageList` returns that entry; `macro_page` returns `a.deps.macroPage(a.profile.Zone, buildIDOf(version))`, and a nil `a.profile.Zone` is an error whose text names `OT_ZONE`; any other name returns an error built in `pageNamed` whose text lists the names of `a.deps.pageList` in order and then `macro_page`. `findPage` is unchanged. `runPage` and `runServe` call `pageNamed` where they now call `findPage` (straight after flag parsing, before the clock is read, before `seriesReader` and before any listen); `pagelist.go` stays under 600 lines.
  3. `pagelist.go` declares `buildIDOf(v string) string` (`TestBuildIDOf`: `dev` stays `dev`, the empty string gives `dev`, `v1.2.3+abc` gives `v1-2-3-abc`, a 100-character input gives 64 characters, and every result matches `^[A-Za-z0-9_-]{1,64}$`).
  4. `TestPageNamedUnknown`: with a fake `pageList` of two fake pages the error contains both fake names, in order, and then `macro_page`; `TestPageMacroPage` (in `page_test.go`): the fake `deps.macroPage` records the zone and build id, `runPage --name macro_page` writes the bytes of the fake page, and the zone recorded is the pointer of `a.profile.Zone`; `TestPageMacroRatesIgnoresZone`: `--name macro_rates` succeeds and the fake `macroPage` is not called. A nil-zone case checks the error of item 2 and that `openSeries` is never called.
  5. `TestServeMacroPage` (in `serve_test.go`) builds through the same `pageNamed`; an unknown name binds no port (the technique of `TestServeUnknownNameBindsNoPort`).
  6. A real build, not a fake: `TestMacroPageRealBuild` (in `pagelist_test.go`) calls `newDeps().macroPage(time.UTC, "dev")` and its `Build` over a fake `store.SeriesReader` with one `DGS10` point and a fixed `generatedAt`; the output is valid JSON with `"schema"` equal to `page.macro_page.v1`, `run_id` equal to `ot-page`, `scope` equal to `macro` and `build_id` equal to `dev`. No network, no store, no clock.
  7. Demo golden in UTC: `TestMacroPageDemoGolden` (in `page_test.go`) runs `runPage --name macro_page --day 2026-11-05` over a demo app that the test builds itself, not with `testApp` (which loads a real profile and lives in `main_test.go`, a file this issue does not edit): `newApp` with a getenv that returns "" for every key, so `profile.Load` gives the demo profile and `Zone` is `time.UTC`, then `a.deps` set in `page_test.go` with a clock literal of the test and the `macroPage` function, and compares the bytes with `cmd/ot/testdata/macro_page_demo.golden.json`; the file is written by the test with a `-update` flag, never by hand, and holds no zone other than UTC.
  8. `docs/contracts/ot.md`: the table of environment names gains the row `OT_ZONE | process environment | the profile (real profile only; unset or invalid stops the start)`; the profile rules say that `Load` reads `OT_ZONE` for the real profile and that the demo profile has `UTC`; the subcommand part says `--name` accepts `macro_page`; the contract-test list names the new tests. The sentence of `TestDemoProfile` stays true (the demo profile reads only `OT_HOME` and `OT_PROFILE`).
  9. Ledger: `grep -c 'internal/store/ops' cmd/ot/page.go cmd/ot/serve.go cmd/ot/pagelist.go` prints 0 for each file, so the fixed run id reaches no ledger row.
  10. No zone literal is added: `git diff origin/main -- '*.go' | grep -E '^\+.*"[A-Z][a-z]+/[A-Za-z_]+"'` prints nothing (the whole-tree grep of the advisor would print `marketZone` of `day.go`, which is the exchange's zone, is old, and is unchanged; so the check reads the diff). The only zone-like names in added lines are `UTC`, the key `OT_ZONE` and `no/such/zone`.
  11. Every test added or changed calls `t.Parallel()`, uses `t.TempDir()` for any file, takes time only from the fake clock of `testApp`, and none sleeps (offline: `grep -c 'time.Sleep' ` on the four test files prints 0 for each); each changed Go file stays under 600 lines.
  12. No file outside the ten listed is in the diff (`git diff --stat` shows `internal/`, `go.mod`, `arch.toml`, `.go-arch-lint.yml` and `docs/design/` absent); `go run ./scripts/arch --check` exits 0.
  Checked by CI: `go build ./...`, `go vet ./cmd/ot/` and `go test ./cmd/ot/` pass; ci-go including go-arch-lint; the PR body carries the `Contracts:` line (set when the pull request is opened).
  Depends on: this design merged and `profile-zone` merged (issue numbers written when they exist).
  Size: M

## Left out

- The owner sets `OT_ZONE` in the runner's launchd environment (an owner action, a service-environment change). It must be set before the binary of `profile-zone` is deployed, because the real profile refuses to start without it; the dispatcher does not set it.
- The field moves to the owner-settings store (with a code default) when that store lands; `profile.Load` is the one place that changes.
- A default zone in code, or the reuse of `marketZone`: a default would put a personal choice in the repository, and the exchange's zone is not the owner's.
- More than one indicator, an indicator list read from the fred_series manifest (needs an exported reader in `internal/jobs`, an interface issue of its own), and the `DetailHref` links.
- A unique run id per run: needed only if a later design writes it to a ledger row.
- Removing `pages.BuildMacroRatesFunc` from `internal/pages` (already left out by issue 109); the old counts in `week1-ot.md`, `contract-docs.md` and `pages-registry.md` ("eleven", "13 fields"), which are history of earlier designs.
- A heading or table marker for the implementation list in `modules.md`, to replace the anchor phrase of issue 115.

## Adversarial

Review: `macro-page-wiring.adversarial.md` (against origin/main 6b28e47; kept on the dispatcher's machine). Six points.

1. Gap (base sha, `docs/contracts/ot.md` on main, `pageNamed` error): adopted in full. All facts are re-based on 6b28e47; `ot.md` is edited by the `ot` issue (it is in the `ot` row) with the `OT_ZONE` row; `pageNamed` builds its own known-names error and `findPage` is unchanged.
2. Biggest risk (the marker change turns `checks.yml` and ci-go red, in the design PR and on main): adopted. The marker is no longer changed before the parser; the order is parser, then the design PR with generated files, then the rest.
3. Simpler way (parser first, design PR with `modules.md` plus generated files, then the `ot` issue): adopted, with two differences: the anchor is the phrase `one constructor each`, since the text has no heading or marker and a `repo` issue may not edit `docs/`; and two issues are added between the design and the wiring (`ot-test-zone-env`, `profile-zone`) because of the zone.
4. Lock-in (`OT_ZONE` in `.env`, run id, one row): the `.env` key is replaced by the profile field, which the advisor required; the run id is a page label and the ledger acceptance (item 9) keeps it so; the single row is a literal with a note and a Left out item. The key name becomes a contract in `ot.md`, which is accepted.
5. False claim ("one added line", "already hold a component"): adopted. The design now says what the generator changes and every acceptance is `go run ./scripts/arch --check` exits 0, with no line count.
6. Likeliest slip (ordering, the `ot` PR with red go-arch-lint, hand-edited generated files): adopted. `Depends on:` and the Order section state the order; the generated files travel with the design PR, so no agent edits them; go-arch-lint is a Checked by CI item.

## Advisor

Answer of optdesk-advisor on 2026-11-06, recorded in `macro-page-wiring.advisor.md`, from the dispatcher's summary (the advisor did not open the draft). Adopted fully.

- Zone: a `Zone` field on `profile.Profile` (interface issue first); demo is UTC; the real profile reads the process environment variable `OT_ZONE` (the runner's launchd environment, never `.env`, never a flag), refuses to start when it is unset or not a valid name, and the error names the key. Left out: the field moves to the owner-settings store later. Reason: `.env` holds secrets only, and the owner's zone appears in no file of the repository.
- Order: parser count-free on an anchor first; then the design PR with `modules.md` and the regenerated files together (the rule, not an exception); then the `ot` issue. Adaptation: the anchor is a phrase, not a heading, and an extra test-fixture issue keeps the profile merge green.
- Fixed values: scope `macro` and DGS10 only are acceptable if the list is read from the manifest, else a literal with a note (the manifest is not reachable, so the literal and the note); the run id must be unique per run only if it reaches a ledger row (it does not).
- Acceptance: no zone literal other than `UTC` (adapted to a diff-based grep, see item 10) and a demo golden in UTC (item 7).
- Owner item: `OT_ZONE` in the launchd environment (Left out).
