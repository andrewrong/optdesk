# Store master: the instrument tables, the resolver and the reader

Author: the optdesk dispatcher (draft)
Owner approved: (pending)
Date: 2026-11-07
Source: owner request optdesk-485 (option collection), first of three designs (`docs/design/store-master.md`, `docs/design/broker.md`,
`docs/design/option-jobs.md`); the jobs design is attached after this design's interface entry and the broker design's interface entry have landed.
Read on main at 0c3d9e6: `AGENTS.md`, `docs/design/modules.md`, `store-week2.md`, `internal/store/api.go`, `internal/store/setup/setup.go`,
`internal/store/snapshot/**`, `migrations/`, `scripts/arch/main.go`. Open pull requests read: 185 (option page design: files `arch.toml`,
`docs/design/modules.md`, `docs/design/option-page.md`, no migration), 184 and 182 (proto and transport).

Goal: every instrument gets a stable `instrument_id` from the master tables of the owner's schema v3 (2026-11-05), so that the rows of
`obs.option_eod` and `obs.quote_snapshot` point at something, and a page can turn an id back into a name and identifiers. This design
adds migration 0008, a store resolver ("the instrument of an identifier, or create it") and a reader. Nothing calls it yet.

Facts the design rests on:

- `instrument_id` and `underlying_id` are plain bigint columns with no master table behind them; the foreign keys were left out on purpose
  (`docs/design/store-week2.md:355`; `migrations/0006_option_eod.sql`, `migrations/0007_quote_snapshot.sql`). Both observation tables hold no live row
  (`store-week2.md:353-354`). `migrations/` ends at `0007_quote_snapshot.sql`; PR 185 claims no migration number, so 0008 is free.
- Roles are made by `Setup` (`internal/store/setup/setup.go:35-36` names two) and their lines are named in `internal/store/api.go:98-115`; a migration grants
  to a role that exists (`migrations/0006_option_eod.sql:92`). Open functions are declared in `api.go:191-207`.
- A sub-package of store opens pgx through its own copy of the seam: `snapshot/db.go` (`openDB`, `pgDB.InTx`, `Select`) is private to its package, and
  `store-week2.md:364` accepts a second copy rather than one more shared package.
- `scripts/arch/main.go:4` says `arch.toml` depends on the table of `modules.md` only; the directory components of a new package reach `.go-arch-lint.yml`
  alone (`deriveTree`, `main.go:231`), and ci-go runs go-arch-lint, so a new package directory needs its entry there.

## Modules

No change to `docs/design/modules.md` or `arch.toml`: `internal/store/**`, `migrations/**` and `docs/contracts/store.md` are store (`modules.md:67`);
`.go-arch-lint.yml` is repo (`modules.md:77`). The new package is `internal/store/instrument`, a sub-package of store that imports the root `store`, `contracts`, pgx and the standard library.

| issue | module |
|---|---|
| store-master-interface | store and repo (licensed, below) |
| store-master | store |

Licence: the interface entry lies in two modules because the stub package `internal/store/instrument/doc.go` must be listed in `.go-arch-lint.yml`
(a repo file) at the moment the directory gets its first Go file; the precedent is the store-week2 option_eod entry (`store-week2.md:225-230`). The entry after it lies in store only.

## Interface

Owner: store, `internal/store/api.go` (declarations only). Added next to the other stores (`api.go:53-79`):

