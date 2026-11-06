# Pages: the Page, Meta and Builder names, and the adapters

Author: advisor of optdesk (revision 2 after the adversarial review)
Source: `## Left out` of `docs/design/pages-builders.md` (the `Page`, `Meta` and `Builder` names, and the adapter that makes a
builder a value). Read on main at c63cbb4: `internal/pages/api.go` holds `BuildMacroRatesFunc` and `NewHandlerFunc`;
`internal/pages/macro_rates.Build(ctx, r, day, generatedAt)` and `internal/pages/macro_page.Build(ctx, r, c, day, generatedAt)`
(with `macro_page.Config`: `RunID`, `BuildID`, `Scope`, `Zone *time.Location`, `Indicators`; and `macro_page.Indicator`: `SeriesID`,
`Name`, `SourceText`, `DetailHref`) are the two builders; both packages exist. `cmd/ot/page.go` and `cmd/ot/serve.go` call
`a.deps.buildMacroRates`, a field of `deps` in `cmd/ot/wire.go` (11 fields, `TestNewDepsFilled`). Open issue 75 edits `cmd/ot/wire.go`.

Goal of the design: freeze the three names so a page is a value (`Page`: a `Meta` and a `Builder`) and make both builders
values through one constructor each. The wiring is not part of this design: `ot` may import an implementation package only
from `cmd/ot/wire.go` (the rule of `docs/design/week1-ot.md` and of issue 75's acceptance grep), and `wire.go` is held by
issue 75. The page list and the `--name` flag of `ot page` and `ot serve` follow in an `ot` issue after issue 75 merges (see
`## Left out`). A registry type is not added: two pages and one caller are served by a `[]pages.Page` and a lookup loop.

## Modules

none. Every path below belongs to the row `pages` (`internal/pages/**`, `docs/contracts/pages.md`). No new package is made, so
`modules.md`, `arch.toml` and `.go-arch-lint.yml` are unchanged and `ot` gains no import edge. No entry edits `go.mod`,
`go.sum`, `contracts/`, `web/`.

| issue | module |
|---|---|
| pages-page-values (interface) | pages |

## Interface

Owner: `pages`, file `internal/pages/api.go`. Three declarations are added; the two existing ones are unchanged.

```go
// Meta names a page. Name is the word a command takes (--name). Schema is informational: the contract file
// the view model fills (page.<name>.v1); no code reads it, and it is not the JSON field "schema" (macro_page
// writes "macro_page" there, macro_rates writes no such field).
type Meta struct {
	Name   string
	Schema string
}

// Builder builds a page view model; the contract of BuildMacroRatesFunc: generatedAt is given, the builder reads no clock.
type Builder func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Page is a builder with its Meta.
type Page struct {
	Meta  Meta
	Build Builder
}
```

Adapters (implementation, not interface): `macro_rates.Page()` returns `Meta{Name: "macro_rates", Schema: "page.macro_rates.v1"}`
with `Build` the package `Build`; `macro_page.NewPage(c Config)` returns `Meta{Name: "macro_page", Schema: "page.macro_page.v1"}`
with a `Build` that calls `macro_page.Build` with a copy of `c` (the `Indicators` slice copied at construction).

Zone: no pages code names a zone. `macro_page.Config.Zone` is given by the caller, as today. The tests use `time.UTC`. The `ot`
follow-up passes the zone of the profile (or, if the profile has none, the owner's answer to the open question), never a
literal in a pages package. Names are not constrained by a pattern here; the `ot` follow-up checks them when it builds its list.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the diff,
with a named test where it is about behaviour.

### pages-page-values (interface)

The `Page`, `Meta` and `Builder` names and the adapters of both builders.
  Goal: `internal/pages/api.go` declares `Meta`, `Builder` and `Page`, and `macro_rates.Page` and `macro_page.NewPage` return `pages.Page` values.
  Files: internal/pages/api.go, internal/pages/api_test.go, internal/pages/macro_rates/page.go, internal/pages/macro_rates/page_test.go, internal/pages/macro_page/page.go, internal/pages/macro_page/page_test.go, docs/contracts/pages.md
  Design: docs/design/pages-registry.md#interface
  Depends on: none (merged work only).
  Acceptance:
  1. `internal/pages/api.go` declares `Meta` (fields `Name` and `Schema`, both `string`), `Builder` (the signature of the Interface part) and `Page` (fields `Meta Meta` and `Build Builder`), and keeps `BuildMacroRatesFunc` and `NewHandlerFunc` with their present signatures; the doc comment of `Meta` says `Schema` is informational; the file holds declarations only.
  2. `internal/pages/api_test.go` has `TestPagesPageContract` (package `pages_test`, `t.Parallel()`): a `Page` built from a fake `Builder` returns the fake's bytes through `Build`, and a `[]pages.Page` is looked up by `Meta.Name`.
  3. `internal/pages/macro_rates/page.go` declares `func Page() pages.Page` returning `Meta{Name: "macro_rates", Schema: "page.macro_rates.v1"}` with `Build` set to the package `Build`; `page_test.go` has `TestPage`, which checks the `Meta` and that the bytes of `Page().Build` equal the bytes of `Build` for the points that `TestBuildDemo` uses.
  4. `internal/pages/macro_page/page.go` declares `func NewPage(c Config) pages.Page` returning `Meta{Name: "macro_page", Schema: "page.macro_page.v1"}`; `page_test.go` has `TestNewPage` (the `Meta`; the bytes equal `Build` called with the same `Config`) and `TestNewPageCopiesConfig` (changing the `Indicators` slice of the argument after `NewPage` leaves the output unchanged). Both use `Zone: time.UTC`, one indicator with `SeriesID` `DGS10`, `Name` `ten-year` and `SourceText` `FRED`, a fixed `generatedAt`, and call `t.Parallel()`; they read no clock, sleep never and touch no network.
  5. The three new Go files import, among `internal/...` packages, only `internal/pages`, `internal/contracts` and `internal/store` (grep of the import blocks); none imports `pgx`, a store implementation package or a sibling builder package; no file under `internal/pages` contains a zone name such as `America/` or `Asia/` (grep).
  6. `docs/contracts/pages.md` names the three declarations, the informational `Schema`, the two constructors and the zone rule, adds a Version history line, and lists under Contract tests the test names of items 2 to 4.
  7. No file other than the seven in Files is changed; no new package is made, so `arch.toml` and `.go-arch-lint.yml` are unchanged.
  8. The result matches `docs/design/pages-registry.md#interface`: the names, fields, signatures and `Meta` values above.
  Checked by CI: `go-arch-lint`, `golangci-lint`, `go test ./internal/pages/...`, `go run ./scripts/arch --check`.

## Left out

- The `ot` entry (`--name` on `ot page` and `ot serve`, an unknown name that lists the known names and fails before a store is opened or a port is bound): a `cmd/ot` file other than `wire.go` may not import implementation packages, and `wire.go` is held by issue 75. After issue 75 merges, an `ot` issue builds the page list in `wire.go` (one `deps` field holding a `[]pages.Page` built from `macro_rates.Page()` and `macro_page.NewPage(cfg)`), passes the profile's zone and fixed run id, scope and indicator values, adds `--name` (default `macro_rates`), and removes `deps.buildMacroRates` and, in a later interface issue, `BuildMacroRatesFunc`. Importing `internal/pages/macro_page` from `ot` needs one more name in the implementation list of `docs/design/modules.md` and the regenerated arch files (the marker words `these eleven` of `scripts/arch` are a count to rename then); that edit belongs to the follow-up design, not here. Tests that build the app (`testApp` in `cmd/ot/main_test.go`) must set the page list, or `ot page` and `ot serve` dereference an empty one; the follow-up lists every such test.
- A `Registry` interface and `NewRegistryFunc` (rejected, revision 2): two implementations and one caller; a slice and a lookup loop is enough. A registry is added when pages grow past a handful or a second caller exists.
- A global registry filled by `init()`: hidden state shared by tests.
- A `Builder` with a config or `any` parameter: each page closes over its own configuration in its adapter; pages that need request parameters (`indicator_detail`) come with their own design.
- A default `BuildID` made from `generatedAt`: it would change on every served request; `BuildID` stays the caller's.
- The owner's zone (an open question): the follow-up reads it from the profile or the owner decides; this design fixes none.
- `detail_href` routes, the other page builders, a units check of stored series: unchanged from `docs/design/pages-builders.md`.

## Adversarial

Source: `pages-registry.adversarial.md`, kept on the dispatcher's machine.

1. Gap against the goal: adopted. The literals of the second page (zone, ids, indicator) leave this design with the `ot` entry; the adapter test names its `Name` and `SourceText`; `Meta.Schema` is stated informational and reconciled with the JSON `schema` value.
2. Biggest unnamed risk (implementation imports outside `wire.go`): adopted. The `ot` entry moves to `## Left out` and waits for issue 75; no `cmd/ot` file is touched, so issue 75's grep is unaffected.
3. Simpler way: adopted. `Registry` and `NewRegistryFunc` are dropped, a slice and a lookup serve; the interface issue now spans `api.go`, two adapters and the contract document, with no new package.
4. Lock-in: adopted in part. Three names are frozen instead of five; the name pattern and the default `BuildID` are dropped; a `Builder` without parameters stays, with parametrised pages left to their own design.
5. Claim likely false: adopted. `.go-arch-lint.yml` leaves the Files line (no new package, no generated output); the `modules.md` and arch edits are reverted; the stale-count facts were checked and hold.
6. Likeliest slip: adopted in part. The nil page list in tests that build the app is named in `## Left out` for the follow-up; the New-York offset test and the `New` shadowing disappear with the registry and the zone literal.
