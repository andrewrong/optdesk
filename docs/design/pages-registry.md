# Pages: the registry, and `ot page` and `ot serve` over it

Author: advisor of optdesk (first draft)
Source: `## Left out` of `docs/design/pages-builders.md` (the `Page`, `Meta` and `Builder` names, "the first design that
adds a registry freezes them"; the adapter and the wiring into `ot page`). Read on main at c63cbb4: `internal/pages/api.go`
holds `BuildMacroRatesFunc` and `NewHandlerFunc`; `internal/pages/macro_rates.Build(ctx, r, day, generatedAt)` and
`internal/pages/macro_page.Build(ctx, r, c, day, generatedAt)` (with `macro_page.Config` and `macro_page.Indicator`) are
the two builders; `cmd/ot/page.go` and `cmd/ot/serve.go` call `a.deps.buildMacroRates` (a field of `deps` in
`cmd/ot/wire.go`, 11 fields, checked by `TestNewDepsFilled`); `cmd/ot/main.go` holds `app` and `run`.

Goal of the design: `ot page --name N` and `ot serve --name N` build any page that is registered, today `macro_rates`
(the default, so the current commands behave as before) and `macro_page`. A page is a value (`Page`) with a name and a
builder; a registry holds the values; each builder package gives one constructor that makes its builder a value.

Open issue 75 edits `cmd/ot/wire.go`, `cmd/ot/migrate.go` and `cmd/ot/calendar.go`. The `ot` entry here edits none of
them and does not depend on that issue (see the entry, and `## Left out` for the field this leaves behind).

## Modules

Changed: the row `ot`, its list of implementation packages in `docs/design/modules.md` gains
`internal/pages/macro_page` and `internal/pages/registry` (the only way `cmd/ot` may import them; sub-package components
may not import each other, so no existing package can reach them for `ot`). This pull request makes that edit and
carries the regenerated `arch.toml` and `.go-arch-lint.yml` of `scripts/arch` (the `ot` may-import list and the two new
components). The sentence of the list keeps the words `these eleven`, which `scripts/arch` parses as its marker, and
adds the two packages after them (see `## Left out`). No other row changes. The depends cell of `pages` stays
`contracts store`; the registry imports the root `pages` only.

| issue | module |
|---|---|
| pages-registry (interface) | pages |
| ot-pages-registry | ot |

No entry edits `go.mod`, `go.sum`, `contracts/`, `web/` or `docs/design/modules.md`. No new direct dependency.

## Interface

Owner: `pages`, file `internal/pages/api.go`. Five declarations are added; the two existing ones are unchanged.

```go
// Meta names a page: Name is what the commands take (--name), Schema the contract it fills (page.<name>.v1).
type Meta struct {
	Name   string
	Schema string
}

// Builder builds a page view model; the same contract as BuildMacroRatesFunc: generatedAt is given,
// the builder reads no clock.
type Builder func(ctx context.Context, r store.SeriesReader, day contracts.Day, generatedAt time.Time) ([]byte, error)

// Page is a builder with its Meta.
type Page struct {
	Meta  Meta
	Build Builder
}

// Registry holds pages by name. List is sorted by Name.
type Registry interface {
	Get(name string) (Page, bool)
	List() []Meta
}

// NewRegistryFunc makes a registry (internal/pages/registry.New). It fails on an empty or malformed Name,
// an empty Schema, a nil Build, or a repeated Name.
type NewRegistryFunc func(pages ...Page) (Registry, error)
```

Rules the contract tests hold:

- `Name` matches `^[a-z][a-z0-9_]{0,63}$`; `Schema` is non-empty; `Build` is non-nil; names are unique.
- `Get` of a name that is not registered returns the zero `Page` and false; `List` returns a fresh slice each call.
- The registry copies the pages it is given: a later change of the argument slice does not change the registry.
- The error of `NewRegistryFunc` names the offending page name and no other text of the input.

The implementations (not interface): `registry.New` in `internal/pages/registry`; `macro_rates.Page()` returns
`Meta{Name: "macro_rates", Schema: "page.macro_rates.v1"}` with `Build` the existing `Build`; `macro_page.NewPage(c Config)`
returns `Meta{Name: "macro_page", Schema: "page.macro_page.v1"}` with a `Build` that calls `macro_page.Build` with a copy
of `c` (the `Indicators` slice copied) and, when `c.BuildID` is empty, `BuildID` set to `build-` followed by
`generatedAt` in UTC as `20060102T150405Z`, which matches the schema pattern.

In `cmd/ot` (not interface): `newPageSet()` in a new file builds the registry of both pages; `app` gains one field
holding it. `ot page` and `ot serve` gain `--name` (default `macro_rates`); an unknown name is an error that lists the
registered names, raised before a store is opened or a port is bound.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is a fact read from the
diff, with a named test where it is about behaviour.

### pages-registry (interface)

The `Page`, `Meta` and `Builder` names, the registry, and the adapters that make both builders registry values.
  Goal: `internal/pages/api.go` declares the registry types, `internal/pages/registry` implements them, and `macro_rates.Page` and `macro_page.NewPage` return registry values.
  Files: internal/pages/api.go, internal/pages/api_test.go, internal/pages/registry/registry.go, internal/pages/registry/registry_test.go, internal/pages/macro_rates/page.go, internal/pages/macro_rates/page_test.go, internal/pages/macro_page/page.go, internal/pages/macro_page/page_test.go, docs/contracts/pages.md, .go-arch-lint.yml.
  Design: docs/design/pages-registry.md#interface
  Depends on: none (merged work only).
  Acceptance:
  1. `internal/pages/api.go` declares `Meta` (fields `Name`, `Schema`, both `string`), `Builder` (the signature of the Interface part), `Page` (fields `Meta Meta` and `Build Builder`), `Registry` (methods `Get(name string) (Page, bool)` and `List() []Meta`) and `NewRegistryFunc` (`func(pages ...Page) (Registry, error)`), and keeps `BuildMacroRatesFunc` and `NewHandlerFunc` with their present signatures; the file holds declarations only and imports no sub-package of `pages`.
  2. `internal/pages/api_test.go` has `TestPagesRegistryContract` (package `pages_test`, `t.Parallel()`): a fake `Registry` and a fake `NewRegistryFunc` are used through the interface, `Get` of a missing name gives the zero `Page` and false, and a `Page` built from a fake `Builder` returns the fake's bytes.
  3. `internal/pages/registry/registry.go` declares `func New(pages ...pages.Page) (pages.Registry, error)`; `registry_test.go` assigns it with `var _ pages.NewRegistryFunc = New`, and has `TestNewAndGet`, `TestListSorted` (names given out of order come back sorted, a second `List` call returns a slice that is not the first), `TestNewRejects` (table: empty name, uppercase name, name over 64 characters, empty schema, nil `Build`, repeated name; each returns an error that contains the page name and no `Schema` text) and `TestNewCopies` (changing the argument slice after `New` leaves `Get` and `List` unchanged); every test calls `t.Parallel()`.
  4. `internal/pages/macro_rates/page.go` declares `func Page() pages.Page` returning `Meta{Name: "macro_rates", Schema: "page.macro_rates.v1"}` and `Build` set to the package `Build`; `page_test.go` has `TestPage`, which checks the `Meta` and that the bytes of `Page().Build` equal the bytes of `Build` for the demo points already used by `TestBuildDemo`.
  5. `internal/pages/macro_page/page.go` declares `func NewPage(c Config) pages.Page` returning `Meta{Name: "macro_page", Schema: "page.macro_page.v1"}`; `page_test.go` has `TestNewPage` (the `Meta`; the bytes equal `Build` called with the same `Config` and a non-empty `BuildID`), `TestNewPageBuildID` (an empty `BuildID` gives an id of the form `build-YYYYMMDDTHHMMSSZ` from `generatedAt` in UTC and the output carries it as `build_id`; the id matches `^[A-Za-z0-9_-]{1,64}$`) and `TestNewPageCopiesConfig` (changing the `Indicators` slice of the argument after `NewPage` leaves the output unchanged); every test calls `t.Parallel()`, reads no clock and touches no network.
  6. The three new Go files import, among `internal/...` packages, only `internal/pages`, `internal/contracts` and `internal/store` (grep of the import blocks); none imports `pgx`, a store implementation package or a sibling builder package.
  7. `docs/contracts/pages.md` names the five declarations, the rules of the Interface part (name pattern, copy, sorted `List`), the two constructors, a Version history line for the registry, and under Contract tests the test names of items 2 to 5.
  8. Generated output: the new package `internal/pages/registry` adds a component to `.go-arch-lint.yml` (`arch.toml` and `modules.md` already carry it from this design); the entry carries the regenerated file and no hand edit. No file other than the ten in Files is changed.
  9. The result matches `docs/design/pages-registry.md#interface`: the names, fields, signatures, the `Meta` values and the `build-` id form above.
  Checked by CI: `go run ./scripts/arch --check`, `go-arch-lint`, `golangci-lint`, `go test ./internal/pages/...`.

### ot-pages-registry

`ot page` and `ot serve` select a page of the registry by name.
  Goal: `ot page --name macro_page` and `ot serve --name macro_page` build the `macro_page` view model; with no `--name` both behave as before.
  Files: cmd/ot/pageset.go, cmd/ot/pageset_test.go, cmd/ot/main.go, cmd/ot/page.go, cmd/ot/serve.go, cmd/ot/page_test.go, cmd/ot/serve_test.go
  Design: docs/design/pages-registry.md#issues
  Depends on: pages-registry (the entry marked (interface); its adapters and registry must be merged first).
  Acceptance:
  1. `cmd/ot/pageset.go` is a new file that declares `func newPageSet() (pages.Registry, error)`; it calls `registry.New` with `macro_rates.Page()` and `macro_page.NewPage(cfg)`, where `cfg` has `RunID` `ot`, `Scope` `macro`, `Zone` loaded from the constant `marketZone` of `cmd/ot/day.go` (no zone name is written in `pageset.go`), and one indicator whose `SeriesID` is `DGS10`; it imports no other implementation package than `internal/pages/registry`, `internal/pages/macro_rates` and `internal/pages/macro_page`.
  2. `cmd/ot/main.go` adds one field of type `pages.Registry` to `app` and fills it in `run` from `newPageSet()` (an error ends the start with exit code 2, like a bad profile); `newApp` is unchanged, so `testApp` still builds an app without it.
  3. `cmd/ot/page.go` reads a `--name` flag (default `macro_rates`), gets the page from the `app` field, and calls its `Build`; the text `buildMacroRates` no longer appears in `cmd/ot/page.go` or `cmd/ot/serve.go` (grep count 0 in each).
  4. `cmd/ot/serve.go` reads the same flag with the same default and builds through the same page; an unknown name in either command returns an error that contains the name and the registered names in sorted order, and `TestPageUnknownName` and `TestServeUnknownName` show that `openSeries` is not called and, for serve, no port is bound.
  5. `cmd/ot/pageset_test.go` has `TestPageSetBuildsBoth`: with the real `newPageSet()` and `demoReader`, both pages build, the JSON of `macro_rates` has a `series` array and no `schema` field, that of `macro_page` has the `schema` field `macro_page`, `generated_at` of `macro_page` is in the zone of `marketZone`, and the test uses a fixed `generatedAt`, no clock, no sleep and no network; and `TestPageSetNames` (the names are `macro_page` and `macro_rates`).
  6. `cmd/ot/page_test.go` and `cmd/ot/serve_test.go` replace the use of `a.deps.buildMacroRates` by a registry of one fake page set on the `app` field; `TestPageBytesAreBuilderBytes` and `TestPageDemoOpensNoStore` keep their assertions, and a new `TestPageNameSelects` shows that `--name macro_page` calls the `macro_page` fake and no flag calls `macro_rates`. All tests call `t.Parallel()` and write files only under `t.TempDir()`.
  7. `git diff --name-only` lists none of `cmd/ot/wire.go`, `cmd/ot/wire_test.go`, `cmd/ot/migrate.go`, `cmd/ot/calendar.go` and `cmd/ot/testdata`, and the `deps` struct is unchanged (the field `buildMacroRates` stays, unused by the commands; see `## Left out` of the design).
  8. No file other than the seven in Files is changed. `cmd/ot` imports `internal/pages/registry` and `internal/pages/macro_page` only from `pageset.go`.
  9. The result matches `docs/design/pages-registry.md#issues` and the last paragraph of its Interface part: `--name`, the default, the unknown-name error before any store or port, and `newPageSet`.
  Checked by CI: `go-arch-lint` (the `ot` component may import the two packages from the edit of `docs/design/modules.md` in this design), `go test ./cmd/...`, `golangci-lint`.

## Left out

- A registry as global state filled by `init()` (as `register` of `cmd/ot` does for commands): hidden state shared by tests, which AGENTS.md forbids for tests, and an import for effect. The registry is a value built by `newPageSet`.
- The registry as a map inside `cmd/ot`: no contract test from both sides, and logic in the composition root.
- A `Builder` that takes a config or `any`: each page has its own configuration (`macro_page.Config`); the adapter closes over it, so the shared signature stays that of `BuildMacroRatesFunc`.
- Removing `deps.buildMacroRates` and the type `BuildMacroRatesFunc`: the field lives in `cmd/ot/wire.go`, which issue 75 edits (it also changes the field count checked by `TestNewDepsFilled`). The field stays unused for now; a later `ot` issue removes it, and the then unused `macro_rates` import of `wire.go`, after issue 75 merges. `BuildMacroRatesFunc` is removed from `api.go` by an interface issue after that.
- Page configuration from a file or from `.env` (run id, scope, indicators, zone): `newPageSet` fixes one indicator (`DGS10`, the only stored series) and uses `marketZone`; the owner's zone is a question for the owner (open question). More indicators wait for more stored series.
- `ot page --list` and one `ot serve` process routing several pages by URL path: `--name` selects one page per process; routing needs `detail_href` routes and a front-end decision.
- `detail_href` routes, the other page builders (`indicator_detail`, `settings_page`, and so on): as in `docs/design/pages-builders.md`.
- Renaming the marker `these eleven` that `scripts/arch` parses in `docs/design/modules.md`: the words stay and the list after them gains two packages, because changing the marker is a `scripts/arch` change (repo module). A repo issue should make the parser take the list without a count.
- A units check of stored series: unchanged from `docs/design/pages-builders.md`.

## Adversarial

pending