| name | declaration |
|---|---|
| `store.Identifier` | `struct{ Scheme, Value string }`; schemes are `ibkr_conid`, `osi`, `ticker_exchange`, `jpx_option_code`, `isin` |
| `store.InstrumentSpec` | `struct{ Type, Name, Currency, MIC, CalendarID, Status, PriceIncrement, LotSize string }`; `PriceIncrement` and `LotSize` are decimal text; `Type` is `stock`, `index` or `option` |
| `store.Instrument` | `struct{ ID int64; Spec InstrumentSpec; Identifiers []Identifier }`; the identifiers that are valid now |
| `store.InstrumentResolver` | `interface{ ResolveInstrument(ctx context.Context, key Identifier, spec InstrumentSpec, extra []Identifier) (int64, error) }`: the id of the instrument that `key` names now, created with `spec` and the identifiers `key` and `extra` when none exists; the same key always gives the same id, under concurrent calls too |
| `store.InstrumentReader` | `interface{ Instruments(ctx context.Context, ids []int64) ([]Instrument, error) }`, ordered by id; an unknown id is absent from the answer |
| `store.InstrumentStore` | the resolver, the reader and `Close() error` |
| `store.OpenInstrumentFunc` | `func(ctx context.Context, dsn string) (InstrumentStore, error)`, implemented by `instrument.Open` |
| `store.DSNEnvMaster` | the line name `OT_PG_DSN_MASTER` (role `ot_w_master`, the writer role the owner's schema names) |

Tables (migration 0008). The two blocks are the owner's schema v3 (2026-11-05), section 3.1, copied; nothing else of that text is used:

```sql
master.instrument(instrument_id bigint generated always as identity PRIMARY KEY, instrument_type text NOT NULL,
  issuer_id bigint, name text NOT NULL, currency char(3) NOT NULL, mic text NOT NULL, calendar_id text NOT NULL,
  status text NOT NULL, price_increment numeric NOT NULL, lot_size numeric NOT NULL, recorded_at timestamptz NOT NULL)
master.instrument_identifier(instrument_id, scheme text, value text, valid_from timestamptz, valid_to timestamptz,
  PRIMARY KEY (scheme, value, valid_from))            -- ibkr_conid | osi | ticker_exchange | jpx_option_code | isin
```

Written out in the migration, `instrument_id` of the identifier table is `bigint NOT NULL REFERENCES master.instrument`, `valid_from` is `NOT NULL`, and `scheme` has a CHECK
over the five values. Three foreign keys are added: `obs.option_eod.instrument_id`, `obs.option_eod.underlying_id` and `obs.quote_snapshot.instrument_id` to `master.instrument(instrument_id)`.
The owner's schema allows a hypertable to point at a plain table ("a hypertable may point at a plain table, not the reverse", section 2.2, `instrument_id` row). The risk this design names: both tables have compression
enabled (`migrations/0006_option_eod.sql:70-72`, the same for 0007), and adding a constraint to a hypertable with compression enabled is the one step I could not verify offline. Both tables are empty, so
the fallback inside the migration is allowed: switch compression off, add the keys, switch it on with the same settings; the `pg` test of acceptance 7 decides which is needed.

Resolver rules (owner: store, `instrument.resolve`): the lookup is the identifier row with `valid_to IS NULL` for `(scheme, value)`; a miss inserts the instrument (`recorded_at` is the database's `now()`, `status` as the spec gives it,
default `active`) and the identifiers `key` and `extra` with `valid_from` = `now()`, in one transaction that first takes `pg_advisory_xact_lock(hashtext(scheme || ':' || value))`, so two callers cannot create two
instruments for one key (the primary key alone would allow it, because `valid_from` is part of it). The resolver never closes an identifier (`valid_to` stays NULL); a renamed contract is a follow-up. An identifier of `extra` that
already names another instrument is a `Permanent` error and nothing is written. An unknown scheme, an empty value, a `Type` outside the three, a `Currency` that is not three upper-case letters or a decimal that does not parse is a `Permanent` error before any statement.

## Issues

Items under `Checked by CI` are decided by the workflows of the head commit; every other item is an offline grep or a named test, run without network, service or database. Every test uses
`t.Parallel()` and `t.TempDir()`; none sleeps; a test that needs PostgreSQL carries the build tag `pg` and runs only in GitHub CI.

### store-master-interface (interface)

The declarations and the stub package
  Goal: `internal/store/api.go` declares the types of `## Interface` and `internal/store/instrument` exists as a stub that the lint knows.
  Files: internal/store/api.go, internal/store/api_test.go, internal/store/instrument/doc.go
  Design: docs/design/store-master.md#interface
  Cross-module: store and repo, licensed by this design (see `## Modules`).
  Acceptance:
  1. `git diff internal/store/api.go` shows additions only and they are exactly the types, `OpenInstrumentFunc` and `DSNEnvMaster` of `## Interface`; `grep -cE '^func ' internal/store/api.go` prints 0.
  2. `api_test.go` tests the contract from both sides: a fake `InstrumentStore` that returns one id for one key and an `Instrument` list ordered by id, and a caller that resolves the same key twice and reads the ids back.
  3. `internal/store/instrument/doc.go` is a package comment and the package clause (`wc -l` is at most 3).
  4. `go run ./scripts/arch --check` prints `arch: ok`; `.go-arch-lint.yml` lists the component `store/instrument`; `git diff -- go.mod go.sum` is empty.
  5. Every test added calls `t.Parallel()` and none sleeps.
  Checked by CI: ci-go (build, vet, test, golangci-lint, go-arch-lint), `checks` (arch); the design verdict on the pull request (an interface change).
  Pull request body: `Contracts: unchanged`.
  Depends on: none (merged work only). Order: before `store-master`.
  Size: S

### store-master (store)

Migration 0008, the role, the resolver and the reader
  Goal: `instrument.Open` returns a `store.InstrumentStore` over the tables of `## Interface`, the role `ot_w_master` exists, and the three observation columns point at `master.instrument`.
  Files: migrations/0008_master_instrument.sql, internal/store/instrument/open.go, internal/store/instrument/db.go, internal/store/instrument/resolve.go, internal/store/instrument/read.go, internal/store/instrument/resolve_test.go, internal/store/instrument/read_test.go, internal/store/instrument/instrument_pg_test.go, internal/store/setup/setup.go, internal/store/setup/setup_test.go, internal/store/migrate/migrate_test.go, docs/contracts/store.md
  Design: docs/design/store-master.md#interface
  Acceptance:
  1. `open.go` holds `var _ store.OpenInstrumentFunc = Open`; `go list -deps ./internal/store/instrument | grep optdesk/internal` lists only `store` and `contracts`; `db.go` is its own copy of the seam (no import of `snapshot`); only `internal/store` packages import pgx (depguard, CI).
  2. `0008_master_instrument.sql` has an Up and a Down part, creates the two tables with the owner's columns, the key, the NOT NULL and the CHECK on `scheme` of `## Interface`, adds the three foreign keys, grants `ot_w_master` `SELECT` and `INSERT` on both tables with `USAGE` on schema `master`, grants `SELECT` on both to `ot_ro_pages` and `ot_runner`, and nothing else; the Down part drops the keys before the tables; `migrate_test.go` (no database) lists the file after 0007 and checks both parts.
  3. `setup.go` creates the role `ot_w_master`; the test checks its name against `store.DSNEnvMaster` as `setup_test.go:129-136` does for the other roles.
  4. `TestResolveCreates` and `TestResolveSame` (a fake `database` seam, as the `snapshot` tests do): an unknown key inserts the instrument and the identifiers `key` and `extra` in one transaction and returns the id; the same key later returns the same id and inserts nothing.
  5. `TestResolveLock`: the transaction's first statement is `pg_advisory_xact_lock` over the key, and the lookup follows it (the test checks the statement sequence on the seam); `TestResolveExtraTaken`: an `extra` identifier that names another instrument gives `Permanent` and no insert.
  6. `TestResolveChecks` (table): the invalid inputs of `## Interface` are `Permanent` and nothing reaches the seam; `TestInstruments`: ids in, instruments with their valid identifiers out ordered by id, an unknown id absent, `PriceIncrement` and `LotSize` round-trip as the decimal text given.
  7. `instrument_pg_test.go` carries the build tag `pg` (GitHub CI only): migration 0008 applies to a database that holds 0001 to 0007 (the compressed-hypertable step of `## Interface` included); an `obs.option_eod` insert with an unknown `instrument_id` or `underlying_id`, and an `obs.quote_snapshot` insert with an unknown `instrument_id`, fail; two concurrent resolves of one key give one id; the role `ot_w_master` cannot update or delete.
  8. `docs/contracts/store.md` lists the tables, the resolver and the reader; each new file is under 600 lines; `git diff -- go.mod go.sum` is empty.
  Checked by CI: ci-go (go test, golangci-lint, go-arch-lint component `store/instrument`); the `pg` job (GitHub CI only).
  Depends on: store-master-interface merged (merged work only).
  Size: L

Width: one entry after the interface. Splitting the resolver from the reader would put two entries in one package (`open.go` and `db.go` are shared), and splitting the migration from the code would leave the `pg` test of either without the other; one entry is the honest width.

## Left out

1. `master.option_terms` (strike, expiry, right, multiplier of an option, `terms_version_id`) and the owner's other master tables: not built. An option's terms are kept as an identifier of scheme `osi` (the option symbol the jobs build) and in the name, which a page can read; the column `terms_version_id` of the observation tables stays NULL. `issuer_id` stays NULL.
2. Closing an identifier (`valid_to`), a renamed or adjusted contract, and any merge of two instruments: a follow-up. The resolver only creates.
3. Which values the jobs give for `mic`, `calendar_id`, `price_increment` and `lot_size` (the broker proto carries none of the first three): the jobs design takes them from its manifests. The values of `Type` (`stock`, `index`, `option`) are this design's choice; the owner's text does not list them.
4. The capture ledger and the foreign key of `capture_id` to `ops.capture_run` (`store-week2.md:351-354`).
5. Wiring: `OpenInstrumentFunc`, the line `OT_PG_DSN_MASTER` and the role on the owner's database are used by `ot-collect` (the follow-up that wires the three designs into `ot` and the runner) and by `ot setup pg` and `ot migrate`, which are the owner's; no entry runs them. Scheduling, live runs and any database of the owner: the owner's.
6. Textual risk: `.go-arch-lint.yml` is edited by the interface entries of all three designs, and PR 185 edits `arch.toml` and `modules.md`; the entries are attached one after another so each rebases onto the last.
7. What the option page design needs from this one: `store.InstrumentReader.Instruments` turns the ids of `OptionEodReader` and `QuoteSnapshotReader` rows (`internal/store/api.go:36-51`) into a name and identifiers; `ot_ro_pages` reads the tables directly (migration grants above).
8. Owner-only and second-opinion cases: none of the paths is owner-only; the new role and the migration take effect only when the owner runs `ot setup pg` and `ot migrate`. No network call and no new dependency.

## Adversarial

1. Gap against the goal: adopted as stated; the tables alone write no row, `ot-collect`, `runner-window-per-dataset` and the pyside serving are named in Left out here and in the other two designs.
2. Unnamed risk (runner shape, lease, capture id, cache): not this design's; answered in `docs/design/option-jobs.md` and `docs/design/broker.md`.
3. Simpler way (conId as key, one table, no resolver): not adopted, the advisor settled that ids come from the owner's master tables; the cost is kept small by two tables, one resolver and no `option_terms` (Left out 1).
4. Lock-in (master shape, FKs on a compressed hypertable): adopted in part; the shape is the owner's text copied, the FK step is named as a risk with a `pg` test and a fallback inside the migration (`## Interface`, acceptance 7); the identity-then-insert race is closed by an advisory lock.
5. False claims: adopted; the arch statement now says `arch.toml` comes from the table and `.go-arch-lint.yml` from the tree, and the stub package is listed in the interface entry.
6. Slip causes (arch files, wide interface entry, go.mod conflicts, PR 185 clash): adopted; the interface entry is two modules, it carries the one lint line, this design touches no `go.mod`, 0008 is free in PR 185, and the shared lint file is a stated textual risk (Left out 6).

Full answer: scratchpad adversarial-broker-jobs.md and adversarial-option-collection.md of the dispatcher window (2026-11-07)
